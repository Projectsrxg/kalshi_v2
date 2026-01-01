# Data Model (Production RDS)

TimescaleDB schema for the production database. Receives deduplicated data from all gatherers.

---

## Design Principles

1. **Append-only writes** — Never update time-series data, only insert
2. **Microsecond precision** — All timestamps as `BIGINT` (µs since epoch)
3. **Integer pricing** — Hundred-thousandths of a dollar (0-100,000) to avoid float errors
4. **TimescaleDB hypertables** — Automatic partitioning by time for time-series tables
5. **Full history** — Store all markets/events regardless of lifecycle

---

## Schema Overview

```mermaid
erDiagram
    series ||--o{ events : contains
    events ||--o{ markets : contains
    markets ||--o{ orderbook_deltas : has
    markets ||--o{ orderbook_snapshots : has
    markets ||--o{ trades : has
    markets ||--o{ tickers : has

    series {
        varchar ticker PK
        text title
        varchar category
        varchar frequency
        jsonb tags
        bigint updated_at
    }

    events {
        varchar event_ticker PK
        varchar series_ticker FK
        text title
        varchar category
        bigint created_ts
        bigint updated_at
    }

    markets {
        varchar ticker PK
        varchar event_ticker FK
        text title
        varchar status
        varchar result
        bigint open_ts
        bigint close_ts
        bigint updated_at
    }

    orderbook_deltas {
        bigint received_at
        bigint exchange_ts
        bigint sequence_num
        varchar ticker
        boolean side
        int price
        int size_delta
    }

    orderbook_snapshots {
        bigint snapshot_ts
        varchar ticker
        varchar source
        jsonb yes_bids
        jsonb yes_asks
    }

    trades {
        uuid trade_id PK
        bigint exchange_ts
        bigint received_at
        varchar ticker
        int price
        int size
        boolean taker_side
    }

    tickers {
        bigint received_at
        bigint exchange_ts
        varchar ticker
        int yes_bid
        int yes_ask
        int last_price
        bigint volume
        bigint open_interest
    }
```

---

## Utility Functions

### Price Conversion

Internal format: integer hundred-thousandths (0-100,000 = $0.00000-$1.00000)

| Internal | Dollars | Notes |
|----------|---------|-------|
| 52000 | $0.52 | 1 cent precision |
| 52500 | $0.525 | Subpenny (0.1 cent) |
| 52550 | $0.5255 | Subpenny (0.01 cent) |
| 99990 | $0.9999 | Tail pricing |

```sql
-- Display helper: internal to dollars
CREATE FUNCTION internal_to_dollars(internal INTEGER)
RETURNS NUMERIC(10,5) AS $$
    SELECT internal::NUMERIC / 100000;
$$ LANGUAGE SQL IMMUTABLE;
```

### Timestamp Conversion

```sql
-- Microseconds to timestamp
CREATE FUNCTION us_to_timestamp(us BIGINT)
RETURNS TIMESTAMPTZ AS $$
    SELECT to_timestamp(us / 1000000.0) AT TIME ZONE 'UTC';
$$ LANGUAGE SQL IMMUTABLE;

-- Timestamp to microseconds
CREATE FUNCTION timestamp_to_us(ts TIMESTAMPTZ)
RETURNS BIGINT AS $$
    SELECT (EXTRACT(EPOCH FROM ts) * 1000000)::BIGINT;
$$ LANGUAGE SQL IMMUTABLE;
```

---

## Relational Tables

### series

```sql
CREATE TABLE series (
    ticker              VARCHAR(128) PRIMARY KEY,
    title               TEXT,
    category            VARCHAR(64),
    frequency           VARCHAR(32),
    tags                JSONB,
    settlement_sources  JSONB,
    created_at          BIGINT NOT NULL,
    updated_at          BIGINT NOT NULL
);

CREATE INDEX idx_series_category ON series(category);
```

### events

```sql
CREATE TABLE events (
    event_ticker    VARCHAR(128) PRIMARY KEY,
    series_ticker   VARCHAR(128) REFERENCES series(ticker),
    title           TEXT,
    category        VARCHAR(64),
    sub_title       TEXT,
    mutually_exclusive BOOLEAN,
    created_ts      BIGINT,
    updated_at      BIGINT NOT NULL
);

CREATE INDEX idx_events_series ON events(series_ticker);
CREATE INDEX idx_events_category ON events(category);
```

