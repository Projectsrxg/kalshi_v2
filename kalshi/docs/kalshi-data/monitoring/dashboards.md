# Dashboards

Grafana dashboard designs for the Kalshi Data Platform.

---

## Dashboard Overview

| Dashboard | Purpose | Refresh |
|-----------|---------|---------|
| System Overview | High-level health at a glance | 10s |
| Data Pipeline | Message flow through components | 10s |
| Writer Performance | Database write metrics | 30s |
| Snapshot Poller | REST polling metrics | 1m |
| Deduplicator | Cross-gatherer sync | 30s |

---

## System Overview Dashboard

Primary dashboard for on-call engineers. Shows system health at a glance.

### Row 1: Health Status (Stat Panels)

```
┌─────────────────┬─────────────────┬─────────────────┐
│   Gatherers     │   WebSocket     │   Active        │
│     3/3         │  Connections    │   Markets       │
│                 │    450/450      │    1,250        │
└─────────────────┴─────────────────┴─────────────────┘
```

| Panel | Query | Thresholds |
|-------|-------|------------|
| Gatherers | `sum(up{job="gatherers"})` | Green: 3, Yellow: 2, Red: <2 |
| WebSocket | `sum(conn_manager_connections_healthy)` | Green: 450, Yellow: 400, Red: <300 |
| Active Markets | `sum(market_registry_active_markets)` | Green: >0 |

**Note:** WebSocket threshold 450 = 3 gatherers × 150 connections per gatherer. See [connection-manager/configuration.md](../connection-manager/configuration.md) for per-gatherer connection breakdown.

### Row 2: Throughput (Graph Panels)

```
┌─────────────────────────────────────────────────────┐
│           Message Throughput (msg/s)                │
│  ━━━ received  ━━━ routed  ━━━ inserted            │
│                                                     │
│  50k ┤                    ╱╲                        │
│  25k ┤    ╱╲    ╱╲      ╱  ╲                       │
│   0k ┼────────────────────────────────────────────  │
│      10:00   10:05   10:10   10:15   10:20         │
└─────────────────────────────────────────────────────┘
```

| Series | Query |
|--------|-------|
| Received | `sum(rate(conn_manager_messages_received_total[1m]))` |
| Routed | `sum(rate(router_messages_routed_total[1m]))` |
| Inserted | `sum(rate(writer_inserts_total[1m]))` |

### Row 3: Error Rates (Graph Panel)

```
┌─────────────────────────────────────────────────────┐
│              Error Rates (errors/s)                 │
│  ━━━ ws_errors  ━━━ parse_errors  ━━━ write_errors │
│                                                     │
│  10  ┤                                              │
│   5  ┤        ╱╲                                    │
│   0  ┼────────────────────────────────────────────  │
└─────────────────────────────────────────────────────┘
```

| Series | Query |
|--------|-------|
| WS Errors | `sum(rate(ws_client_errors_total[1m]))` |
| Parse Errors | `sum(rate(router_parse_errors_total[1m]))` |
| Write Errors | `sum(rate(writer_errors_total[1m]))` |

### Row 4: Deduplicator (Stat + Graph)

```
┌─────────────────┬───────────────────────────────────┐
│   Sync Lag      │        Records/Second             │
│     2.3s        │  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━   │
│                 │  5k ┤   ╱╲    ╱╲    ╱╲           │
│                 │   0 ┼─────────────────────────────│
└─────────────────┴───────────────────────────────────┘
```

---

## Data Pipeline Dashboard

Visualizes message flow from WebSocket to database.

