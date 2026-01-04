# Interprocess Communication

Database polling for communication between gatherers and deduplicator.

---

## Overview

The Kalshi Data Platform uses **database polling** instead of message queues for interprocess communication:

```mermaid
flowchart LR
    subgraph "Gatherer 1"
        G1[gatherer] --> TS1[(TimescaleDB)]
        G1 --> PG1[(PostgreSQL)]
    end

    subgraph "Gatherer 2"
        G2[gatherer] --> TS2[(TimescaleDB)]
        G2 --> PG2[(PostgreSQL)]
    end

    subgraph "Gatherer 3"
        G3[gatherer] --> TS3[(TimescaleDB)]
        G3 --> PG3[(PostgreSQL)]
    end

    subgraph "Deduplicator"
        D[deduplicator]
    end

    TS1 & TS2 & TS3 -->|SQL poll| D
    PG1 & PG2 & PG3 -->|SQL poll| D

    D -->|SQL write| PROD[(Production RDS)]
```

---

## Why Database Polling?

| Approach | Pros | Cons |
|----------|------|------|
| **Message Queue** (Kafka, SQS) | Real-time, push-based | Extra infra, operational complexity, cost |
| **Database Polling** (current) | Simple, no extra infra, data already in DB | Slight latency, polling overhead |

**Decision:** Database polling is simpler and sufficient for our use case:
- Data is already written to local databases
- Sub-second latency is acceptable
- No additional infrastructure to manage
- Built-in durability (database persistence)

---

## Polling Architecture

### Deduplicator Connections

```mermaid
flowchart TD
    DEDUP[Deduplicator]

    subgraph "Read Connections (12)"
        G1_TS[Gatherer 1 TimescaleDB<br/>2 conns]
        G1_PG[Gatherer 1 PostgreSQL<br/>2 conns]
        G2_TS[Gatherer 2 TimescaleDB<br/>2 conns]
        G2_PG[Gatherer 2 PostgreSQL<br/>2 conns]
        G3_TS[Gatherer 3 TimescaleDB<br/>2 conns]
        G3_PG[Gatherer 3 PostgreSQL<br/>2 conns]
    end

    subgraph "Write Connections (4-6)"
        PROD[Production RDS<br/>4-6 conns]
    end

    DEDUP --> G1_TS & G1_PG
    DEDUP --> G2_TS & G2_PG
    DEDUP --> G3_TS & G3_PG
    DEDUP --> PROD
```

| Connection Type | Per Gatherer | Total |
|-----------------|--------------|-------|
| TimescaleDB (read) | 2 | 6 |
| PostgreSQL (read) | 2 | 6 |
| Production RDS (write) | - | 4-6 |
| **Total** | | **16-18** |

---

## Cursor-Based Sync

### How It Works

Each table is synced independently using a cursor (timestamp of last synced record):

```mermaid
sequenceDiagram
    participant D as Deduplicator
    participant G as Gatherer DB
    participant P as Production RDS

    D->>P: Load cursor for trades (gatherer-1)
    P-->>D: cursor = 1705312800000000

    loop Every 100ms
        D->>G: SELECT * FROM trades<br/>WHERE received_at > cursor<br/>ORDER BY received_at<br/>LIMIT 5000
        G-->>D: [batch of records]
        D->>D: Deduplicate against other gatherers
        D->>P: INSERT INTO trades (ON CONFLICT DO NOTHING)
        D->>P: UPDATE cursor = max(received_at)
    end
```

### Cursor Storage

Cursors are stored in the production RDS:

```sql
CREATE TABLE sync_cursors (
    gatherer_id  VARCHAR(32) NOT NULL,
    table_name   VARCHAR(64) NOT NULL,
    last_synced  BIGINT NOT NULL,  -- received_at timestamp
    updated_at   TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (gatherer_id, table_name)
);
```

Example state:

| gatherer_id | table_name | last_synced |
|-------------|------------|-------------|
| gatherer-1 | trades | 1705312856123456 |
| gatherer-1 | orderbook_deltas | 1705312856234567 |

### Cursor Atomicity

**Critical:** Data insert and cursor update MUST be in the same database transaction:

