# Configuration

Config options and metrics for Writers.

---

## Config Struct

```go
type WriterConfig struct {
    // Batching
    BatchSize     int           // Messages per batch insert
    FlushInterval time.Duration // Max time before flush

    // Database
    DBConnPoolSize int // Connection pool size
}

// Per-writer configs can vary
type OrderbookWriterConfig struct {
    WriterConfig
    DeltaBatchSize    int // Separate batch size for deltas
    SnapshotBatchSize int // Separate batch size for snapshots
}
```

---

## Options

### Common Options (all writers)

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `BatchSize` | int | 100 | Messages per batch insert |
| `FlushInterval` | duration | 100ms | Max time before triggering flush |
| `DBConnPoolSize` | int | 10 | PostgreSQL connection pool size |

### Orderbook Writer

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `DeltaBatchSize` | int | 200 | Deltas per batch (higher volume) |
| `SnapshotBatchSize` | int | 50 | Snapshots per batch |

### Trade Writer

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `BatchSize` | int | 100 | Trades per batch |

### Ticker Writer

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `BatchSize` | int | 100 | Ticker updates per batch |

### Snapshot Writer

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| - | - | - | No batching - writes immediately on each REST poll |

---

## Tuning Guidelines

### Batch Size

**Trade-off:** Larger batches = fewer DB round-trips, but higher memory and latency.

| Scenario | Recommended BatchSize |
|----------|----------------------|
| Low volume (<100 msg/s) | 50 |
| Normal volume (100-1000 msg/s) | 100 |
| High volume (>1000 msg/s) | 200-500 |

### Flush Interval

**Trade-off:** Shorter intervals = lower latency, but more DB overhead.

| Scenario | Recommended FlushInterval |
|----------|---------------------------|
| Real-time required | 50ms |
| Normal operation | 100ms |
| Batch-oriented | 500ms |

### Connection Pool Size

**Rule of thumb:** 2-3x the number of writer goroutines.

| Writers | Recommended Pool Size |
|---------|----------------------|
| 4 writers (default) | 10-15 |
| High throughput | 20-30 |

---

## Error Handling

| Error | Behavior |
|-------|----------|
| DB connection error | Log error, increment metric, retry on next flush |
| Batch insert failure | Log error, drop batch, continue (data recovered via deduplication) |
| Individual row failure | Skip row, log warning, continue batch |
| Channel buffer full | Router drops message (see Message Router docs) |

**Recovery strategy:** Rely on redundancy:
- 3 independent gatherers capture same data
- REST snapshot polling provides 1-minute backup
- Deduplicator merges all sources

---

## Metrics

### Per-Writer Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `writer_messages_received_total` | Counter | `writer` | Messages received from channel |
| `writer_inserts_total` | Counter | `writer` | Rows successfully inserted |
| `writer_conflicts_total` | Counter | `writer` | Duplicates (ON CONFLICT hit) |
| `writer_errors_total` | Counter | `writer`, `type` | Insert failures |
| `writer_batch_size` | Histogram | `writer` | Rows per batch |
| `writer_flush_duration_seconds` | Histogram | `writer` | Time to execute batch insert |
| `writer_seq_gaps_total` | Counter | `writer`, `ticker` | Sequence gaps detected (orderbook only) |

### Labels

| Label | Values |
|-------|--------|
| `writer` | `orderbook`, `trade`, `ticker`, `snapshot` |
| `type` | `connection`, `constraint`, `timeout` |

### Histogram Buckets

| Metric | Buckets |
|--------|---------|
| `writer_batch_size` | `[10, 50, 100, 500, 1000, 5000]` |
| `writer_flush_duration_seconds` | `[0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0]` |

**Note:** Snapshot Writer is synchronous and doesn't batch, so it doesn't emit `writer_batch_size` metrics. Only `orderbook`, `trade`, and `ticker` writers emit batch metrics.

---

## Example Prometheus Queries

```promql
# Insert throughput per writer
sum(rate(writer_inserts_total[5m])) by (writer)

# Duplicate rate (expected to be non-zero with 3 gatherers)
rate(writer_conflicts_total[5m]) / rate(writer_inserts_total[5m])

# Average batch size
histogram_quantile(0.5, sum(rate(writer_batch_size_bucket[5m])) by (le, writer))

# P99 flush latency
histogram_quantile(0.99, sum(rate(writer_flush_duration_seconds_bucket[5m])) by (le, writer))

# Error rate
rate(writer_errors_total[5m])
```

---

## Alerting Thresholds

| Condition | Threshold | Action |
|-----------|-----------|--------|
| Error rate > 1% | `rate(writer_errors_total[5m]) > 0.01` | Check DB connectivity |
| Flush latency P99 > 1s | `histogram_quantile(0.99, ...) > 1` | Increase pool size or batch size |
| Backlog growing | Router drops increasing | Increase writer throughput |

---

## Database Requirements

### TimescaleDB

Writers require TimescaleDB for hypertable support:

```sql
-- Required extension
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- Hypertables with compression
SELECT create_hypertable('trades', 'exchange_ts', chunk_time_interval => 86400000000);
SELECT create_hypertable('orderbook_deltas', 'exchange_ts', chunk_time_interval => 3600000000);
SELECT create_hypertable('orderbook_snapshots', 'snapshot_ts', chunk_time_interval => 3600000000);
SELECT create_hypertable('tickers', 'exchange_ts', chunk_time_interval => 3600000000);
```

### Connection String

```go
// Example connection config
dbURL := "postgres://user:pass@localhost:5432/kalshi_data?pool_max_conns=10"
```

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `WRITER_BATCH_SIZE` | 100 | Override default batch size |
| `WRITER_FLUSH_INTERVAL` | 100ms | Override flush interval |
| `DB_POOL_SIZE` | 10 | Connection pool size |
| `DB_URL` | - | TimescaleDB connection string |
