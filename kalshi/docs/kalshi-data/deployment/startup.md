# Startup

Initialization order, dependencies, and systemd configuration.

---

## Startup Order

```mermaid
flowchart TD
    subgraph "Phase 1: Infrastructure"
        RDS[Production RDS]
        S3[S3 Bucket]
    end

    subgraph "Phase 2: Gatherers (parallel)"
        G1[Gatherer 1]
        G2[Gatherer 2]
        G3[Gatherer 3]
    end

    subgraph "Phase 3: Deduplicator"
        DEDUP[Deduplicator]
    end

    RDS --> DEDUP
    G1 & G2 & G3 --> DEDUP
```

| Phase | Component | Dependencies | Can Start In Parallel |
|-------|-----------|--------------|----------------------|
| 1 | RDS | None | - |
| 1 | S3 | None | Yes (with RDS) |
| 2 | Gatherer 1 | None | Yes |
| 2 | Gatherer 2 | None | Yes |
| 2 | Gatherer 3 | None | Yes |
| 3 | Deduplicator | RDS, at least 1 Gatherer | No |

---

## Gatherer Startup

### Initialization Sequence

```mermaid
sequenceDiagram
    participant G as Gatherer
    participant TS as Local TimescaleDB
    participant K as Kalshi API

    G->>TS: Connect to TimescaleDB
    G->>K: GET /exchange/status
    K-->>G: {exchange_active: true}
    G->>K: GET /markets (paginated)
    K-->>G: [all markets]
    G->>G: Store markets in memory (Market Registry)
    G->>K: Connect WebSocket
    G->>K: Subscribe ticker, trade, lifecycle
    G->>K: Subscribe orderbook_delta (all markets)
    G->>G: Start Snapshot Poller
    G->>G: Healthy
```

### Startup Script

```bash
#!/bin/bash
# scripts/gatherer-init.sh

set -e

# Install dependencies
dnf install -y postgresql16-server timescaledb

# Initialize database
postgresql-setup --initdb
systemctl start postgresql

# Create TimescaleDB database
sudo -u postgres psql -c "CREATE DATABASE kalshi_ts;"
sudo -u postgres psql -d kalshi_ts -c "CREATE EXTENSION IF NOT EXISTS timescaledb;"

# Run migrations
/opt/kalshi/gatherer migrate --config /etc/kalshi/gatherer.yaml

# Start gatherer service
systemctl enable gatherer
systemctl start gatherer
```

### Health Check

Gatherer exposes `/health` endpoint:

```bash
curl http://localhost:8080/health
```

```json
{
  "status": "healthy",
  "components": {
    "timescaledb": "connected",
    "websocket": {
      "connections": 150,
      "healthy": 148
    },
    "market_registry": {
      "markets": 425000
    }
  }
}
```

### Health Status Definitions

| Status | Criteria | HTTP Code |
|--------|----------|-----------|
| `healthy` | All checks pass | 200 |
| `degraded` | Partial functionality | 200 |
| `unhealthy` | Critical failure | 503 |

### Health Check Thresholds

| Component | Healthy | Degraded | Unhealthy |
|-----------|---------|----------|-----------|
| **TimescaleDB** | Connected, ping < 100ms | Ping 100-500ms | Disconnected |
| **WebSocket** | ≥140 of 150 connections | 100-139 connections | <100 connections |
| **Market Registry** | >0 markets loaded | - | 0 markets |

**Threshold Rationale:**
- **WebSocket 140/150:** Target is 150 connections (6 global + 144 orderbook). 140+ = near full capacity. Below 100 = significant orderbook coverage gaps.
- **Database pings:** Local databases should respond in <100ms. Higher latency indicates disk or load issues.

```go
func (g *Gatherer) determineHealth() Health {
    h := Health{Status: "healthy", Components: make(map[string]interface{})}

    // Database check
    if !g.timescaleDB.IsConnected() {
        h.Status = "unhealthy"
    }

    // WebSocket check (target: 150 connections)
    wsHealthy := g.connManager.HealthyConnections()
    if wsHealthy < 100 {
        h.Status = "unhealthy"  // Major coverage gap
    } else if wsHealthy < 140 {
        if h.Status == "healthy" {
            h.Status = "degraded"  // Some connections down
        }
    }

    // Market Registry check (in-memory)
    if g.registry.MarketCount() == 0 {
        h.Status = "unhealthy"
    }

    return h
}
```

