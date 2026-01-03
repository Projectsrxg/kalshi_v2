# Data Model (Gatherer-Local)

TimescaleDB schema for individual gatherer instances. Production/Aurora schema documented separately.

---

## Design Principles

1. **Append-only writes** — Never update, only insert
2. **Microsecond precision** — All timestamps as `BIGINT` (µs since epoch)
3. **Integer pricing** — Hundred-thousandths of a dollar (0-100,000) to avoid float errors
4. **TimescaleDB hypertables** — Automatic partitioning by time

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
    }

    events {
        varchar event_ticker PK
        varchar series_ticker FK
        text title
        varchar category
        bigint created_ts
    }

    markets {
        varchar ticker PK
        varchar event_ticker FK
        text title
        varchar market_status
        varchar trading_status
        varchar result
        int yes_bid
        int yes_ask
        int last_price
        bigint volume
        bigint open_ts
        bigint close_ts
        bigint expiration_ts
    }

    orderbook_deltas {
        varchar ticker PK
        bigint exchange_ts PK
        bigint seq PK
        int price PK
        boolean side PK
        int size_delta
        bigint received_at
    }

    orderbook_snapshots {
        varchar ticker PK
        bigint snapshot_ts PK
        varchar source PK
        jsonb yes_bids
        jsonb yes_asks
        int best_yes_bid
        int best_yes_ask
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
        varchar ticker PK
        bigint exchange_ts PK
        bigint received_at
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

Kalshi API returns prices in both cent and dollar formats. Use `*_dollars` fields for subpenny precision.

Our internal format: integer hundred-thousandths (0-100,000 = $0.00000-$1.00000)

**Conversion (done by gatherer before storage):**

| Kalshi `price_dollars` | Internal | Notes |
|------------------------|----------|-------|
| `"0.52"` | 52000 | 1 cent precision |
| `"0.5250"` | 52500 | Subpenny (0.1 cent) |
| `"0.5255"` | 52550 | Subpenny (0.01 cent) |
| `"0.99"` | 99000 | Near 100% |
| `"0.9999"` | 99990 | Tail pricing |

```sql
-- Display helper: internal to dollars
-- 52500 -> 0.52500
CREATE FUNCTION internal_to_dollars(internal INTEGER)
RETURNS NUMERIC(10,5) AS $$
    SELECT internal::NUMERIC / 100000;
$$ LANGUAGE SQL IMMUTABLE;
```

### Timestamp Conversion

**Two timestamp fields in time-series tables:**

| Field | Source | Description |
|-------|--------|-------------|
| `exchange_ts` | Kalshi API `ts` field | When Kalshi processed the event (server-side) |
| `received_at` | Gatherer `time.Now()` | When gatherer received the message (client-side) |

Both stored in microseconds (µs since Unix epoch).

**Use cases:**
- `exchange_ts`: Ordering events, deduplication, analytics
- `received_at`: Debugging latency, identifying gatherer delays

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
    ticker          VARCHAR(128) PRIMARY KEY,
    title           TEXT,
    category        VARCHAR(64),
    frequency       VARCHAR(32),
    tags            JSONB,
    settlement_sources JSONB,
    updated_at      BIGINT NOT NULL
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
    created_ts      BIGINT,
    updated_at      BIGINT NOT NULL
);

CREATE INDEX idx_events_series ON events(series_ticker);
```

### markets

```sql
CREATE TABLE markets (
    ticker          VARCHAR(128) PRIMARY KEY,
    event_ticker    VARCHAR(128) REFERENCES events(event_ticker),
    title           TEXT,
    subtitle        TEXT,

    -- Status (see "Market Status Values" section below)
    market_status   VARCHAR(16) NOT NULL,
    trading_status  VARCHAR(16) NOT NULL,
    market_type     VARCHAR(8) NOT NULL,
    result          VARCHAR(8),

    -- Current prices (from REST, updated periodically)
    yes_bid         INTEGER,
    yes_ask         INTEGER,
    last_price      INTEGER,

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

    CONSTRAINT valid_market_status CHECK (market_status IN ('initialized', 'inactive', 'active', 'closed', 'determined', 'disputed', 'amended', 'finalized')),
    CONSTRAINT valid_market_type CHECK (market_type IN ('binary', 'scalar'))
);

CREATE INDEX idx_markets_event ON markets(event_ticker);
CREATE INDEX idx_markets_status ON markets(market_status, trading_status);
```

### Market Status Values

The Kalshi API uses two different status representations:

#### 1. Market Status (stored field)

The `market_status` field contains the granular lifecycle state:

| Status | Description |
|--------|-------------|
| `initialized` | Market created, not yet active |
| `inactive` | Temporarily inactive |
| `active` | Open for trading |
| `closed` | Trading closed, awaiting settlement |
| `determined` | Outcome determined |
| `disputed` | Settlement disputed |
| `amended` | Settlement amended |
| `finalized` | Fully settled, final |

#### 2. Production Status (simplified)

The production database uses simplified statuses. Deduplicator maps gatherer statuses:

| Production Status | Gatherer Statuses |
|-------------------|-------------------|
| `unopened` | `initialized`, `inactive` |
| `open` | `active` |
| `closed` | `closed`, `disputed` |
| `settled` | `determined`, `amended`, `finalized` |

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
    event_ticker    VARCHAR(128),          -- Nullable, not populated by gatherer

    -- Trade data
    price           INTEGER NOT NULL,      -- 0-100,000 (hundred-thousandths)
    size            INTEGER NOT NULL,
    taker_side      BOOLEAN NOT NULL,      -- TRUE = YES, FALSE = NO

    -- Metadata
    sid             BIGINT                 -- Subscription ID for debugging
);

SELECT create_hypertable('trades', 'exchange_ts',
    chunk_time_interval => 86400000000);  -- 1 day in µs

CREATE INDEX idx_trades_ticker_time ON trades(ticker, exchange_ts DESC);

-- Compression after 1 day
ALTER TABLE trades SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'ticker',
    timescaledb.compress_orderby = 'exchange_ts DESC'
);

SELECT add_compression_policy('trades', INTERVAL '1 day');
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

    -- Metadata
    sid             BIGINT,                -- Subscription ID for debugging

    PRIMARY KEY (ticker, exchange_ts, seq, price, side)
);

SELECT create_hypertable('orderbook_deltas', 'exchange_ts',
    chunk_time_interval => 3600000000);  -- 1 hour in µs

CREATE INDEX idx_orderbook_ticker_time ON orderbook_deltas(ticker, exchange_ts DESC);

-- Compression after 1 day, retention 7 days
ALTER TABLE orderbook_deltas SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'ticker',
    timescaledb.compress_orderby = 'exchange_ts DESC'
);

SELECT add_compression_policy('orderbook_deltas', INTERVAL '1 day');
SELECT add_retention_policy('orderbook_deltas', INTERVAL '7 days');
```

