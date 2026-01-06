# Configuration

Configuration schemas for gatherer and deduplicator binaries.

---

## Overview

Configuration is loaded from YAML files with environment variable substitution:

```bash
# Run with config file
./gatherer --config /etc/kalshi/gatherer.yaml

# Environment variables are substituted
# In YAML: api_key: "${KALSHI_API_KEY}"
# Resolved from: export KALSHI_API_KEY="..."
```

---

## Gatherer Configuration

### Full Schema

```yaml
# /etc/kalshi/gatherer.yaml

# Gatherer identity (used for metrics and logging)
gatherer_id: "gatherer-1"  # Required

# HTTP server for health checks and metrics
server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: 10s
  write_timeout: 10s

# Kalshi API configuration
api:
  base_url: "https://api.elections.kalshi.com/trade-api/v2"
  ws_url: "wss://api.elections.kalshi.com/trade-api/ws/v2"
  api_key: "${KALSHI_API_KEY}"           # Required
  api_secret: "${KALSHI_API_SECRET}"     # Required if using signed requests
  timeout: 30s
  max_retries: 3
  retry_backoff: 1s

# Local TimescaleDB (time-series data only)
# Note: Gatherers only store time-series data. Market metadata lives in-memory (Market Registry).
timescaledb:
  host: "localhost"
  port: 5432
  database: "kalshi_ts"
  user: "gatherer"
  password: "${TIMESCALEDB_PASSWORD}"    # Required
  ssl_mode: "prefer"                     # disable, prefer, require, verify-full
  pool:
    min_conns: 2
    max_conns: 10
    max_conn_lifetime: 1h
    max_conn_idle_time: 30m
    health_check_period: 1m

# Market Registry
market_registry:
  reconcile_interval: 5m                 # REST reconciliation frequency
  page_size: 1000                        # Markets per REST page
  initial_load_timeout: 5m               # Timeout for initial market load

# Connection Manager
connection_manager:
  max_connections: 150                   # WebSocket connection pool size
  markets_per_connection: 50             # Orderbook subscriptions per connection
  connect_timeout: 10s
  read_timeout: 30s                      # Consider connection dead after
  initial_backoff: 1s
  max_backoff: 5m
  ping_interval: 15s

# Message Router
router:
  orderbook_buffer_size: 5000
  trade_buffer_size: 1000
  ticker_buffer_size: 1000

# Writers
writers:
  batch_size: 1000                       # Records per batch
  flush_interval: 1s                     # Max time before flush
  orderbook:
    batch_size: 2000                     # Override for orderbook
  trade:
    batch_size: 500
  ticker:
    batch_size: 1000

# Snapshot Poller
snapshot_poller:
  enabled: true
  poll_interval: 15m
  concurrency: 100                       # Max concurrent REST requests
  request_timeout: 10s

# Logging
logging:
  level: "info"                          # debug, info, warn, error
  format: "json"                         # json, text
  output: "stdout"                       # stdout, stderr, file
  file_path: "/var/log/kalshi/gatherer.log"

# Metrics
metrics:
  enabled: true
  path: "/metrics"
  namespace: "kalshi"
  subsystem: "gatherer"
```

### Required Fields

| Field | Environment Variable | Description |
|-------|---------------------|-------------|
| `gatherer_id` | - | Unique identifier for this gatherer |
| `api.api_key` | `KALSHI_API_KEY` | Kalshi API key |
| `timescaledb.password` | `TIMESCALEDB_PASSWORD` | TimescaleDB password |

### Default Values

| Field | Default | Notes |
|-------|---------|-------|
| `server.port` | 8080 | Health check and metrics port |
| `connection_manager.max_connections` | 150 | WebSocket pool size |
| `router.orderbook_buffer_size` | 5000 | Channel buffer |
| `writers.batch_size` | 1000 | Records per insert batch |
| `snapshot_poller.poll_interval` | 15m | REST polling frequency |
| `logging.level` | info | Log verbosity |

---

## Deduplicator Configuration

### Full Schema

