# Local Development

Local development environment that mirrors production architecture on a single machine.

---

## Overview

| Aspect | Production | Local Dev |
|--------|------------|-----------|
| Gatherers | 3 EC2 instances | 1 process |
| WebSocket connections | 450 (3 × 150) | 150 |
| Deduplicator | 1 EC2 instance | Not needed (single source) |
| TimescaleDB | Per-gatherer + RDS | 1 Docker container |
| S3 | AWS S3 | MinIO or local filesystem |
| Secrets | AWS Secrets Manager | `.env` file |
| Kalshi API | Production | Production (same) |
| Monitoring | Prometheus + Grafana | Optional Docker stack |

```mermaid
flowchart TB
    subgraph External["Kalshi API"]
        KALSHI[api.elections.kalshi.com]
    end

    subgraph Local["Local Machine"]
        subgraph Docker["Docker Compose"]
            TS[("TimescaleDB<br/>:5432<br/>kalshi_ts")]
            MINIO[("MinIO<br/>:9000<br/>S3-compatible")]
            PROM[Prometheus<br/>:9090]
            GRAF[Grafana<br/>:3000]
        end

        subgraph Process["Go Processes"]
            GATHERER[gatherer<br/>:8080]
            DEDUP[deduplicator<br/>:8081]
        end
    end

    KALSHI -->|WebSocket + REST| GATHERER
    GATHERER --> TS
    DEDUP --> TS
    DEDUP --> MINIO
    GATHERER -.->|metrics| PROM
    DEDUP -.->|metrics| PROM
    PROM --> GRAF
```

---

## Quick Start

### Prerequisites

```bash
# Required
brew install go docker docker-compose

# Optional (for database inspection)
brew install postgresql  # for psql client
```

### 1. Start Infrastructure

```bash
# Clone and navigate
cd kalshi-data

# Start Docker containers
docker-compose up -d

# Verify containers are running
docker-compose ps
# Should show: timescaledb, minio (healthy)
```

### 2. Configure Environment

```bash
# Copy example env file
cp .env.example .env

# Edit with your Kalshi API credentials
# Get API key from: https://kalshi.com/account/api
vim .env
```

**.env file:**
```bash
# Kalshi API (production API works for read-only data collection)
KALSHI_API_URL=https://api.elections.kalshi.com/trade-api/v2
KALSHI_WS_URL=wss://api.elections.kalshi.com
KALSHI_API_KEY=your-api-key
KALSHI_API_SECRET=your-api-secret

# Local database
TIMESCALEDB_URL=postgres://postgres:postgres@localhost:5432/kalshi_ts?sslmode=disable

# Local S3 (MinIO)
S3_ENDPOINT=http://localhost:9000
S3_ACCESS_KEY=minioadmin
S3_SECRET_KEY=minioadmin
S3_BUCKET=kalshi-data-local

# Development settings
LOG_LEVEL=debug
BATCH_SIZE=100
FLUSH_INTERVAL=1s
```

### 3. Run Migrations

```bash
# Run database migrations
make migrate-up

# Or manually:
psql $TIMESCALEDB_URL -f migrations/local/001_schema.sql
```

### 4. Start Gatherer

```bash
# Run gatherer
make run-gatherer

# Or:
go run ./cmd/gatherer --config configs/local/gatherer.yaml
```

### 5. Verify Data Flow

```bash
# Check health
curl localhost:8080/health | jq

# Check data is flowing (after a minute)
psql $TIMESCALEDB_URL -c "SELECT COUNT(*) FROM trades;"

# Watch trades in real-time (exchange_ts is in µs)
psql $TIMESCALEDB_URL -c "
  SELECT ticker, price, size, exchange_ts
  FROM trades ORDER BY exchange_ts DESC LIMIT 10;
"
```

---

## Docker Compose

### docker-compose.yml