**Load balancer configuration:**
- Health check path: `/health`
- Interval: 10 seconds
- Healthy threshold: 2 consecutive passes
- Unhealthy threshold: 3 consecutive failures
- Only route traffic to `healthy` instances

---

## Deduplicator Startup

### Initialization Sequence

```mermaid
sequenceDiagram
    participant D as Deduplicator
    participant K as Kalshi API
    participant G1 as Gatherer 1
    participant G2 as Gatherer 2
    participant G3 as Gatherer 3
    participant P as Production RDS

    D->>P: Connect to Production RDS
    D->>P: Load sync cursors
    D->>G1: Connect to TimescaleDB
    D->>G2: Connect to TimescaleDB
    D->>G3: Connect to TimescaleDB
    D->>K: Test API connection
    D->>D: Wait for at least 1 gatherer healthy
    D->>D: Start sync loops (time-series from gatherers)
    D->>D: Start market sync (from Kalshi API)
    D->>D: Healthy
```

### Startup Script

```bash
#!/bin/bash
# scripts/deduplicator-init.sh

set -e

# Wait for RDS to be available
until pg_isready -h $RDS_HOST -p 5432; do
  echo "Waiting for RDS..."
  sleep 5
done

# Wait for at least one gatherer
GATHERER_READY=false
for i in {1..60}; do
  for g in $GATHERER_IPS; do
    if curl -s "http://$g:8080/health" | grep -q '"status":"healthy"'; then
      GATHERER_READY=true
      break 2
    fi
  done
  echo "Waiting for gatherers... ($i/60)"
  sleep 5
done

if [ "$GATHERER_READY" = false ]; then
  echo "ERROR: No gatherers available after 5 minutes"
  exit 1
fi

# Start deduplicator service
systemctl enable deduplicator
systemctl start deduplicator
```

### Health Check

```bash
curl http://localhost:8080/health
```

```json
{
  "status": "healthy",
  "components": {
    "production_rds": "connected",
    "gatherers": {
      "gatherer-1": "connected",
      "gatherer-2": "connected",
      "gatherer-3": "connected"
    },
    "sync": {
      "max_lag_seconds": 0.5,
      "tables_syncing": 6
    }
  }
}
```

### Health Status Definitions

| Status | Criteria | HTTP Code |
|--------|----------|-----------|
| `healthy` | All checks pass | 200 |
| `degraded` | Partial functionality | 200 |
| `unhealthy` | Critical failure | 503 |

### Health Check Thresholds

| Component | Healthy | Degraded | Unhealthy |
|-----------|---------|----------|-----------|
| **Production RDS** | Connected, ping < 50ms | Ping 50-200ms | Disconnected |
| **Gatherers** | 3 connected | 1-2 connected | 0 connected |
| **Sync Lag** | < 5 seconds | 5-30 seconds | > 30 seconds |

```go
func (d *Deduplicator) determineHealth() Health {
    h := Health{Status: "healthy", Components: make(map[string]interface{})}

    // Production RDS check
    if !d.production.IsConnected() {
        h.Status = "unhealthy"
    }

    // Gatherer count check
    connectedGatherers := d.countConnectedGatherers()
    if connectedGatherers == 0 {
        h.Status = "unhealthy"
    } else if connectedGatherers < 3 {
        if h.Status == "healthy" {
            h.Status = "degraded"
        }
    }

    // Sync lag check
    maxLag := d.getMaxSyncLag()
    if maxLag > 30*time.Second {
        h.Status = "unhealthy"
    } else if maxLag > 5*time.Second {
        if h.Status == "healthy" {
            h.Status = "degraded"
        }
    }

    return h
}
```

### Sync Lag Alert Thresholds

