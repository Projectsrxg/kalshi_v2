# Configuration

Database connection and storage tuning options.

---

## Connection Configuration

### Gatherer Database

```go
type DBConfig struct {
    // Connection
    Host     string        // Database hostname
    Port     int           // Default: 5432
    Database string        // Database name
    User     string        // Database user
    Password string        // Database password (from secrets)
    SSLMode  string        // "disable", "require", "verify-full"

    // Pool
    MaxConns          int           // Maximum connections in pool
    MinConns          int           // Minimum idle connections
    MaxConnLifetime   time.Duration // Max lifetime of a connection
    MaxConnIdleTime   time.Duration // Max idle time before closing
    HealthCheckPeriod time.Duration // Interval for health checks

    // Timeouts
    ConnectTimeout time.Duration // Connection establishment timeout
    QueryTimeout   time.Duration // Default query timeout
}
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_HOST` | `localhost` | Database hostname |
| `DB_PORT` | `5432` | Database port |
| `DB_NAME` | `kalshi_data` | Database name |
| `DB_USER` | `gatherer` | Database user |
| `DB_PASSWORD` | - | Database password (required) |
| `DB_SSL_MODE` | `require` | SSL mode |
| `DB_MAX_CONNS` | `15` | Maximum pool connections |
| `DB_MIN_CONNS` | `5` | Minimum idle connections |
| `DB_CONNECT_TIMEOUT` | `10s` | Connection timeout |
| `DB_QUERY_TIMEOUT` | `30s` | Default query timeout |

### Connection String Format

```
postgres://user:password@host:port/database?sslmode=require&pool_max_conns=15
```

**Example:**

```bash
# Gatherer local database
export DB_URL="postgres://gatherer:secret@localhost:5432/kalshi_data?sslmode=disable&pool_max_conns=15"

# Production RDS
export PROD_DB_URL="postgres://dedup:secret@prod-rds.cluster-xxx.us-east-1.rds.amazonaws.com:5432/kalshi_prod?sslmode=verify-full"
```

---

## Pool Settings

### Gatherer Pool

| Setting | Value | Rationale |
|---------|-------|-----------|
| `MaxConns` | 15 | 4 writers × 3 conns + buffer |
| `MinConns` | 5 | Keep connections warm |
| `MaxConnLifetime` | 1h | Prevent stale connections |
| `MaxConnIdleTime` | 15m | Release unused connections |
| `HealthCheckPeriod` | 30s | Detect dead connections |

### Deduplicator Pool

| Setting | Gatherer DBs | Production RDS |
|---------|--------------|----------------|
| `MaxConns` | 2 per gatherer | 6 |
| `MinConns` | 1 per gatherer | 2 |
| `MaxConnLifetime` | 30m | 1h |

**Total deduplicator connections:**
- 12 read connections (2 per database × 2 databases × 3 gatherers)
- 4-6 write connections to production
- **16-18 total**

---

## Timeout Configuration

### Writer Timeouts

| Operation | Timeout | Notes |
|-----------|---------|-------|
| Batch insert | 10s | 100-500 rows |
| Single insert | 5s | Snapshot writes |
| Connection acquire | 5s | From pool |

### Deduplicator Timeouts

| Operation | Timeout | Notes |
|-----------|---------|-------|
| Poll query | 30s | Large batch reads |
| Insert batch | 60s | Up to 50K rows |
| Cursor update | 5s | Small update |

### Query Timeout Configuration

```go
// Per-query timeout using context
ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
defer cancel()

rows, err := pool.Query(ctx, sql, args...)
```

---

## TimescaleDB Settings

### Chunk Interval

| Table | Chunk Interval | Rationale |
|-------|----------------|-----------|
| trades | 1 day | Moderate volume |
| orderbook_deltas | 1 hour | High volume |
| orderbook_snapshots | 1 hour | Large rows |
| tickers | 1 hour | High volume |

```sql
-- Adjust chunk interval
SELECT set_chunk_time_interval('orderbook_deltas', INTERVAL '1 hour');
```

### Compression Settings

| Setting | Value | Description |
|---------|-------|-------------|
| `compress_segmentby` | `ticker` | Segment compressed data by market |
| `compress_orderby` | `exchange_ts DESC` | Sort within segments |

```sql
ALTER TABLE trades SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'ticker',
    timescaledb.compress_orderby = 'exchange_ts DESC'
);
```

---

## Performance Tuning

