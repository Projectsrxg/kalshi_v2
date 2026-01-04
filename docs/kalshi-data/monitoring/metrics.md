# Metrics Reference

Complete Prometheus metrics by component.

---

## Summary

| Component | Counters | Gauges | Histograms | Total |
|-----------|----------|--------|------------|-------|
| Market Registry | 2 | 4 | 1 | 7 |
| WebSocket Client | 5 | 2 | 0 | 7 |
| Connection Manager | 5 | 5 | 0 | 10 |
| Message Router | 5 | 0 | 0 | 5 |
| Writers | 5 | 0 | 2 | 7 |
| Snapshot Poller | 3 | 1 | 1 | 5 |
| Deduplicator | 0 | 5 | 0 | 5 |
| Database Pool | 1 | 3 | 1 | 5 |
| S3 Export | 2 | 1 | 1 | 5 |
| **Total** | **28** | **21** | **6** | **56** |

---

## Market Registry

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `market_registry_markets_total` | Gauge | - | Total markets in cache |
| `market_registry_active_markets` | Gauge | - | Markets currently open for trading |
| `market_registry_lifecycle_events_total` | Counter | `type` | Lifecycle events received |
| `market_registry_reconcile_duration_seconds` | Histogram | - | Time to complete reconciliation |
| `market_registry_rest_errors_total` | Counter | `endpoint` | REST API errors |
| `market_registry_last_sync_timestamp` | Gauge | - | Unix timestamp of last successful sync |
| `market_registry_exchange_active` | Gauge | - | 1 if exchange is active, 0 otherwise |

**Labels:**
- `type`: `created`, `status_change`, `settled`
- `endpoint`: `markets`, `events`, `series`

**Histogram Buckets:**
- `market_registry_reconcile_duration_seconds`: `[1, 5, 10, 30, 60, 120, 300]`

**Example Queries:**

```promql
# Active market count
market_registry_active_markets

# Lifecycle event rate by type
rate(market_registry_lifecycle_events_total[5m])

# Time since last sync
time() - market_registry_last_sync_timestamp
```

---

## WebSocket Client

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `ws_client_messages_received_total` | Counter | - | Raw messages received |
| `ws_client_messages_dropped_total` | Counter | - | Messages dropped (buffer full) |
| `ws_client_bytes_received_total` | Counter | - | Bytes received |
| `ws_client_bytes_sent_total` | Counter | - | Bytes sent |
| `ws_client_errors_total` | Counter | `type` | Errors by type |
| `ws_client_last_ping_timestamp` | Gauge | - | Last ping received (Unix) |
| `ws_client_connected` | Gauge | - | 1 if connected, 0 otherwise |

**Labels:**
- `type`: `read`, `write`, `connect`, `ping_timeout`

**Example Queries:**

```promql
# Message throughput
rate(ws_client_messages_received_total[1m])

# Connection status across gatherers
sum(ws_client_connected) by (instance)

# Drop rate
rate(ws_client_messages_dropped_total[5m]) / rate(ws_client_messages_received_total[5m])
```

---

## Connection Manager

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `conn_manager_connections_total` | Gauge | - | Total connections (should be 150) |
| `conn_manager_connections_healthy` | Gauge | `role` | Healthy connections by role |
| `conn_manager_subscriptions_total` | Gauge | - | Active subscriptions |
| `conn_manager_markets_total` | Gauge | - | Markets with orderbook subscriptions |
| `conn_manager_messages_received_total` | Counter | - | Messages received from WebSocket |
| `conn_manager_messages_forwarded_total` | Counter | - | Messages forwarded to router |
| `conn_manager_messages_dropped_total` | Counter | - | Messages dropped (buffer full) |
| `conn_manager_sequence_gaps_total` | Counter | - | Sequence gaps detected |
| `conn_manager_reconnects_total` | Counter | `conn_id`, `role` | Reconnection attempts |
| `conn_manager_subscribe_errors_total` | Counter | `channel`, `error_code` | Subscribe failures |

