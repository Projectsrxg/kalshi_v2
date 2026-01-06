-- Kalshi Data Platform - Local Development Schema
-- This script runs automatically when the TimescaleDB container starts

-- Create database
CREATE DATABASE kalshi_ts;

-- Connect to the database
\c kalshi_ts

-- Enable TimescaleDB extension
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- =============================================================================
-- Trades Table
-- =============================================================================
CREATE TABLE trades (
    trade_id        UUID NOT NULL,
    exchange_ts     BIGINT NOT NULL,          -- Exchange timestamp (µs since epoch)
    received_at     BIGINT NOT NULL,          -- When gatherer received (µs since epoch)
    ticker          TEXT NOT NULL,
    price           INTEGER NOT NULL,          -- Hundred-thousandths (0-100000)
    size            INTEGER NOT NULL,          -- Number of contracts
    taker_side      BOOLEAN NOT NULL,          -- true = yes, false = no
    sid             BIGINT,                    -- Subscription ID (for debugging)
    PRIMARY KEY (trade_id, exchange_ts)       -- exchange_ts required for hypertable partitioning
);

SELECT create_hypertable('trades', 'exchange_ts',
    chunk_time_interval => 86400000000);  -- 1 day in microseconds

CREATE INDEX idx_trades_ticker ON trades (ticker, exchange_ts DESC);
CREATE INDEX idx_trades_received ON trades (received_at DESC);

-- =============================================================================
-- Orderbook Deltas Table
-- =============================================================================
CREATE TABLE orderbook_deltas (
    exchange_ts     BIGINT NOT NULL,          -- Exchange timestamp (µs since epoch)
    received_at     BIGINT NOT NULL,          -- When gatherer received (µs since epoch)
    ticker          TEXT NOT NULL,
    side            BOOLEAN NOT NULL,          -- true = yes, false = no
    price           INTEGER NOT NULL,          -- Hundred-thousandths (0-100000)
    size_delta      INTEGER NOT NULL,          -- Change in quantity (+/-)
    seq             BIGINT,                    -- Sequence number (per subscription)
    sid             BIGINT,                    -- Subscription ID (for debugging)
    PRIMARY KEY (exchange_ts, ticker, price, side)  -- exchange_ts first for hypertable partitioning
);

SELECT create_hypertable('orderbook_deltas', 'exchange_ts',
    chunk_time_interval => 3600000000);  -- 1 hour in microseconds

CREATE INDEX idx_deltas_ticker_time ON orderbook_deltas (ticker, exchange_ts DESC);
CREATE INDEX idx_deltas_received ON orderbook_deltas (received_at DESC);

-- =============================================================================
-- Orderbook Snapshots Table
-- =============================================================================
CREATE TABLE orderbook_snapshots (
    snapshot_ts     BIGINT NOT NULL,          -- When snapshot was taken (µs since epoch)
    exchange_ts     BIGINT,                   -- Exchange timestamp if from WS
    ticker          TEXT NOT NULL,
    source          TEXT NOT NULL,             -- 'ws' or 'rest'
    yes_bids        JSONB NOT NULL,            -- [[price, size], ...]
    yes_asks        JSONB NOT NULL,
    no_bids         JSONB NOT NULL,
    no_asks         JSONB NOT NULL,
    best_yes_bid    INTEGER,                   -- Best bid price
    best_yes_ask    INTEGER,                   -- Best ask price
    spread          INTEGER,                   -- Ask - bid
    sid             BIGINT,                    -- Subscription ID (for debugging)
    PRIMARY KEY (snapshot_ts, ticker, source)  -- snapshot_ts first for hypertable partitioning
);

SELECT create_hypertable('orderbook_snapshots', 'snapshot_ts',
    chunk_time_interval => 86400000000);  -- 1 day in microseconds

CREATE INDEX idx_snapshots_ticker ON orderbook_snapshots (ticker, snapshot_ts DESC);