```yaml
# /etc/kalshi/deduplicator.yaml

# Deduplicator identity
deduplicator_id: "deduplicator-1"

# HTTP server
server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: 10s
  write_timeout: 10s

# Gatherer connections (read time-series data from each)
# Note: Gatherers only have TimescaleDB for time-series data. No PostgreSQL.
gatherers:
  - id: "gatherer-1"
    host: "10.0.1.10"
    timescaledb:
      port: 5432
      database: "kalshi_ts"
      user: "dedup_reader"
      password: "${GATHERER1_TS_PASSWORD}"
      ssl_mode: "require"
  - id: "gatherer-2"
    host: "10.0.2.10"
    timescaledb:
      port: 5432
      database: "kalshi_ts"
      user: "dedup_reader"
      password: "${GATHERER2_TS_PASSWORD}"
      ssl_mode: "require"
  - id: "gatherer-3"
    host: "10.0.3.10"
    timescaledb:
      port: 5432
      database: "kalshi_ts"
      user: "dedup_reader"
      password: "${GATHERER3_TS_PASSWORD}"
      ssl_mode: "require"

# Production RDS (write target)
production:
  host: "kalshi-prod.xxx.rds.amazonaws.com"
  port: 5432
  database: "kalshi_prod"
  user: "deduplicator"
  password: "${PRODUCTION_PASSWORD}"
  ssl_mode: "require"
  ssl_root_cert: "/etc/kalshi/rds-ca-bundle.pem"
  pool:
    min_conns: 4
    max_conns: 20
    max_conn_lifetime: 1h

# Sync configuration (time-series tables only)
# Note: Market/event metadata is managed separately via REST API sync to production RDS.
sync:
  # Per-table configuration
  tables:
    trades:
      poll_interval: 100ms
      batch_size: 5000
      parallel: true
    orderbook_deltas:
      poll_interval: 100ms
      batch_size: 5000
      parallel: true
    orderbook_snapshots:
      poll_interval: 1s
      batch_size: 1000
      parallel: true
    tickers:
      poll_interval: 100ms
      batch_size: 5000
      parallel: true

# Kalshi API (for syncing market/event metadata to production RDS)
api:
  base_url: "https://api.elections.kalshi.com/trade-api/v2"
  api_key: "${KALSHI_API_KEY}"
  sync_interval: 5m                        # How often to refresh markets/events

# S3 export configuration
s3:
  enabled: true
  bucket: "kalshi-data-archive"
  region: "us-east-1"
  # Use IAM role (recommended) or explicit credentials
  # access_key_id: "${AWS_ACCESS_KEY_ID}"
  # secret_access_key: "${AWS_SECRET_ACCESS_KEY}"
  export:
    trades:
      frequency: 1h
      lag: 1h
      partition_by: ["year", "month", "day"]
    orderbook_deltas:
      frequency: 1h
      lag: 1h
      partition_by: ["year", "month", "day", "hour"]
    orderbook_snapshots:
      frequency: 1d
      lag: 1d
      partition_by: ["year", "month", "day"]
    tickers:
      frequency: 1h
      lag: 1h
      partition_by: ["year", "month", "day", "hour"]

# Logging
logging:
  level: "info"
  format: "json"
  output: "stdout"

# Metrics
metrics:
  enabled: true
  path: "/metrics"
  namespace: "kalshi"
  subsystem: "deduplicator"
```

### Required Fields

| Field | Environment Variable | Description |
|-------|---------------------|-------------|
| `production.password` | `PRODUCTION_PASSWORD` | Production RDS password |
| `gatherers[*].timescaledb.password` | `GATHERER{N}_TS_PASSWORD` | Gatherer TimescaleDB password |
| `api.api_key` | `KALSHI_API_KEY` | Kalshi API key (for market/event sync) |

### Gatherer Connection Requirements

At least 1 gatherer must be configured and accessible:

```yaml
gatherers:
  - id: "gatherer-1"
    host: "10.0.1.10"
    # ... (required: at least one gatherer)
```

---

## Environment Variable Substitution

### Syntax

```yaml
# Direct substitution
password: "${DB_PASSWORD}"

# With default value
password: "${DB_PASSWORD:-default_value}"

# Required (fails if not set)
password: "${DB_PASSWORD:?Database password required}"
```

### Loading

```go
func LoadConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    // Expand environment variables
    expanded := os.ExpandEnv(string(data))

    var cfg Config
    if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
        return nil, err
    }

    return &cfg, cfg.Validate()
}
```

---

## Secrets Management

### AWS Systems Manager Parameter Store

Recommended for production:

```bash
# Store secrets
aws ssm put-parameter \
  --name "/kalshi/prod/gatherer-1/timescaledb-password" \
  --value "secret123" \
  --type SecureString \
  --key-id alias/kalshi-secrets

# Retrieve at startup
export TIMESCALEDB_PASSWORD=$(aws ssm get-parameter \
  --name "/kalshi/prod/gatherer-1/timescaledb-password" \
  --with-decryption \
  --query 'Parameter.Value' \
  --output text)
```