**Labels:**
- `role`: `ticker`, `trade`, `lifecycle`, `orderbook`
- `channel`: `orderbook_delta`, `trade`, `ticker`
- `error_code`: `not_found`, `rate_limit`, `invalid`

**Example Queries:**

```promql
# Healthy connections by role
conn_manager_connections_healthy

# Connection health ratio
sum(conn_manager_connections_healthy) / conn_manager_connections_total

# Reconnection rate
rate(conn_manager_reconnects_total[5m])

# Sequence gap rate
rate(conn_manager_sequence_gaps_total[5m])
```

---

## Message Router

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `router_messages_received_total` | Counter | `type` | Messages received from Connection Manager |
| `router_messages_routed_total` | Counter | `type` | Messages successfully routed |
| `router_messages_dropped_total` | Counter | `type` | Messages dropped (buffer full) |
| `router_parse_errors_total` | Counter | `type` | JSON parse failures |
| `router_unknown_messages_total` | Counter | - | Messages with unknown type |

**Labels:**
- `type`: `orderbook_snapshot`, `orderbook_delta`, `trade`, `ticker`

**Example Queries:**

```promql
# Routing success rate
sum(rate(router_messages_routed_total[5m])) /
sum(rate(router_messages_received_total[5m]))

# Drop rate by message type
rate(router_messages_dropped_total[5m])

# Parse error rate
rate(router_parse_errors_total[5m])
```

---

## Writers

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `writer_messages_received_total` | Counter | `writer` | Messages received from channel |
| `writer_inserts_total` | Counter | `writer` | Rows successfully inserted |
| `writer_conflicts_total` | Counter | `writer` | Duplicates (ON CONFLICT hit) |
| `writer_errors_total` | Counter | `writer`, `type` | Insert failures |
| `writer_seq_gaps_total` | Counter | `writer`, `ticker` | Sequence gaps detected (orderbook only) |
| `writer_batch_size` | Histogram | `writer` | Rows per batch |
| `writer_flush_duration_seconds` | Histogram | `writer` | Time to execute batch insert |

**Labels:**
- `writer`: `orderbook`, `trade`, `ticker`, `snapshot`
- `type` (errors): `connection`, `constraint`, `timeout`
- `ticker`: Market ticker (for seq_gaps only)

**Histogram Buckets:**
- `writer_batch_size`: `[10, 50, 100, 500, 1000, 5000]`
- `writer_flush_duration_seconds`: `[0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0]`

**Note:** Snapshot Writer (`writer="snapshot"`) is synchronous and doesn't batch. It emits `writer_inserts_total` and `writer_errors_total` but NOT `writer_batch_size` or `writer_flush_duration_seconds`.

**Example Queries:**

```promql
# Insert throughput by writer
sum(rate(writer_inserts_total[5m])) by (writer)

# Duplicate rate (expected to be non-zero with 3 gatherers)
rate(writer_conflicts_total[5m]) / rate(writer_inserts_total[5m])

# Average batch size
histogram_quantile(0.5, sum(rate(writer_batch_size_bucket[5m])) by (le, writer))

# P99 flush latency
histogram_quantile(0.99, sum(rate(writer_flush_duration_seconds_bucket[5m])) by (le, writer))

# Error rate
rate(writer_errors_total[5m])
```

---

## Snapshot Poller

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `poller_snapshots_fetched_total` | Counter | - | REST snapshots successfully fetched |
| `poller_fetch_errors_total` | Counter | - | REST API fetch errors |
| `poller_write_errors_total` | Counter | - | Snapshot Writer errors |
| `poller_markets_polled` | Gauge | - | Markets in last poll cycle |
| `poller_poll_duration_seconds` | Histogram | - | Time to complete poll cycle |

**Histogram Buckets:**
- `poller_poll_duration_seconds`: `[1, 5, 10, 30, 60, 120]`

**Example Queries:**