```yaml
version: '3.8'

services:
  timescaledb:
    image: timescale/timescaledb:latest-pg16
    container_name: kalshi-timescaledb
    ports:
      - "5432:5432"
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: postgres
    volumes:
      - timescaledb_data:/var/lib/postgresql/data
      - ./migrations/local/init.sql:/docker-entrypoint-initdb.d/init.sql
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      timeout: 5s
      retries: 5

  minio:
    image: minio/minio:latest
    container_name: kalshi-minio
    ports:
      - "9000:9000"
      - "9001:9001"  # Console
    environment:
      MINIO_ROOT_USER: minioadmin
      MINIO_ROOT_PASSWORD: minioadmin
    command: server /data --console-address ":9001"
    volumes:
      - minio_data:/data
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9000/minio/health/live"]
      interval: 5s
      timeout: 5s
      retries: 5

  # Optional: Monitoring stack
  prometheus:
    image: prom/prometheus:latest
    container_name: kalshi-prometheus
    ports:
      - "9090:9090"
    volumes:
      - ./configs/local/prometheus.yml:/etc/prometheus/prometheus.yml
    profiles:
      - monitoring

  grafana:
    image: grafana/grafana:latest
    container_name: kalshi-grafana
    ports:
      - "3000:3000"
    environment:
      GF_SECURITY_ADMIN_PASSWORD: admin
    volumes:
      - grafana_data:/var/lib/grafana
      - ./configs/local/grafana/dashboards:/etc/grafana/provisioning/dashboards
    profiles:
      - monitoring

volumes:
  timescaledb_data:
  minio_data:
  grafana_data:
```

### Database Init Script

**migrations/local/init.sql:**
```sql
-- Create database
CREATE DATABASE kalshi_ts;

-- Connect to TimescaleDB database
\c kalshi_ts

-- Enable TimescaleDB
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- Create tables (simplified for local dev)
CREATE TABLE trades (
    trade_id        UUID PRIMARY KEY,
    exchange_ts     BIGINT NOT NULL,
    received_at     BIGINT NOT NULL,
    ticker          VARCHAR(128) NOT NULL,
    price           INTEGER NOT NULL,
    size            INTEGER NOT NULL,
    taker_side      BOOLEAN NOT NULL
);

SELECT create_hypertable('trades', 'exchange_ts',
    chunk_time_interval => 86400000000);

CREATE INDEX idx_trades_ticker ON trades (ticker, exchange_ts DESC);

CREATE TABLE orderbook_deltas (
    exchange_ts     BIGINT NOT NULL,
    received_at     BIGINT NOT NULL,
    ticker          VARCHAR(128) NOT NULL,
    side            BOOLEAN NOT NULL,
    price           INTEGER NOT NULL,
    size_delta      INTEGER NOT NULL,
    seq             BIGINT,
    PRIMARY KEY (ticker, exchange_ts, price, side)
);

SELECT create_hypertable('orderbook_deltas', 'exchange_ts',
    chunk_time_interval => 3600000000);

CREATE TABLE orderbook_snapshots (
    snapshot_ts     BIGINT NOT NULL,
    exchange_ts     BIGINT,
    ticker          VARCHAR(128) NOT NULL,
    source          VARCHAR(8) NOT NULL,
    yes_bids        JSONB NOT NULL,
    yes_asks        JSONB NOT NULL,
    no_bids         JSONB NOT NULL,
    no_asks         JSONB NOT NULL,
    PRIMARY KEY (ticker, snapshot_ts, source)
);

SELECT create_hypertable('orderbook_snapshots', 'snapshot_ts',
    chunk_time_interval => 86400000000);

CREATE TABLE tickers (
    exchange_ts     BIGINT NOT NULL,
    received_at     BIGINT NOT NULL,
    ticker          VARCHAR(128) NOT NULL,
    yes_bid         INTEGER,
    yes_ask         INTEGER,
    last_price      INTEGER,
    volume          BIGINT,
    open_interest   BIGINT,
    PRIMARY KEY (ticker, exchange_ts)
);

SELECT create_hypertable('tickers', 'exchange_ts',
    chunk_time_interval => 3600000000);
```

**Note:** Market metadata (series, events, markets) is stored in-memory by the Market Registry. In local development, you can inspect the registry via the `/debug/markets` endpoint.

---

## Configuration

### Local Gatherer Config