### markets

Stores all markets regardless of lifecycle status.

```sql
CREATE TABLE markets (
    ticker          VARCHAR(128) PRIMARY KEY,
    event_ticker    VARCHAR(128) REFERENCES events(event_ticker),
    title           TEXT,
    subtitle        TEXT,

    -- Status (stored for all lifecycle stages)
    market_status   VARCHAR(16) NOT NULL,
    trading_status  VARCHAR(16) NOT NULL,
    market_type     VARCHAR(8) NOT NULL,
    result          VARCHAR(8),

    -- Volume
    volume          BIGINT,
    volume_24h      BIGINT,
    open_interest   BIGINT,

    -- Timing (µs since epoch)
    open_ts         BIGINT,
    close_ts        BIGINT,
    expiration_ts   BIGINT,
    created_ts      BIGINT,
    updated_at      BIGINT NOT NULL,

    CONSTRAINT valid_market_status CHECK (market_status IN ('unopened', 'open', 'closed', 'settled')),
    CONSTRAINT valid_market_type CHECK (market_type IN ('binary', 'scalar'))
);

CREATE INDEX idx_markets_event ON markets(event_ticker);
CREATE INDEX idx_markets_status ON markets(market_status);
CREATE INDEX idx_markets_created ON markets(created_ts DESC);
```

---

## Time-Series Tables (TimescaleDB)

### trades

```sql
CREATE TABLE trades (
    trade_id        UUID PRIMARY KEY,

    -- Timing (µs since epoch)
    exchange_ts     BIGINT NOT NULL,
    received_at     BIGINT NOT NULL,

    -- Market
    ticker          VARCHAR(128) NOT NULL,
    event_ticker    VARCHAR(128),

    -- Trade data
    price           INTEGER NOT NULL,      -- 0-100,000 (hundred-thousandths)
    size            INTEGER NOT NULL,
    taker_side      BOOLEAN NOT NULL       -- TRUE = YES, FALSE = NO
);

SELECT create_hypertable('trades', 'exchange_ts',
    chunk_time_interval => 86400000000);  -- 1 day in µs

CREATE INDEX idx_trades_ticker_time ON trades(ticker, exchange_ts DESC);
CREATE INDEX idx_trades_event_time ON trades(event_ticker, exchange_ts DESC);

-- Compression after 7 days
ALTER TABLE trades SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'ticker',
    timescaledb.compress_orderby = 'exchange_ts DESC'
);

SELECT add_compression_policy('trades', INTERVAL '7 days');
```

### orderbook_deltas

```sql
CREATE TABLE orderbook_deltas (
    -- Timing (µs since epoch)
    exchange_ts     BIGINT NOT NULL,       -- Kalshi exchange timestamp
    received_at     BIGINT NOT NULL,       -- When gatherer received

    -- Sequence
    seq             BIGINT NOT NULL,       -- Kalshi sequence number

    -- Market
    ticker          VARCHAR(128) NOT NULL,

    -- Delta data
    side            BOOLEAN NOT NULL,      -- TRUE = YES, FALSE = NO
    price           INTEGER NOT NULL,      -- 0-100,000
    size_delta      INTEGER NOT NULL,      -- Positive = add, negative = remove

    PRIMARY KEY (ticker, exchange_ts, seq, price, side)
);

SELECT create_hypertable('orderbook_deltas', 'exchange_ts',
    chunk_time_interval => 86400000000);  -- 1 day in µs

CREATE INDEX idx_orderbook_deltas_ticker_time ON orderbook_deltas(ticker, exchange_ts DESC);

-- Compression after 7 days
ALTER TABLE orderbook_deltas SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'ticker',
    timescaledb.compress_orderby = 'exchange_ts DESC'
);

SELECT add_compression_policy('orderbook_deltas', INTERVAL '7 days');
```

### orderbook_snapshots

Stores both WebSocket snapshots and 1-minute REST API snapshots.