### Row 1: Pipeline Flow (Diagram + Stats)

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Data Pipeline                                │
│                                                                      │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐      │
│  │WebSocket │───▶│  Router  │───▶│ Writers  │───▶│   DB     │      │
│  │  50k/s   │    │  49.8k/s │    │  49.5k/s │    │  49.5k/s │      │
│  └──────────┘    └──────────┘    └──────────┘    └──────────┘      │
│       │               │               │                              │
│     Drops: 0      Drops: 200     Errors: 50                         │
└─────────────────────────────────────────────────────────────────────┘
```

### Row 2: Per-Stage Throughput

| Panel | Query |
|-------|-------|
| WS Received | `sum(rate(ws_client_messages_received_total[1m]))` |
| WS Dropped | `sum(rate(ws_client_messages_dropped_total[1m]))` |
| Router Received | `sum(rate(router_messages_received_total[1m]))` |
| Router Dropped | `sum(rate(router_messages_dropped_total[1m]))` |
| Writer Inserts | `sum(rate(writer_inserts_total[1m]))` |
| Writer Errors | `sum(rate(writer_errors_total[1m]))` |

### Row 3: Message Type Breakdown

```
┌─────────────────────────────────────────────────────┐
│           Messages by Type (msg/s)                  │
│  ████ orderbook_delta  ████ trade  ████ ticker     │
│                                                     │
│  40k ┤████████████████████████                      │
│  20k ┤████████████████████████████████             │
│   0k ┤████████████████████████████████████████████ │
└─────────────────────────────────────────────────────┘
```

Query: `sum(rate(router_messages_routed_total[1m])) by (type)`

### Row 4: Latency Distribution

```
┌─────────────────────────────────────────────────────┐
│           Flush Latency by Writer (ms)              │
│                                                     │
│  orderbook  ├────────────┤ P50: 5ms  P99: 50ms     │
│  trade      ├───────┤     P50: 3ms  P99: 20ms      │
│  ticker     ├────┤        P50: 2ms  P99: 10ms      │
│  snapshot   ├──────────────────┤ P50: 10ms P99:80ms│
└─────────────────────────────────────────────────────┘
```

Query: `histogram_quantile(0.99, sum(rate(writer_flush_duration_seconds_bucket[5m])) by (le, writer))`

---

## Writer Performance Dashboard

Detailed metrics for each writer.

### Row 1: Throughput by Writer

```
┌─────────────────────────────────────────────────────┐
│           Insert Rate by Writer (rows/s)            │
│  ━━━ orderbook  ━━━ trade  ━━━ ticker  ━━━ snapshot│
│                                                     │
│  20k ┤━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━  │
│  10k ┤                    ━━━━━━━━━━━━━━━━━━━━━━━━  │
│   0k ┤────────────────────────────────────────────  │
└─────────────────────────────────────────────────────┘
```

### Row 2: Batch Metrics

| Panel | Query |
|-------|-------|
| Avg Batch Size | `histogram_quantile(0.5, sum(rate(writer_batch_size_bucket[5m])) by (le, writer))` |
| Max Batch Size | `histogram_quantile(0.99, sum(rate(writer_batch_size_bucket[5m])) by (le, writer))` |
| Batches/sec | `sum(rate(writer_flush_duration_seconds_count[1m])) by (writer)` |

### Row 3: Conflict/Duplicate Rate

```
┌─────────────────────────────────────────────────────┐
│           Duplicate Rate by Writer (%)              │
│  Expected: ~66% (3 gatherers writing same data)     │
│                                                     │
│  orderbook  ████████████████████████████  68%       │
│  trade      ██████████████████████████    65%       │
│  ticker     ████████████████████████████  67%       │
└─────────────────────────────────────────────────────┘
```

Query: `rate(writer_conflicts_total[5m]) / rate(writer_inserts_total[5m])`

### Row 4: Error Breakdown

| Panel | Query |
|-------|-------|
| Connection Errors | `sum(rate(writer_errors_total{type="connection"}[5m])) by (writer)` |
| Constraint Errors | `sum(rate(writer_errors_total{type="constraint"}[5m])) by (writer)` |
| Timeout Errors | `sum(rate(writer_errors_total{type="timeout"}[5m])) by (writer)` |

---

## Snapshot Poller Dashboard

REST polling metrics.

### Row 1: Poll Cycle Overview

```
┌─────────────────┬─────────────────┬─────────────────┐
│ Markets Polled  │ Success Rate    │ Cycle Duration  │
│     1,250       │     99.8%       │     45.2s       │
└─────────────────┴─────────────────┴─────────────────┘
```

### Row 2: Poll Duration Over Time

```
┌─────────────────────────────────────────────────────┐
│           Poll Cycle Duration (seconds)             │
│  ━━━ P50  ━━━ P95  ─── 60s limit                   │
│                                                     │
│  60s ┤─────────────────────────────────────────────│
│  30s ┤    ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━   │
│   0s ┤────────────────────────────────────────────  │
└─────────────────────────────────────────────────────┘
```

### Row 3: Fetch Results

| Panel | Query |
|-------|-------|
| Fetched | `sum(rate(poller_snapshots_fetched_total[5m]))` |
| Fetch Errors | `sum(rate(poller_fetch_errors_total[5m]))` |
| Write Errors | `sum(rate(poller_write_errors_total[5m]))` |

---

## Deduplicator Dashboard

Cross-gatherer synchronization.

### Row 1: Gatherer Health

```
┌─────────────────┬─────────────────┬─────────────────┐
│   Gatherer 1    │   Gatherer 2    │   Gatherer 3    │
│   ● Connected   │   ● Connected   │   ● Connected   │
│   Lag: 2.1s     │   Lag: 1.8s     │   Lag: 2.5s     │
└─────────────────┴─────────────────┴─────────────────┘
```

### Row 2: Sync Metrics

| Panel | Query |
|-------|-------|
| Sync Lag | `dedup_sync_lag_seconds` |
| Write Rate | `dedup_records_per_second` |
| Duplicate Rate | `dedup_duplicate_rate` |

### Row 3: Records Synced Over Time

```
┌─────────────────────────────────────────────────────┐
│           Records Written to RDS (rows/s)           │
│                                                     │
│  10k ┤    ╱╲    ╱╲    ╱╲    ╱╲    ╱╲              │
│   5k ┤  ╱    ╲╱    ╲╱    ╲╱    ╲╱    ╲            │
│   0k ┤────────────────────────────────────────────  │
└─────────────────────────────────────────────────────┘
```

---

## Dashboard Variables

All dashboards support these Grafana variables:

| Variable | Query | Default |
|----------|-------|---------|
| `$instance` | `label_values(up, instance)` | All |
| `$writer` | `label_values(writer_inserts_total, writer)` | All |
| `$interval` | - | `$__auto_interval` |

---

## Dashboard Provisioning

```yaml
# /etc/grafana/provisioning/dashboards/kalshi.yaml
apiVersion: 1
providers:
  - name: 'Kalshi'
    orgId: 1
    folder: 'Kalshi Data Platform'
    type: file
    disableDeletion: false
    updateIntervalSeconds: 30
    options:
      path: /var/lib/grafana/dashboards/kalshi
```