```promql
# Fetch success rate
rate(poller_snapshots_fetched_total[5m]) /
(rate(poller_snapshots_fetched_total[5m]) + rate(poller_fetch_errors_total[5m]))

# Poll cycle duration P95
histogram_quantile(0.95, sum(rate(poller_poll_duration_seconds_bucket[5m])) by (le))

# Markets being polled
poller_markets_polled
```

---

## Deduplicator

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `dedup_sync_lag_seconds` | Gauge | - | Time since last successful sync |
| `dedup_records_per_second` | Gauge | - | Write throughput to RDS |
| `dedup_duplicate_rate` | Gauge | - | % of records already in RDS |
| `dedup_gatherer_health` | Gauge | `gatherer` | Per-gatherer connection status (1=connected, 0=disconnected) |
| `dedup_rds_connected` | Gauge | - | 1 if production RDS reachable, 0 otherwise |

**Labels:**
- `gatherer`: `gatherer-1`, `gatherer-2`, `gatherer-3`

**Example Queries:**

```promql
# Sync lag
dedup_sync_lag_seconds

# All gatherers healthy
sum(dedup_gatherer_health) == 3

# Write throughput
dedup_records_per_second
```

---

## Database Pool

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `db_pool_total_conns` | Gauge | `database` | Total connections in pool |
| `db_pool_idle_conns` | Gauge | `database` | Idle connections |
| `db_pool_acquired_conns` | Gauge | `database` | In-use connections |
| `db_pool_acquire_duration_seconds` | Histogram | `database` | Time to acquire connection |
| `db_pool_acquire_timeout_total` | Counter | `database` | Timeouts acquiring connection |

**Labels:**
- `database`: `timescaledb`, `postgresql`, `production`

**Histogram Buckets:**
- `db_pool_acquire_duration_seconds`: `[0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0]`

**Example Queries:**

```promql
# Pool utilization
db_pool_acquired_conns / db_pool_total_conns

# Acquire timeout rate
rate(db_pool_acquire_timeout_total[5m])

# P99 acquire latency
histogram_quantile(0.99, sum(rate(db_pool_acquire_duration_seconds_bucket[5m])) by (le, database))
```

---

## S3 Export

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `export_rows_total` | Counter | `table` | Rows exported to S3 |
| `export_bytes_total` | Counter | `table` | Bytes written to S3 |
| `export_duration_seconds` | Histogram | `table` | Export job duration |
| `export_errors_total` | Counter | `table`, `type` | Failed exports |
| `export_lag_seconds` | Gauge | `table` | Time since last successful export |

**Labels:**
- `table`: `trades`, `orderbook_deltas`, `orderbook_snapshots`, `tickers`
- `type` (errors): `query`, `s3_upload`, `parquet_write`

**Histogram Buckets:**
- `export_duration_seconds`: `[1, 5, 10, 30, 60, 120, 300]`

**Example Queries:**

```promql
# Export throughput
sum(rate(export_rows_total[5m])) by (table)

# Export lag per table
export_lag_seconds

# Error rate
rate(export_errors_total[5m])
```

---

## Prometheus Scrape Config

```yaml
scrape_configs:
  - job_name: 'gatherers'
    static_configs:
      - targets:
        - 'gatherer-1.internal:9090'
        - 'gatherer-2.internal:9090'
        - 'gatherer-3.internal:9090'
    scrape_interval: 15s

  - job_name: 'deduplicator'
    static_configs:
      - targets:
        - 'deduplicator.internal:9090'
    scrape_interval: 15s
```

---

## Naming Conventions

All metrics follow Prometheus naming conventions:

| Pattern | Example |
|---------|---------|
| `<component>_<noun>_total` | `writer_inserts_total` |
| `<component>_<noun>_seconds` | `writer_flush_duration_seconds` |
| `<component>_<noun>` (gauge) | `market_registry_active_markets` |

Units are always in base units (seconds, bytes, not milliseconds or KB).