**configs/local/gatherer.yaml:**
```yaml
# Kalshi API
kalshi:
  rest_url: ${KALSHI_API_URL}
  ws_url: ${KALSHI_WS_URL}
  api_key: ${KALSHI_API_KEY}
  api_secret: ${KALSHI_API_SECRET}

# Local mode: single gatherer, reduced connections
mode: local
gatherer_id: local-1

# Same WebSocket connections as production (need full market coverage)
websocket:
  ticker_connections: 2
  trade_connections: 2
  lifecycle_connections: 2
  orderbook_connections: 144
  reconnect_backoff: 1s
  max_reconnect_backoff: 30s

# Local database
timescaledb:
  url: ${TIMESCALEDB_URL}
  max_connections: 5         # 20 in prod

# Smaller batches for faster feedback
writers:
  trades:
    batch_size: 100          # 1000 in prod
    flush_interval: 1s       # 100ms in prod
  orderbook:
    batch_size: 500          # 5000 in prod
    flush_interval: 500ms    # 50ms in prod
  tickers:
    batch_size: 100          # 1000 in prod
    flush_interval: 1s       # 100ms in prod
  snapshots:
    batch_size: 10           # 100 in prod
    flush_interval: 5s       # 1s in prod

# Snapshot polling (less aggressive for demo)
snapshot_poller:
  enabled: true
  interval: 5m               # 15m in prod
  max_markets: 50            # all in prod

# Logging
logging:
  level: debug               # info in prod
  format: text               # json in prod

# Metrics
metrics:
  enabled: true
  port: 9090
```

### Local Deduplicator Config (Optional)

For testing deduplication locally, you can run a second gatherer instance and a deduplicator:

**configs/local/deduplicator.yaml:**
```yaml
mode: local

# In local mode, gatherer writes directly to "production" DB
# Deduplicator is optional, mainly for testing the dedup logic

gatherers:
  - id: local-1
    timescaledb: ${TIMESCALEDB_URL}

# Same DB acts as "production" in local mode
production:
  url: ${TIMESCALEDB_URL}

# Local S3 (MinIO)
s3_export:
  enabled: true
  endpoint: ${S3_ENDPOINT}
  access_key: ${S3_ACCESS_KEY}
  secret_key: ${S3_SECRET_KEY}
  bucket: ${S3_BUCKET}
  region: us-east-1

sync:
  poll_interval: 1s          # 100ms in prod
  batch_size: 100            # 5000 in prod

logging:
  level: debug
  format: text

metrics:
  enabled: true
  port: 9091
```

---

## Development Workflows

### Running Tests

```bash
# Unit tests
make test

# Integration tests (requires Docker)
make test-integration

# With coverage
make test-coverage
```

### Debugging

```bash
# Run with delve debugger
dlv debug ./cmd/gatherer -- --config configs/local/gatherer.yaml

# Or attach to running process
dlv attach $(pgrep gatherer)
```

### Watching Logs

```bash
# Gatherer logs
go run ./cmd/gatherer --config configs/local/gatherer.yaml 2>&1 | tee gatherer.log

# Filter for specific events
tail -f gatherer.log | grep -E "(trade|orderbook|error)"
```

### Database Inspection

```bash
# Connect to TimescaleDB
psql postgres://postgres:postgres@localhost:5432/kalshi_ts

# Useful queries (all timestamps in µs, prices in hundred-thousandths):

# Recent trades (raw values - convert in application)
SELECT ticker, price, size, exchange_ts
FROM trades ORDER BY exchange_ts DESC LIMIT 20;

# Trade volume by market (last hour)
# Calculate cutoff in Go: time.Now().Add(-1*time.Hour).UnixMicro()
# For ad-hoc queries: SELECT extract(epoch from now()) * 1000000;
SELECT ticker, COUNT(*), SUM(size) as volume
FROM trades
WHERE exchange_ts > (SELECT extract(epoch from now()) * 1000000 - 3600000000)
GROUP BY ticker ORDER BY volume DESC;

# Orderbook depth (last 5 minutes = 300000000 µs)
SELECT ticker, side, COUNT(*) as levels
FROM orderbook_deltas
WHERE exchange_ts > (SELECT extract(epoch from now()) * 1000000 - 300000000)
GROUP BY ticker, side;
```

**Note:** For ad-hoc queries, you can calculate timestamps inline. Production code should always compute timestamps in Go and pass as parameters.

**Market metadata:** The Market Registry stores market state in memory. Use the `/debug/markets` HTTP endpoint to inspect active markets.

### MinIO (Local S3)

