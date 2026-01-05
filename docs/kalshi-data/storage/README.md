# Storage

Persistence layer for the Kalshi Data Platform.

---

## Overview

The platform uses a two-tier storage architecture:

```mermaid
flowchart TD
    subgraph Gatherer["Gatherer (x3)"]
        TS[(TimescaleDB\nTime-series)]
    end

    subgraph Production
        RDS[(Production RDS\nTimescaleDB)]
        S3[(S3\nCold Storage)]
    end

    KALSHI[Kalshi API] --> DEDUP
    TS --> DEDUP[Deduplicator]
    DEDUP --> RDS
    RDS --> EXPORT[Export Job]
    EXPORT --> S3
```

| Tier | Purpose | Technology | Retention |
|------|---------|------------|-----------|
| Gatherer-local | Buffer, redundancy | TimescaleDB | 7-30 days |
| Production | Authoritative source | TimescaleDB on RDS | Varies by table |
| Cold storage | Archive, analytics | Parquet on S3 | Forever |

---

## Storage Components

### Gatherer-Local Storage

Each gatherer has one database:

| Database | Purpose | Tables |
|----------|---------|--------|
| TimescaleDB | Time-series data | trades, orderbook_deltas, orderbook_snapshots, tickers |

Market metadata (series, events, markets) is kept in-memory by the Market Registry and synced to production RDS by the Deduplicator directly from the Kalshi API.

**Characteristics:**
- Append-only writes (no updates to time-series)
- Short retention (data moves to production via deduplicator)
- 3 independent copies for redundancy
- ON CONFLICT DO NOTHING for deduplication

### Production RDS

Single authoritative database receiving deduplicated data.

| Database | Purpose | Tables |
|----------|---------|--------|
| TimescaleDB | All data types | series, events, markets, trades, orderbook_deltas, orderbook_snapshots, tickers |

**Characteristics:**
- Receives deduplicated data from all gatherers
- Longer retention with compression
- Read replicas for analytics
- Automated backups

### S3 Cold Storage

Parquet export for long-term archive and analytics.

| Bucket | Data | Format | Lifecycle |
|--------|------|--------|-----------|
| `raw/` | Unprocessed exports | Parquet | Glacier after 30 days |
| `processed/` | Cleaned, validated | Parquet | IA after 90 days |
| `aggregates/` | Pre-computed analytics | Parquet | Standard |

---

## Data Flow

### Write Path

```mermaid
sequenceDiagram
    participant WS as WebSocket
    participant W as Writer
    participant TS as Local TimescaleDB
    participant D as Deduplicator
    participant RDS as Production RDS
    participant S3 as S3

    WS->>W: Message
    W->>W: Batch (100 rows)
    W->>TS: INSERT ... ON CONFLICT DO NOTHING
    D->>TS: Poll (WHERE received_at > cursor)
    D->>RDS: INSERT ... ON CONFLICT DO NOTHING
    RDS->>S3: Export (hourly/daily)
```

### Read Path

```mermaid
flowchart LR
    subgraph Sources
        RDS[(Production RDS)]
        S3[(S3)]
    end

    subgraph Consumers
        API[API Server]
        ANALYTICS[Analytics]
        ML[ML Pipelines]
    end

    RDS --> API
    RDS --> ANALYTICS
    S3 --> ML
    S3 --> ANALYTICS
```

---

## Connection Architecture

### Per-Gatherer Connections

| Component | Connections | Purpose |
|-----------|-------------|---------|
| Orderbook Writer | 2-3 | Batch inserts |
| Trade Writer | 2-3 | Batch inserts |
| Ticker Writer | 2-3 | Batch inserts |
| Snapshot Writer | 1-2 | REST snapshot inserts |

**Total per gatherer:** 7-11 connections

Note: Market Registry uses in-memory storage only (no database connections).

### Deduplicator Connections

| Target | Connections | Mode |
|--------|-------------|------|
| Gatherer 1 TimescaleDB | 2 | Read-only |
| Gatherer 2 TimescaleDB | 2 | Read-only |
| Gatherer 3 TimescaleDB | 2 | Read-only |
| Kalshi REST API | 1 | Read-only |
| Production RDS | 4-6 | Read-write |

**Total:** 11-13 connections

Note: Market metadata (series, events, markets) is synced directly from Kalshi API to Production RDS, not from gatherers.

---

## Key Design Decisions

### Why TimescaleDB?

| Requirement | TimescaleDB Feature |
|-------------|---------------------|
| Time-series optimized | Hypertables with automatic chunking |
| High insert throughput | Parallel inserts, batch optimization |
| Compression | 10x compression for historical data |
| Retention policies | Automatic data expiration |
| SQL compatibility | Standard PostgreSQL queries |

### Why Separate Gatherer Databases?

| Reason | Benefit |
|--------|---------|
| Independence | Each gatherer can fail independently |
| Locality | Low-latency writes (same AZ) |
| Redundancy | 3 complete copies of all data |
| Recovery | Any gatherer can backfill others |

### Why Deduplication at Write?

All data has exchange-provided unique identifiers (trade_id, exchange_ts + seq).
Using `ON CONFLICT DO NOTHING` is:
- Idempotent: Safe to retry
- Efficient: No read-before-write
- Simple: No external dedup service

---

## Documentation

| Document | Content |
|----------|---------|
| [Configuration](./configuration.md) | Connection strings, pool settings, tuning |
| [TimescaleDB](./timescaledb.md) | Hypertables, compression, retention policies |
| [Operations](./operations.md) | Backup, restore, migrations, maintenance |
| [S3 Export](./s3.md) | Cold storage format, lifecycle, querying |

---

## Schema References

For detailed schema documentation:

- [Data Model (Gatherer)](../architecture/data-model.md) - Gatherer-local schema
- [Data Model (Production)](../architecture/data-model-production.md) - Production RDS schema