| Lag | Severity | Action |
|-----|----------|--------|
| < 5s | Normal | None |
| 5-30s | Warning | Investigate gatherer/network issues |
| 30s-5min | Critical | Check database performance |
| > 5min | Emergency | Manual intervention required |

---

## Systemd Units

### Gatherer Service

```ini
# /etc/systemd/system/gatherer.service
[Unit]
Description=Kalshi Data Gatherer
After=network.target postgresql.service
Requires=postgresql.service

[Service]
Type=simple
User=kalshi
Group=kalshi
ExecStart=/opt/kalshi/gatherer --config /etc/kalshi/gatherer.yaml
Restart=always
RestartSec=10
StartLimitInterval=60
StartLimitBurst=3

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=gatherer

# Security
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/kalshi

# Resource limits
LimitNOFILE=65536
MemoryMax=28G

[Install]
WantedBy=multi-user.target
```

### Deduplicator Service

```ini
# /etc/systemd/system/deduplicator.service
[Unit]
Description=Kalshi Data Deduplicator
After=network.target

[Service]
Type=simple
User=kalshi
Group=kalshi
ExecStart=/opt/kalshi/deduplicator --config /etc/kalshi/deduplicator.yaml
Restart=always
RestartSec=10
StartLimitInterval=60
StartLimitBurst=3

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=deduplicator

# Security
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/kalshi

# Resource limits
LimitNOFILE=65536
MemoryMax=14G

[Install]
WantedBy=multi-user.target
```

---

## Database Users and Roles

### User Inventory

| User | Database | Host | Permissions | Used By |
|------|----------|------|-------------|---------|
| `gatherer` | kalshi_ts (TimescaleDB) | Gatherer | Read/Write | Gatherer |
| `dedup_reader` | kalshi_ts (TimescaleDB) | Gatherer | Read-only | Deduplicator |
| `deduplicator` | kalshi_prod (RDS) | RDS | Read/Write | Deduplicator |
| `exporter` | kalshi_prod (RDS) | RDS | Read-only | S3 Exporter |

Note: Market metadata (series, events, markets) is synced directly from Kalshi API to Production RDS by the Deduplicator.

### Gatherer Database Users

```sql
-- Run as postgres superuser on each gatherer

-- Create gatherer user (read/write for local operations)
CREATE USER gatherer WITH PASSWORD 'secure_password_here';

-- TimescaleDB permissions
GRANT CONNECT ON DATABASE kalshi_ts TO gatherer;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO gatherer;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO gatherer;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO gatherer;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO gatherer;

-- Create dedup_reader user (read-only for deduplicator)
CREATE USER dedup_reader WITH PASSWORD 'secure_password_here';

-- TimescaleDB read-only permissions
GRANT CONNECT ON DATABASE kalshi_ts TO dedup_reader;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO dedup_reader;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO dedup_reader;
```

### Production RDS Users

```sql
-- Run as RDS master user

-- Create deduplicator user (read/write)
CREATE USER deduplicator WITH PASSWORD 'secure_password_here';
GRANT CONNECT ON DATABASE kalshi_prod TO deduplicator;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO deduplicator;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO deduplicator;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO deduplicator;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO deduplicator;

-- Create exporter user (read-only for S3 export)
CREATE USER exporter WITH PASSWORD 'secure_password_here';
GRANT CONNECT ON DATABASE kalshi_prod TO exporter;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO exporter;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO exporter;
```

### pg_hba.conf Configuration (Gatherer)

```
# /etc/postgresql/16/main/pg_hba.conf

# Local connections (gatherer binary)
local   kalshi_ts       gatherer                                scram-sha-256

# Remote connections (deduplicator)
hostssl kalshi_ts       dedup_reader    10.0.0.0/8              scram-sha-256

# Reject all other connections
host    all             all             0.0.0.0/0               reject
```

### SSL Certificate Configuration

For RDS connections:

```bash
# Download RDS CA bundle
wget https://truststore.pki.rds.amazonaws.com/global/global-bundle.pem \
  -O /etc/kalshi/rds-ca-bundle.pem

# Verify certificate
openssl x509 -in /etc/kalshi/rds-ca-bundle.pem -text -noout | head -20
```

Connection string with SSL:

