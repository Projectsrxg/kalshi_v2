# Implementation Checklist

This document tracks the implementation status of all components in the Kalshi Data Platform.

## Legend
- [x] Completed
- [ ] Not started
- [~] Partially implemented

---

## Infrastructure & Setup

### Build & Configuration
- [x] Makefile with build targets
- [x] Docker Compose for local development (TimescaleDB, MinIO)
- [x] Configuration loading with YAML + env var substitution
- [x] Configuration validation
- [x] Configuration defaults
- [x] Example config files (`gatherer.example.yaml`, `deduplicator.example.yaml`)

### Database
- [x] Connection pool management (`internal/database/pools.go`)
- [x] Connection string building
- [x] SSL mode support
- [ ] Database migrations (schema creation)
- [ ] TimescaleDB hypertable setup

---

## Gatherer Components

### API Client (`internal/api/`)
- [x] REST client with retries and backoff
- [x] Exchange status endpoint
- [x] Markets endpoint (single + paginated)
- [x] Events endpoint (single + paginated)
- [x] Series endpoint
- [x] Orderbook endpoint
- [x] Price conversion (string dollars to integer hundred-thousandths)
- [x] API types and response models
- [x] Unit tests (96.2% coverage)

### Market Registry (`internal/market/`)
- [x] Interface definition
- [x] In-memory state management
- [x] Initial sync from REST API (open + unopened markets)
- [x] Periodic reconciliation loop
- [x] Market change notifications (channel-based)
- [x] Active market filtering
- [x] WebSocket lifecycle message handling (created, status_change, settled)
- [x] Unit tests (75.7% coverage)

### Connection Manager (`internal/connection/`)
- [x] WebSocket client (low-level connection handling)
- [x] WebSocket connection pool
- [x] 144 orderbook connections (up to 7,500 markets each = 1.08M capacity)
- [x] 6 global connections (2 ticker, 2 trade, 2 lifecycle - with redundancy)
- [x] Reconnection with exponential backoff
- [x] Subscription management (subscribe/unsubscribe)
- [x] Ping/pong keepalive
- [x] Sequence gap detection
- [x] Command/response correlation
- [x] Message channel output (for Message Router integration)
- [x] Unit tests (66.1% coverage)

### Message Router (`internal/router/`)
- [x] Message type detection and routing
- [x] GrowableBuffer (auto-doubles at 70% capacity, never drops)
- [x] Route to orderbook, trade, ticker buffers
- [x] Timestamp conversion (Unix seconds → microseconds)
- [x] Sequence gap pass-through from Connection Manager
- [x] Stats tracking (received, routed, parse errors)
- [x] Unit tests (84.1% coverage)

### Writers (`internal/writer/`)
- [x] Trade writer (batch insert with ON CONFLICT DO NOTHING)
- [x] Orderbook delta writer (batch insert)
- [x] Orderbook snapshot writer (WS snapshots with derived asks)
- [x] Ticker writer (batch insert)
- [x] Price conversion (dollars → hundred-thousandths)
- [x] Side conversion (yes/no → boolean)
- [x] Batch insert logic with pgx.Batch
- [x] Flush interval timer
- [x] Unit tests (62.3% coverage)

### Snapshot Poller (`internal/poller/`)
- [x] Poller interface and implementation
- [x] Concurrent orderbook fetching
- [x] Rate limiting
- [x] Unit tests (98.4% coverage)
- [ ] Integration with gatherer main loop

### Metrics (`internal/metrics/`)
- [ ] Prometheus metrics definitions
- [ ] WebSocket connection metrics
- [ ] Message processing metrics
- [ ] Buffer overflow metrics
- [ ] Database write metrics
- [ ] HTTP metrics endpoint

### Gatherer Main (`cmd/gatherer/`)
- [x] Configuration loading
- [x] Database connection
- [x] Market Registry startup
- [x] Health server (early startup for monitoring)
- [x] Graceful shutdown handling
- [ ] Connection Manager integration
- [ ] Message Router integration
- [ ] Writers integration
- [ ] Snapshot Poller integration
- [ ] Metrics server integration

---

## Deduplicator Components

### Deduplicator (`internal/dedup/`)
- [ ] Cursor-based sync from gatherer databases
- [ ] Deduplication logic per table type
- [ ] Trade deduplication (by trade_id)
- [ ] Orderbook delta deduplication (by ticker, exchange_ts, price, side)
- [ ] Snapshot deduplication (by ticker, snapshot_ts, source)
- [ ] Ticker deduplication (by ticker, exchange_ts)
- [ ] Write to production RDS
- [ ] Unit tests

### S3 Export
- [ ] Periodic export to S3
- [ ] Parquet file format
- [ ] Partitioning by date/market