```go
func (d *Deduplicator) syncBatch(ctx context.Context, table string, records []Record) error {
    tx, err := d.production.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // Insert records
    for _, r := range records {
        if _, err := tx.ExecContext(ctx, insertSQL, r.Fields()...); err != nil {
            return err  // Rollback on error
        }
    }

    // Update cursor (same transaction)
    maxTs := records[len(records)-1].ReceivedAt
    if _, err := tx.ExecContext(ctx,
        `UPDATE sync_cursors SET last_synced = $1, updated_at = NOW()
         WHERE gatherer_id = $2 AND table_name = $3`,
        maxTs, d.gathererID, table,
    ); err != nil {
        return err  // Rollback on error
    }

    return tx.Commit()  // Atomic: both succeed or both fail
}
```

**Failure scenarios:**

| Scenario | Insert | Cursor Update | Result |
|----------|--------|---------------|--------|
| Success | Committed | Committed | Normal |
| Crash before commit | Rolled back | Rolled back | Re-sync same batch |
| Insert fails | Rolled back | Not attempted | Re-sync same batch |
| Cursor update fails | Rolled back | Rolled back | Re-sync same batch |

**Guarantee:** No data loss, no cursor advancement without data. Duplicates handled by `ON CONFLICT DO NOTHING`.

---

## Polling Strategy

### Per-Table Polling

```go
type SyncConfig struct {
    Tables []TableConfig
}

type TableConfig struct {
    Name          string
    PollInterval  time.Duration  // How often to poll
    BatchSize     int            // Records per batch
    Parallel      bool           // Poll gatherers in parallel
}

var DefaultConfig = SyncConfig{
    Tables: []TableConfig{
        {Name: "trades", PollInterval: 100*time.Millisecond, BatchSize: 5000, Parallel: true},
        {Name: "orderbook_deltas", PollInterval: 100*time.Millisecond, BatchSize: 5000, Parallel: true},
        {Name: "orderbook_snapshots", PollInterval: 1*time.Second, BatchSize: 1000, Parallel: true},
        {Name: "tickers", PollInterval: 100*time.Millisecond, BatchSize: 5000, Parallel: true},
        {Name: "markets", PollInterval: 5*time.Second, BatchSize: 1000, Parallel: false},
        {Name: "events", PollInterval: 30*time.Second, BatchSize: 100, Parallel: false},
    },
}
```

### Polling Loop

```go
func (d *Deduplicator) syncTable(ctx context.Context, table TableConfig) {
    ticker := time.NewTicker(table.PollInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if table.Parallel {
                d.pollGatherersParallel(ctx, table)
            } else {
                d.pollGatherersSequential(ctx, table)
            }
        }
    }
}

func (d *Deduplicator) pollGatherersParallel(ctx context.Context, table TableConfig) {
    var wg sync.WaitGroup
    results := make(chan []Record, len(d.gatherers))

    for _, g := range d.gatherers {
        wg.Add(1)
        go func(gatherer *Gatherer) {
            defer wg.Done()
            records := d.pollGatherer(ctx, gatherer, table)
            results <- records
        }(g)
    }

    go func() {
        wg.Wait()
        close(results)
    }()

    // Collect and deduplicate
    var allRecords []Record
    for records := range results {
        allRecords = append(allRecords, records...)
    }

    deduplicated := d.deduplicate(allRecords)
    d.writeToProduction(ctx, table.Name, deduplicated)
}
```

---

## Deduplication

### By Unique Key

Each record type has a unique key from Kalshi:

| Table | Unique Key |
|-------|------------|
| trades | `trade_id` (UUID) |
| orderbook_deltas | `ticker` + `exchange_ts` + `price` + `side` |
| orderbook_snapshots | `ticker` + `snapshot_ts` + `source` |
| tickers | `ticker` + `exchange_ts` |
| markets | `ticker` |
| events | `event_ticker` |

### Deduplication Logic

```go
func (d *Deduplicator) deduplicate(records []Record) []Record {
    seen := make(map[string]struct{})
    result := make([]Record, 0, len(records))

    for _, r := range records {
        key := r.UniqueKey()
        if _, exists := seen[key]; !exists {
            seen[key] = struct{}{}
            result = append(result, r)
        }
    }

    return result
}
```

### Database-Level Deduplication

Production tables use `ON CONFLICT DO NOTHING`:

