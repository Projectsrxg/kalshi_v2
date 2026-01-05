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
- [~] WebSocket lifecycle message handling (stub only)
- [x] Unit tests (65.6% coverage)

### Connection Manager (`internal/connection/`)
- [ ] WebSocket connection pool
- [ ] 144 orderbook connections (up to 7,500 markets each = 1.08M capacity)
- [ ] 6 global connections (2 ticker, 2 trade, 2 lifecycle - with redundancy)
- [ ] Reconnection with exponential backoff
- [ ] Subscription management (subscribe/unsubscribe)
- [ ] Ping/pong keepalive
- [ ] Message routing to Message Router
- [ ] Unit tests

### Message Router (`internal/router/`)
- [ ] Message type detection and routing
- [ ] Non-blocking channel buffers
- [ ] Buffer overflow handling (drop oldest)
- [ ] Route to appropriate writers
- [ ] Metrics for dropped messages
- [ ] Unit tests

### Writers (`internal/writer/`)
- [ ] Orderbook delta writer
- [ ] Trade writer
- [ ] Ticker writer
- [ ] Snapshot writer
- [ ] Batch insert logic
- [ ] Flush interval timer
- [ ] Buffer management
- [ ] Unit tests

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
- [x] `internal/database` - 70.4% coverage
- [x] `internal/market` - 65.6% coverage
- [x] `internal/poller` - 98.4% coverage
- [x] `internal/version` - 100.0% coverage
- [ ] `internal/connection` - no tests
- [ ] `internal/router` - no tests
- [ ] `internal/writer` - no tests
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

1. **Connection Manager** - WebSocket pool for real-time data
2. **Message Router** - Route messages to writers
3. **Writers** - Persist data to TimescaleDB
4. **Database Migrations** - Create tables and hypertables
5. **Integrate components** in gatherer main loop
6. **Deduplicator** - Poll gatherers and deduplicate to production
7. **Metrics** - Prometheus instrumentation
8. **Deployment** - Terraform and production setup

---

## Recent Changes

### 2026-01-05
- Fixed initial sync timeout (30 minutes for paginated API calls)
- Health server now starts before initial sync for monitoring
- Initial sync filters to open + unopened markets only (excludes 1M+ settled/closed)
- Added pagination progress logging