### PostgreSQL Settings

| Setting | Gatherer | Production | Description |
|---------|----------|------------|-------------|
| `shared_buffers` | 8GB | 16GB | Memory for caching |
| `effective_cache_size` | 24GB | 48GB | Planner cache estimate |
| `work_mem` | 256MB | 512MB | Per-operation memory |
| `maintenance_work_mem` | 2GB | 4GB | For VACUUM, INDEX |
| `max_worker_processes` | 8 | 16 | Background workers |
| `max_parallel_workers` | 8 | 16 | Parallel query workers |

### High-Insert Tuning

```sql
-- Reduce WAL overhead for bulk inserts
ALTER SYSTEM SET wal_level = 'replica';
ALTER SYSTEM SET synchronous_commit = 'off';  -- Gatherer only, not production

-- Increase checkpoint interval
ALTER SYSTEM SET checkpoint_timeout = '15min';
ALTER SYSTEM SET max_wal_size = '4GB';

-- Tune autovacuum for high insert rates
ALTER TABLE orderbook_deltas SET (
    autovacuum_vacuum_scale_factor = 0.01,
    autovacuum_analyze_scale_factor = 0.005
);
```

### Batch Insert Optimization

```go
// Use COPY for large batches (10x faster than INSERT)
_, err := conn.CopyFrom(
    ctx,
    pgx.Identifier{"trades"},
    []string{"trade_id", "exchange_ts", "received_at", "ticker", "price", "size", "taker_side"},
    pgx.CopyFromRows(rows),
)
```

---

## SSL/TLS Configuration

### Gatherer to Local DB

```bash
# Development: no SSL
DB_SSL_MODE=disable

# Production: require SSL
DB_SSL_MODE=require
```

### Deduplicator to RDS

```bash
# Always verify RDS certificate
PROD_DB_SSL_MODE=verify-full
PROD_DB_SSL_ROOT_CERT=/path/to/rds-ca-cert.pem
```

### RDS Certificate

Download from AWS:

```bash
curl -o rds-ca-cert.pem https://truststore.pki.rds.amazonaws.com/global/global-bundle.pem
```

---

## Connection Monitoring

### Pool Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `db_pool_total_conns` | Gauge | Total connections in pool |
| `db_pool_idle_conns` | Gauge | Idle connections |
| `db_pool_acquired_conns` | Gauge | In-use connections |
| `db_pool_acquire_duration_seconds` | Histogram | Time to acquire connection |
| `db_pool_acquire_timeout_total` | Counter | Timeouts acquiring connection |

### Health Check

```go
// Periodic health check
func (db *DB) HealthCheck(ctx context.Context) error {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    return db.pool.Ping(ctx)
}
```

---

## Multi-Database Configuration

### Gatherer (2 databases)

```yaml
databases:
  timescaledb:
    host: localhost
    port: 5432
    database: kalshi_ts
    pool_max_conns: 12
  postgresql:
    host: localhost
    port: 5433
    database: kalshi_pg
    pool_max_conns: 5
```

### Deduplicator (7 databases)

```yaml
gatherers:
  - id: gatherer-1
    timescaledb:
      host: gatherer-1.internal
      port: 5432
      database: kalshi_ts
    postgresql:
      host: gatherer-1.internal
      port: 5433
      database: kalshi_pg
  - id: gatherer-2
    # ...
  - id: gatherer-3
    # ...

production:
  host: prod-rds.cluster-xxx.us-east-1.rds.amazonaws.com
  port: 5432
  database: kalshi_prod
  pool_max_conns: 6
```

---

## Failover Configuration

### RDS Failover

```yaml
production:
  # Use cluster endpoint for automatic failover
  host: prod-rds.cluster-xxx.us-east-1.rds.amazonaws.com

  # Alternatively, specify reader endpoint for read replicas
  reader_host: prod-rds.cluster-ro-xxx.us-east-1.rds.amazonaws.com
```

### Connection Retry

```go
type RetryConfig struct {
    MaxRetries     int           // Maximum retry attempts
    InitialBackoff time.Duration // Initial backoff duration
    MaxBackoff     time.Duration // Maximum backoff duration
    Multiplier     float64       // Backoff multiplier
}

// Defaults
var DefaultRetryConfig = RetryConfig{
    MaxRetries:     5,
    InitialBackoff: 100 * time.Millisecond,
    MaxBackoff:     10 * time.Second,
    Multiplier:     2.0,
}
```