### orderbook_snapshots

Full orderbook state from WebSocket snapshots and 1-minute REST polling.

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

    -- Metadata
    sid             BIGINT,                -- Subscription ID (0 for REST)

    PRIMARY KEY (ticker, snapshot_ts, source)
);

SELECT create_hypertable('orderbook_snapshots', 'snapshot_ts',
    chunk_time_interval => 3600000000);  -- 1 hour in µs

CREATE INDEX idx_ob_snapshots_ticker ON orderbook_snapshots(ticker, snapshot_ts DESC);
CREATE INDEX idx_ob_snapshots_source ON orderbook_snapshots(source, snapshot_ts DESC);

-- Compression after 1 day, retention 30 days
ALTER TABLE orderbook_snapshots SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'ticker',
    timescaledb.compress_orderby = 'snapshot_ts DESC'
);

SELECT add_compression_policy('orderbook_snapshots', INTERVAL '1 day');
SELECT add_retention_policy('orderbook_snapshots', INTERVAL '30 days');
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
    dollar_volume   BIGINT,                -- Dollar-denominated volume
    dollar_open_interest BIGINT,           -- Dollar-denominated open interest

    -- Metadata
    sid             BIGINT,                -- Subscription ID for debugging

    PRIMARY KEY (ticker, exchange_ts)
);

SELECT create_hypertable('tickers', 'exchange_ts',
    chunk_time_interval => 3600000000);  -- 1 hour in µs

CREATE INDEX idx_tickers_ticker_time ON tickers(ticker, exchange_ts DESC);

-- Compression after 1 day, retention 7 days
ALTER TABLE tickers SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'ticker',
    timescaledb.compress_orderby = 'exchange_ts DESC'
);

SELECT add_compression_policy('tickers', INTERVAL '1 day');
SELECT add_retention_policy('tickers', INTERVAL '7 days');
```

---

## Deduplication Keys

Uses Kalshi's exchange-provided identifiers for deduplication:

| Table | Primary Key | Conflict Handling |
|-------|-------------|-------------------|
| `trades` | `trade_id` | `ON CONFLICT DO NOTHING` |
| `orderbook_deltas` | `(ticker, exchange_ts, seq, price, side)` | `ON CONFLICT DO NOTHING` |
| `orderbook_snapshots` | `(ticker, snapshot_ts, source)` | `ON CONFLICT DO NOTHING` |
| `tickers` | `(ticker, exchange_ts)` | `ON CONFLICT DO NOTHING` |
| `markets` | `ticker` | `ON CONFLICT DO UPDATE` |
| `events` | `event_ticker` | `ON CONFLICT DO UPDATE` |

---

## Storage Estimates

| Table | Row Size | Daily Rows (Peak) | Daily Size |
|-------|----------|-------------------|------------|
| `trades` | ~80 bytes | 1M | ~80 MB |
| `orderbook_deltas` | ~60 bytes | 10M | ~600 MB |
| `orderbook_snapshots` | ~2 KB | 100K | ~200 MB |
| `tickers` | ~50 bytes | 1M | ~50 MB |

**With 10x compression:** ~100 MB/day per gatherer

---

## Queries

### Latest orderbook state

```sql
SELECT price, SUM(size_delta) as quantity, side
FROM orderbook_deltas
WHERE ticker = $1
  AND received_at >= (
    SELECT snapshot_ts FROM orderbook_snapshots
    WHERE ticker = $1
    ORDER BY snapshot_ts DESC
    LIMIT 1
  )
GROUP BY price, side
HAVING SUM(size_delta) > 0
ORDER BY price DESC;
```

### Recent trades

```sql
SELECT trade_id, us_to_timestamp(exchange_ts) as ts, size,
       internal_to_dollars(price) as price, taker_side
FROM trades
WHERE ticker = $1
ORDER BY exchange_ts DESC
LIMIT 100;
```

### Market activity (last 24h)

```sql
SELECT
    m.ticker,
    m.title,
    COUNT(t.trade_id) as trade_count,
    SUM(t.size) as total_volume
FROM markets m
LEFT JOIN trades t ON t.ticker = m.ticker
    AND t.exchange_ts > (timestamp_to_us(NOW()) - 86400000000)
WHERE m.market_status = 'open'
GROUP BY m.ticker, m.title
ORDER BY total_volume DESC
LIMIT 100;
```
