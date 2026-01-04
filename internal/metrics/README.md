# Metrics Package

Prometheus metrics for monitoring.

## Metrics

### Gatherer Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `ws_connections_active` | Gauge | Active WebSocket connections |
| `ws_messages_total` | Counter | Messages received by channel |
| `ws_reconnections_total` | Counter | Reconnection attempts |
| `writer_batch_size` | Histogram | Batch sizes per writer |
| `writer_latency_seconds` | Histogram | Write latency per writer |
| `buffer_utilization` | Gauge | Buffer fill percentage |
| `buffer_overflow_total` | Counter | Dropped messages due to overflow |

### Deduplicator Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `dedup_records_processed` | Counter | Records processed per source |
| `dedup_duplicates_total` | Counter | Duplicate records skipped |
| `dedup_lag_seconds` | Gauge | Sync lag per gatherer |

## Usage

```go
metrics.Init()
http.Handle("/metrics", promhttp.Handler())
```