```
postgres://deduplicator:xxx@kalshi-prod.xxx.rds.amazonaws.com:5432/kalshi_prod?sslmode=verify-full&sslrootcert=/etc/kalshi/rds-ca-bundle.pem
```

### Password Rotation

Passwords should be rotated regularly (every 90 days recommended).

**Important:** PostgreSQL doesn't support multiple passwords per user. Rotation requires a brief window where apps use the new password.

#### Rotation Procedure

```bash
#!/bin/bash
# scripts/rotate-password.sh

set -e

SECRET_NAME="$1"  # e.g., kalshi-data/prod/rds-password
DB_USER="$2"      # e.g., deduplicator
DB_HOST="$3"      # e.g., kalshi-prod.xxx.rds.amazonaws.com

# Step 1: Generate new password
NEW_PASSWORD=$(openssl rand -base64 32)

# Step 2: Update database user password
PGPASSWORD="$OLD_PASSWORD" psql -h "$DB_HOST" -U admin -c \
  "ALTER USER $DB_USER WITH PASSWORD '$NEW_PASSWORD';"

# Step 3: Update secret in Secrets Manager
aws secretsmanager update-secret \
  --secret-id "$SECRET_NAME" \
  --secret-string "$NEW_PASSWORD"

# Step 4: Restart application to pick up new password
# For gatherer:
ssh gatherer-1 "sudo systemctl restart gatherer"
ssh gatherer-2 "sudo systemctl restart gatherer"
ssh gatherer-3 "sudo systemctl restart gatherer"

# For deduplicator:
ssh deduplicator "sudo systemctl restart deduplicator"

echo "Password rotation complete for $DB_USER"
```

#### Rotation Steps (Manual)

| Step | Action | Downtime |
|------|--------|----------|
| 1 | Generate new password | None |
| 2 | Update DB user password | None |
| 3 | Update Secrets Manager | None |
| 4 | Restart applications (rolling) | ~10s per instance |

**Rollback:** If rotation fails, restore old password from Secrets Manager version history:

```bash
# List versions
aws secretsmanager list-secret-version-ids --secret-id "$SECRET_NAME"

# Restore previous version
aws secretsmanager update-secret \
  --secret-id "$SECRET_NAME" \
  --secret-string "$(aws secretsmanager get-secret-value \
    --secret-id "$SECRET_NAME" \
    --version-id "previous-version-id" \
    --query SecretString --output text)"
```

---

## Database Initialization

### TimescaleDB (Gatherer)