```bash
# Access MinIO console
open http://localhost:9001
# Login: minioadmin / minioadmin

# Create bucket via CLI
mc alias set local http://localhost:9000 minioadmin minioadmin
mc mb local/kalshi-data-local

# List exported files
mc ls local/kalshi-data-local/raw/
```

---

## Architecture Comparison

```mermaid
flowchart TB
    subgraph Kalshi["Kalshi API"]
        K[api.elections.kalshi.com]
    end

    subgraph Production["Production (3 AZs)"]
        direction TB
        P_G1[Gatherer 1]
        P_G2[Gatherer 2]
        P_G3[Gatherer 3]
        P_D[Deduplicator]
        P_RDS[(RDS)]
        P_S3[(S3)]

        P_G1 & P_G2 & P_G3 --> P_D
        P_D --> P_RDS & P_S3
    end

    subgraph Local["Local Dev (Single Machine)"]
        direction TB
        L_G[Gatherer]
        L_TS[(TimescaleDB<br/>Docker)]
        L_MINIO[(MinIO<br/>Docker)]

        L_G --> L_TS
        L_G -.->|optional| L_MINIO
    end

    K --> P_G1 & P_G2 & P_G3
    K --> L_G
```

| Feature | Production | Local |
|---------|------------|-------|
| Redundancy | 3x gatherers | 1 gatherer |
| Deduplication | Required | Not needed (single source) |
| WebSocket connections | 450 (3 × 150) | 150 |
| Markets tracked | All active (200K-600K) | All active |
| Data volume | High | Same (single gatherer) |
| Storage | Multi-TB | ~10-50GB/day |

---

## Makefile

```makefile
.PHONY: dev up down migrate run-gatherer run-dedup test clean

# Start development environment
dev: up migrate run-gatherer

# Docker commands
up:
	docker-compose up -d

down:
	docker-compose down

up-monitoring:
	docker-compose --profile monitoring up -d

# Database
migrate:
	psql postgres://postgres:postgres@localhost:5432/postgres \
		-f migrations/local/init.sql

reset-db:
	docker-compose down -v
	docker-compose up -d
	sleep 5
	$(MAKE) migrate

# Run applications
run-gatherer:
	go run ./cmd/gatherer --config configs/local/gatherer.yaml

run-dedup:
	go run ./cmd/deduplicator --config configs/local/deduplicator.yaml

# Testing
test:
	go test ./...

test-integration:
	docker-compose up -d
	go test -tags=integration ./...
	docker-compose down

test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# Build
build:
	go build -o bin/gatherer ./cmd/gatherer
	go build -o bin/deduplicator ./cmd/deduplicator

# Clean
clean:
	rm -rf bin/
	rm -f coverage.out
	docker-compose down -v
```

---

## Troubleshooting

### Docker Issues

```bash
# Container won't start
docker-compose logs timescaledb

# Reset everything
docker-compose down -v
docker-compose up -d

# Check disk space
docker system df
docker system prune -a  # WARNING: removes all unused data
```

### Connection Issues

```bash
# Can't connect to database
pg_isready -h localhost -p 5432

# Check if port is in use
lsof -i :5432

# Container networking
docker-compose exec timescaledb pg_isready
```

### Kalshi API Issues

```bash
# Test API connectivity
curl -I https://api.elections.kalshi.com/trade-api/v2/exchange/status

# Check credentials
curl -X POST https://api.elections.kalshi.com/trade-api/v2/login \
  -H "Content-Type: application/json" \
  -d '{"email":"your@email.com","password":"yourpassword"}'

# Common issues:
# - Rate limits on order operations (not market data)
# - WebSocket connection limits per API key
# - Check API key permissions in Kalshi dashboard
```

### Performance Issues

```bash
# Gatherer using too much CPU
# → Reduce batch sizes, increase flush intervals

# Database slow
# → Check for missing indexes
psql $TIMESCALEDB_URL -c "\di"

# Too many connections
psql $TIMESCALEDB_URL -c "SELECT count(*) FROM pg_stat_activity;"
```

---

## Next Steps

After local development, see:
- [Deployment Runbook](../deployment/runbook.md) - Deploy to AWS
- [Configuration](../deployment/configuration.md) - Production config differences
- [Testing](./testing.md) - Test strategies and fixtures