```sql
INSERT INTO trades (trade_id, ticker, exchange_ts, ...)
VALUES ($1, $2, $3, ...)
ON CONFLICT (trade_id) DO NOTHING;
```

This ensures duplicates from multiple gatherers don't cause errors.

---

## Network Flow

### Port Usage

| Source | Destination | Port | Protocol |
|--------|-------------|------|----------|
| Deduplicator | Gatherer TimescaleDB | 5432 | PostgreSQL |
| Deduplicator | Gatherer PostgreSQL | 5433 | PostgreSQL |
| Deduplicator | Production RDS | 5432 | PostgreSQL |

### Connection Strings

```yaml
# Deduplicator config
gatherers:
  - id: "gatherer-1"
    timescaledb: "postgres://dedup:xxx@10.0.1.10:5432/kalshi_ts?sslmode=require"
    postgresql: "postgres://dedup:xxx@10.0.1.10:5433/kalshi_meta?sslmode=require"
  - id: "gatherer-2"
    timescaledb: "postgres://dedup:xxx@10.0.2.10:5432/kalshi_ts?sslmode=require"
    postgresql: "postgres://dedup:xxx@10.0.2.10:5433/kalshi_meta?sslmode=require"
  - id: "gatherer-3"
    timescaledb: "postgres://dedup:xxx@10.0.3.10:5432/kalshi_ts?sslmode=require"
    postgresql: "postgres://dedup:xxx@10.0.3.10:5433/kalshi_meta?sslmode=require"

production:
  url: "postgres://dedup:xxx@kalshi-prod.xxx.rds.amazonaws.com:5432/kalshi_prod?sslmode=require"
```

---

## Latency

### End-to-End

```
Kalshi API → Gatherer → Local DB → Deduplicator Poll → Production RDS

                                   ↑
                           100ms poll interval
```

| Stage | Latency |
|-------|---------|
| Kalshi → Gatherer (WebSocket) | ~10-50ms |
| Gatherer → Local DB (write) | ~1-5ms |
| Poll interval | 100ms (average wait: 50ms) |
| Deduplicator → Production RDS | ~5-10ms |
| **Total** | **~70-115ms** |

### Sync Lag Metric

```promql
# Sync lag per gatherer per table
dedup_sync_lag_seconds{gatherer="gatherer-1", table="trades"}

# Max lag across all gatherers
max(dedup_sync_lag_seconds)
```

Normal: < 1 second
Warning: > 30 seconds
Critical: > 300 seconds (5 minutes)

---

## Failure Handling

### Gatherer Unreachable

```go
func (d *Deduplicator) pollGatherer(ctx context.Context, g *Gatherer, table TableConfig) []Record {
    records, err := g.Query(ctx, table.Name, g.cursor)
    if err != nil {
        d.logger.Warn("gatherer unreachable",
            "gatherer", g.ID,
            "error", err,
        )
        d.metrics.GathererErrors.WithLabelValues(g.ID).Inc()
        return nil  // Skip this gatherer, continue with others
    }
    return records
}
```

- Other gatherers continue to be polled
- Cursor is not advanced (will retry on next poll)
- Alert if gatherer is down for > 5 minutes

### Production RDS Unreachable

```go
func (d *Deduplicator) writeToProduction(ctx context.Context, table string, records []Record) error {
    for attempt := 0; attempt < 3; attempt++ {
        err := d.production.Insert(ctx, table, records)
        if err == nil {
            return nil
        }
        d.logger.Warn("production write failed",
            "attempt", attempt+1,
            "error", err,
        )
        time.Sleep(time.Duration(attempt+1) * time.Second)
    }
    return fmt.Errorf("production write failed after 3 attempts")
}
```

- Retry with backoff
- Gatherers continue to buffer data locally
- On recovery, deduplicator catches up from cursor

---

## Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `dedup_records_polled_total` | Counter | `gatherer`, `table` | Records polled from gatherers |
| `dedup_records_written_total` | Counter | `table` | Records written to production |
| `dedup_duplicates_total` | Counter | `table` | Duplicate records skipped |
| `dedup_sync_lag_seconds` | Gauge | `gatherer`, `table` | Sync lag per gatherer/table |
| `dedup_poll_duration_seconds` | Histogram | `gatherer`, `table` | Poll query duration |
| `dedup_gatherer_errors_total` | Counter | `gatherer` | Gatherer connection errors |