```sql
CREATE TABLE orderbook_snapshots (
    -- Timing (µs since epoch)
    snapshot_ts     BIGINT NOT NULL,
    exchange_ts     BIGINT,

    -- Market
    ticker          VARCHAR(128) NOT NULL,

    -- Source
    source          VARCHAR(8) NOT NULL,   -- 'ws' or 'rest'

    -- Book data (JSONB for flexibility)
    yes_bids        JSONB NOT NULL,        -- [{price: int, size: int}, ...]
    yes_asks        JSONB NOT NULL,
    no_bids         JSONB NOT NULL,
    no_asks         JSONB NOT NULL,

    -- Derived fields (for fast filtering)
    best_yes_bid    INTEGER,
    best_yes_ask    INTEGER,
    spread          INTEGER,

    PRIMARY KEY (ticker, snapshot_ts, source)
);

SELECT create_hypertable('orderbook_snapshots', 'snapshot_ts',
    chunk_time_interval => 86400000000);  -- 1 day in µs

CREATE INDEX idx_ob_snapshots_ticker_time ON orderbook_snapshots(ticker, snapshot_ts DESC);
CREATE INDEX idx_ob_snapshots_source ON orderbook_snapshots(source, snapshot_ts DESC);

-- Compression after 7 days
ALTER TABLE orderbook_snapshots SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'ticker',
    timescaledb.compress_orderby = 'snapshot_ts DESC'
);

SELECT add_compression_policy('orderbook_snapshots', INTERVAL '7 days');
```

### tickers

```sql
CREATE TABLE tickers (
    -- Timing (µs since epoch)
    exchange_ts     BIGINT NOT NULL,       -- Kalshi exchange timestamp
    received_at     BIGINT NOT NULL,       -- When gatherer received

    -- Market
    ticker          VARCHAR(128) NOT NULL,

    -- Ticker data
    yes_bid         INTEGER,
    yes_ask         INTEGER,
    last_price      INTEGER,
    volume          BIGINT,
    open_interest   BIGINT,

    PRIMARY KEY (ticker, exchange_ts)
);

SELECT create_hypertable('tickers', 'exchange_ts',
    chunk_time_interval => 86400000000);  -- 1 day in µs

CREATE INDEX idx_tickers_ticker_time ON tickers(ticker, exchange_ts DESC);

-- Compression after 7 days
ALTER TABLE tickers SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'ticker',
    timescaledb.compress_orderby = 'exchange_ts DESC'
);

SELECT add_compression_policy('tickers', INTERVAL '7 days');
```

---

## Deduplication

Uses Kalshi's exchange-provided identifiers for deduplication. All inserts use `ON CONFLICT DO NOTHING`:

| Table | Primary Key | Source |
|-------|-------------|--------|
| `trades` | `trade_id` | Kalshi trade ID |
| `orderbook_deltas` | `(ticker, exchange_ts, seq, price, side)` | Kalshi timestamp + sequence |
| `orderbook_snapshots` | `(ticker, snapshot_ts, source)` | Snapshot timestamp + source |
| `tickers` | `(ticker, exchange_ts)` | Kalshi timestamp |
| `markets` | `ticker` | Upsert (DO UPDATE) |
| `events` | `event_ticker` | Upsert (DO UPDATE) |
| `series` | `ticker` | Upsert (DO UPDATE) |

---

## Retention Policies

| Table | Retention | Notes |
|-------|-----------|-------|
| `series` | Forever | Relational, small |
| `events` | Forever | Relational, small |
| `markets` | Forever | Relational, small |
| `trades` | Forever | Core data, compressed |
| `orderbook_deltas` | 90 days | High volume, export to S3 |
| `orderbook_snapshots` | Forever | 1-min resolution, compressed |
| `tickers` | 30 days | Can be derived, export to S3 |

```sql
-- Retention policies for high-volume tables
SELECT add_retention_policy('orderbook_deltas', INTERVAL '90 days');
SELECT add_retention_policy('tickers', INTERVAL '30 days');
```

---

## Storage Estimates

| Table | Row Size | Daily Rows | Daily Size | Compressed |
|-------|----------|------------|------------|------------|
| `trades` | ~100 bytes | 1M | ~100 MB | ~10 MB |
| `orderbook_deltas` | ~60 bytes | 10M | ~600 MB | ~60 MB |
| `orderbook_snapshots` | ~2 KB | 1.4M (1M markets × 1440 min) | ~2.8 GB | ~280 MB |
| `tickers` | ~50 bytes | 1M | ~50 MB | ~5 MB |

**Daily total (compressed):** ~355 MB/day

**Note:** orderbook_snapshots is the largest table due to 1-minute REST polling.