### AWS Secrets Manager

For more complex secret rotation:

```bash
# Store secret
aws secretsmanager create-secret \
  --name "kalshi/prod/database-credentials" \
  --secret-string '{"username":"gatherer","password":"secret123"}'

# Retrieve in code
secret, _ := sm.GetSecretValue(&secretsmanager.GetSecretValueInput{
    SecretId: aws.String("kalshi/prod/database-credentials"),
})
```

### Secret Inventory

| Secret | Parameter Store Path | Used By |
|--------|---------------------|---------|
| Kalshi API Key | `/kalshi/prod/api-key` | Gatherer, Deduplicator |
| Kalshi API Secret | `/kalshi/prod/api-secret` | Gatherer |
| TimescaleDB Password | `/kalshi/prod/{gatherer-id}/ts-password` | Gatherer |
| Production RDS Password | `/kalshi/prod/rds-password` | Deduplicator |
| Gatherer Reader Password | `/kalshi/prod/gatherer-reader-password` | Deduplicator |

### Startup Script with Secrets

```bash
#!/bin/bash
# /opt/kalshi/start-gatherer.sh

set -e

# Load secrets from Parameter Store
export KALSHI_API_KEY=$(aws ssm get-parameter \
  --name "/kalshi/prod/api-key" \
  --with-decryption --query 'Parameter.Value' --output text)

export TIMESCALEDB_PASSWORD=$(aws ssm get-parameter \
  --name "/kalshi/prod/${GATHERER_ID}/ts-password" \
  --with-decryption --query 'Parameter.Value' --output text)

# Start gatherer
exec /opt/kalshi/gatherer --config /etc/kalshi/gatherer.yaml
```

---

## Validation

### Gatherer Validation

```go
func (c *GathererConfig) Validate() error {
    if c.GathererID == "" {
        return errors.New("gatherer_id is required")
    }
    if c.API.APIKey == "" {
        return errors.New("api.api_key is required")
    }
    if c.TimescaleDB.Password == "" {
        return errors.New("timescaledb.password is required")
    }
    if c.ConnectionManager.MaxConnections < 1 {
        return errors.New("connection_manager.max_connections must be >= 1")
    }
    return nil
}
```

### Deduplicator Validation

```go
func (c *DeduplicatorConfig) Validate() error {
    if len(c.Gatherers) == 0 {
        return errors.New("at least one gatherer is required")
    }
    if c.Production.Host == "" {
        return errors.New("production.host is required")
    }
    if c.Production.Password == "" {
        return errors.New("production.password is required")
    }
    return nil
}
```

---

## Example Configurations

### Development (Local)

```yaml
# configs/gatherer-dev.yaml
gatherer_id: "dev-gatherer"

api:
  base_url: "https://demo-api.kalshi.co/trade-api/v2"
  ws_url: "wss://demo-api.kalshi.co/trade-api/ws/v2"
  api_key: "${KALSHI_API_KEY}"
  private_key_path: "${KALSHI_PRIVATE_KEY_PATH}"

timescaledb:
  host: "localhost"
  port: 5432
  database: "kalshi_ts_dev"
  user: "kalshi"
  password: "devpassword"
  ssl_mode: "disable"

connection_manager:
  max_connections: 10  # Reduced for dev

logging:
  level: "debug"
  format: "text"
```

### Production

```yaml
# /etc/kalshi/gatherer.yaml
gatherer_id: "gatherer-1"

api:
  base_url: "https://api.elections.kalshi.com/trade-api/v2"
  ws_url: "wss://api.elections.kalshi.com/trade-api/ws/v2"
  api_key: "${KALSHI_API_KEY}"

timescaledb:
  host: "localhost"
  port: 5432
  database: "kalshi_ts"
  user: "gatherer"
  password: "${TIMESCALEDB_PASSWORD}"
  ssl_mode: "require"
  pool:
    min_conns: 5
    max_conns: 20

connection_manager:
  max_connections: 150

logging:
  level: "info"
  format: "json"
```

---

## File Locations

| Environment | Config Path | Logs |
|-------------|-------------|------|
| Development | `./configs/*.yaml` | stdout |
| Production | `/etc/kalshi/*.yaml` | `/var/log/kalshi/*.log` |

### Directory Structure

```
/etc/kalshi/
├── gatherer.yaml
├── deduplicator.yaml
└── rds-ca-bundle.pem       # RDS SSL certificate

/var/log/kalshi/
├── gatherer.log
└── deduplicator.log

/var/lib/kalshi/
└── (runtime data if needed)
```