### Deduplicator Main (`cmd/deduplicator/`)
- [ ] Configuration loading
- [ ] Connect to gatherer databases (3x)
- [ ] Connect to production RDS
- [ ] Cursor-based sync polling loop
- [ ] Deduplication workers
- [ ] S3 export (optional)
- [ ] Metrics server
- [ ] Health server
- [ ] Graceful shutdown

---

## Data Model (`internal/model/`)
- [x] Market type
- [x] Trade type
- [x] OrderbookDelta type
- [x] OrderbookSnapshot type
- [x] Ticker type
- [x] Price conversion helpers
- [x] Timestamp helpers (microseconds)

---

## Testing

### Unit Tests
- [x] `internal/api` - 96.2% coverage
- [x] `internal/config` - 100.0% coverage
- [x] `internal/connection` - 66.1% coverage
- [x] `internal/database` - 70.4% coverage
- [x] `internal/market` - 75.7% coverage
- [x] `internal/poller` - 98.4% coverage
- [x] `internal/version` - 100.0% coverage
- [x] `internal/router` - 84.1% coverage
- [x] `internal/writer` - 62.3% coverage
- [ ] `internal/metrics` - no tests
- [ ] `internal/dedup` - no tests

### Integration Tests
- [ ] End-to-end gatherer test
- [ ] End-to-end deduplicator test
- [ ] WebSocket connection test
- [ ] Database write test

---

## Documentation
- [x] CLAUDE.md (project overview)
- [x] Architecture documentation (`docs/kalshi-data/architecture/`)
- [x] API documentation (`docs/kalshi-api/`)
- [x] Deployment documentation (`docs/kalshi-data/deployment/`)
- [x] Local development guide (`docs/kalshi-data/development/`)
- [x] Implementation checklist (this file)

---

## Deployment
- [ ] Terraform infrastructure (AWS)
- [ ] EC2 instances for gatherers (3x AZs)
- [ ] RDS for production database
- [ ] S3 bucket for exports
- [ ] Systemd service files
- [ ] CloudWatch alarms
- [ ] Grafana dashboards

---

## Next Steps (Priority Order)

1. **Database Migrations** - Create tables and hypertables
2. **Integrate components** in gatherer main loop (Connection Manager + Router + Writers)
3. **Deduplicator** - Poll gatherers and deduplicate to production
4. **Metrics** - Prometheus instrumentation
5. **Deployment** - Terraform and production setup

---

## Recent Changes

### 2026-01-05 (continued)
- **Updated Message Router** to use GrowableBuffer instead of channels:
  - GrowableBuffer automatically doubles capacity at 70% full
  - Never drops messages - guarantees delivery to Writers
  - Blocking Receive() and non-blocking TryReceive() support
  - Thread-safe with mutex + condition variable pattern
- **Implemented Writers** (`internal/writer/`):
  - TradeWriter: Batch inserts to `trades` table with deduplication by `trade_id`
  - OrderbookWriter: Handles both deltas and snapshots
    - Deltas: Batch inserts with deduplication by `(ticker, exchange_ts, price, side)`
    - Snapshots: Derives asks from opposite-side bids, stores as JSONB
  - TickerWriter: Batch inserts with deduplication by `(ticker, exchange_ts)`
  - Price conversion: `"0.52"` → `52000` (hundred-thousandths)
  - Side conversion: `"yes"`/`"no"` → `true`/`false`
  - Configurable batch size and flush interval
- **Added unit tests** for writer package (62.3% coverage)

### 2026-01-05
- Fixed initial sync timeout (30 minutes for paginated API calls)
- Health server now starts before initial sync for monitoring
- Initial sync filters to open + unopened markets only (excludes 1M+ settled/closed)
- Added pagination progress logging
- **Implemented Connection Manager** (`internal/connection/`):
  - WebSocket client with ping/pong keepalive
  - Connection pool (144 orderbook + 6 global connections)
  - Subscription management with least-loaded assignment
  - Command/response correlation for subscribe/unsubscribe
  - Sequence gap detection for orderbook messages
  - Reconnection with exponential backoff
- **Completed WebSocket lifecycle message handling** in Market Registry:
  - Parse `market_lifecycle` WebSocket messages
  - Handle `created`, `status_change`, and `settled` events
  - Fetch new market details from REST API on create
  - Update active market set on status changes
- **Added comprehensive unit tests** for connection package (66.1% coverage)
- **Implemented Message Router** (`internal/router/`):
  - Parses orderbook_snapshot, orderbook_delta, trade, ticker messages
  - Routes to typed output channels for Writers
  - Non-blocking sends with buffer overflow handling (drop + log)
  - Timestamp conversion (Unix seconds → microseconds)
  - Sequence gap pass-through for recovery signaling
  - Stats tracking for monitoring
- **Added comprehensive unit tests** for router package (71.9% coverage)