```sql
-- migrations/001_timescaledb.sql

-- Enable TimescaleDB
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- Trades table
CREATE TABLE trades (
    trade_id        UUID PRIMARY KEY,
    exchange_ts     BIGINT NOT NULL,
    received_at     BIGINT NOT NULL,
    ticker          VARCHAR(128) NOT NULL,
    price           INTEGER NOT NULL,
    size            INTEGER NOT NULL,
    taker_side      BOOLEAN NOT NULL,
    sid             BIGINT
);

SELECT create_hypertable('trades', 'received_at',
    chunk_time_interval => 86400000000);  -- 1 day in microseconds

CREATE INDEX idx_trades_ticker ON trades (ticker, received_at DESC);

-- Orderbook deltas table
-- Note: PK is (ticker, exchange_ts, price, side) for deduplication
-- TimescaleDB requires time column in PK, so we use a composite approach
CREATE TABLE orderbook_deltas (
    exchange_ts     BIGINT NOT NULL,
    received_at     BIGINT NOT NULL,
    ticker          VARCHAR(128) NOT NULL,
    side            BOOLEAN NOT NULL,
    price           INTEGER NOT NULL,
    size_delta      INTEGER NOT NULL,
    seq             BIGINT,
    sid             BIGINT,
    PRIMARY KEY (ticker, exchange_ts, price, side)
);

SELECT create_hypertable('orderbook_deltas', 'exchange_ts',
    chunk_time_interval => 3600000000);  -- 1 hour in microseconds

CREATE INDEX idx_deltas_ticker_time ON orderbook_deltas (ticker, exchange_ts DESC);

-- Orderbook snapshots table
CREATE TABLE orderbook_snapshots (
    snapshot_ts     BIGINT NOT NULL,
    exchange_ts     BIGINT,
    ticker          VARCHAR(128) NOT NULL,
    source          VARCHAR(8) NOT NULL,  -- 'ws' or 'rest'
    yes_bids        JSONB NOT NULL,
    yes_asks        JSONB NOT NULL,
    no_bids         JSONB NOT NULL,
    no_asks         JSONB NOT NULL,
    best_yes_bid    INTEGER,
    best_yes_ask    INTEGER,
    spread          INTEGER,
    sid             BIGINT,
    PRIMARY KEY (ticker, snapshot_ts, source)
);

SELECT create_hypertable('orderbook_snapshots', 'snapshot_ts',
    chunk_time_interval => 3600000000);  -- 1 hour in microseconds

CREATE INDEX idx_snapshots_ticker ON orderbook_snapshots (ticker, snapshot_ts DESC);

-- Tickers table
CREATE TABLE tickers (
    exchange_ts     BIGINT NOT NULL,
    received_at     BIGINT NOT NULL,
    ticker          VARCHAR(128) NOT NULL,
    yes_bid         INTEGER,
    yes_ask         INTEGER,
    last_price      INTEGER,
    volume          BIGINT,
    open_interest   BIGINT,
    dollar_volume   BIGINT,
    dollar_open_interest BIGINT,
    sid             BIGINT,
    PRIMARY KEY (ticker, exchange_ts)
);

SELECT create_hypertable('tickers', 'exchange_ts',
    chunk_time_interval => 3600000000);  -- 1 hour in microseconds

CREATE INDEX idx_tickers_ticker_time ON tickers (ticker, exchange_ts DESC);
```

---

## Graceful Shutdown

### Gatherer

```go
func (g *Gatherer) Shutdown(ctx context.Context) error {
    g.logger.Info("shutting down gatherer")

    // 1. Stop accepting new WebSocket messages
    g.connManager.StopReceiving()

    // 2. Flush all writers
    g.router.FlushAll(ctx)

    // 3. Close WebSocket connections
    g.connManager.Close()

    // 4. Close database connection
    g.timescaleDB.Close()

    g.logger.Info("gatherer shutdown complete")
    return nil
}
```

### Deduplicator

```go
func (d *Deduplicator) Shutdown(ctx context.Context) error {
    d.logger.Info("shutting down deduplicator")

    // 1. Stop sync loops
    d.cancelSync()

    // 2. Wait for in-flight writes to complete
    d.wg.Wait()

    // 3. Close gatherer connections
    for _, g := range d.gatherers {
        g.Close()
    }

    // 4. Close production connection
    d.production.Close()

    d.logger.Info("deduplicator shutdown complete")
    return nil
}
```

### Systemd Stop

```bash
# Graceful stop (SIGTERM)
systemctl stop gatherer

# Force stop after 30s timeout
# (Configured via TimeoutStopSec in unit file)
```

---

## Deployment Checklist

### Pre-Deployment

- [ ] RDS instance available and accessible
- [ ] S3 bucket created with lifecycle policies
- [ ] Security groups configured
- [ ] IAM roles created
- [ ] Secrets stored in Parameter Store/Secrets Manager
- [ ] AMI built with dependencies

### Gatherer Deployment

- [ ] EC2 instance launched in correct AZ
- [ ] EBS volume attached
- [ ] Local TimescaleDB initialized
- [ ] Database migrations applied
- [ ] Configuration file deployed
- [ ] Systemd service enabled and started
- [ ] Health check passing

### Deduplicator Deployment

- [ ] EC2 instance launched
- [ ] At least one gatherer healthy
- [ ] Production RDS accessible
- [ ] Configuration file deployed
- [ ] Systemd service enabled and started
- [ ] Health check passing
- [ ] Sync lag within acceptable range

### Post-Deployment

- [ ] All gatherers healthy
- [ ] Deduplicator syncing all gatherers
- [ ] Prometheus scraping metrics
- [ ] Alerts configured and firing correctly
- [ ] Grafana dashboards showing data