-- =============================================================================
-- Tickers Table
-- =============================================================================
CREATE TABLE tickers (
    exchange_ts     BIGINT NOT NULL,          -- Exchange timestamp (µs since epoch)
    received_at     BIGINT NOT NULL,          -- When gatherer received (µs since epoch)
    ticker          TEXT NOT NULL,
    yes_bid         INTEGER,                   -- Best yes bid price
    yes_ask         INTEGER,                   -- Best yes ask price
    last_price      INTEGER,                   -- Last trade price
    volume          BIGINT,                    -- Total contracts traded today
    open_interest   BIGINT,                    -- Open contracts
    dollar_volume   BIGINT,                    -- Dollar volume (cents)
    dollar_open_interest BIGINT,               -- Dollar open interest (cents)
    sid             BIGINT,                    -- Subscription ID (for debugging)
    PRIMARY KEY (exchange_ts, ticker)          -- exchange_ts first for hypertable partitioning
);

SELECT create_hypertable('tickers', 'exchange_ts',
    chunk_time_interval => 3600000000);  -- 1 hour in microseconds

CREATE INDEX idx_tickers_ticker_time ON tickers (ticker, exchange_ts DESC);
CREATE INDEX idx_tickers_received ON tickers (received_at DESC);

-- =============================================================================
-- Sync Cursors Table (for deduplicator)
-- =============================================================================
CREATE TABLE sync_cursors (
    gatherer_id     VARCHAR(32) NOT NULL,
    table_name      VARCHAR(64) NOT NULL,
    last_sync_ts    BIGINT NOT NULL,          -- Last synced timestamp (µs)
    last_sync_at    TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (gatherer_id, table_name)
);

-- =============================================================================
-- Integer Now Function (required for retention policies with BIGINT time)
-- =============================================================================
-- TimescaleDB needs to know "now" for integer time dimensions
CREATE OR REPLACE FUNCTION unix_now_microseconds() RETURNS BIGINT
LANGUAGE SQL STABLE AS $$
    SELECT (EXTRACT(EPOCH FROM NOW()) * 1000000)::BIGINT;
$$;

-- Register the function with each hypertable
SELECT set_integer_now_func('trades', 'unix_now_microseconds');
SELECT set_integer_now_func('orderbook_deltas', 'unix_now_microseconds');
SELECT set_integer_now_func('orderbook_snapshots', 'unix_now_microseconds');
SELECT set_integer_now_func('tickers', 'unix_now_microseconds');

-- =============================================================================
-- Compression Policies
-- =============================================================================
-- Compress data older than 1 day for storage efficiency
-- Using integer microseconds: 1 day = 86400000000 µs

ALTER TABLE trades SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'ticker',
    timescaledb.compress_orderby = 'exchange_ts DESC'
);
SELECT add_compression_policy('trades', 86400000000::BIGINT);  -- 1 day in µs

ALTER TABLE orderbook_deltas SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'ticker',
    timescaledb.compress_orderby = 'exchange_ts DESC'
);
SELECT add_compression_policy('orderbook_deltas', 86400000000::BIGINT);  -- 1 day in µs

ALTER TABLE orderbook_snapshots SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'ticker',
    timescaledb.compress_orderby = 'snapshot_ts DESC'
);
SELECT add_compression_policy('orderbook_snapshots', 86400000000::BIGINT);  -- 1 day in µs

ALTER TABLE tickers SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'ticker',
    timescaledb.compress_orderby = 'exchange_ts DESC'
);
SELECT add_compression_policy('tickers', 86400000000::BIGINT);  -- 1 day in µs

-- =============================================================================
-- Retention Policies (local dev only - shorter retention for testing)
-- =============================================================================
-- Production uses longer retention (see data-model-production.md)
-- Using integer microseconds: 7 days = 604800000000 µs, 30 days = 2592000000000 µs

SELECT add_retention_policy('orderbook_deltas', 604800000000::BIGINT);   -- 7 days in µs
SELECT add_retention_policy('tickers', 604800000000::BIGINT);            -- 7 days in µs
SELECT add_retention_policy('orderbook_snapshots', 2592000000000::BIGINT); -- 30 days in µs
-- trades: no retention policy (keep all)

-- =============================================================================
-- Grant permissions
-- =============================================================================
-- In local dev, postgres user has full access
-- Production uses separate users (see deployment/startup.md)
