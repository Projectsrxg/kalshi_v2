# Configuration

Config options and metrics for Snapshot Poller.

---

## PollerConfig

```go
type PollerConfig struct {
    // Polling
    PollInterval   time.Duration  // Time between poll cycles
    RequestTimeout time.Duration  // Timeout per REST request
    Concurrency    int            // Max concurrent HTTP requests

    // API
    BaseURL string  // REST API base URL
}
```

---

## Config Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `PollInterval` | `time.Duration` | `15 * time.Minute` | Time between polling cycles |
| `RequestTimeout` | `time.Duration` | `30 * time.Second` | Timeout for each REST request |
| `Concurrency` | `int` | `100` | Max concurrent HTTP requests per poll cycle |
| `BaseURL` | `string` | `https://api.elections.kalshi.com/trade-api/v2` | Kalshi REST API base URL |

### Concurrency

Always use max concurrency (100) to poll all markets as fast as possible:

| Concurrency | Avg Latency | Markets/15min |
|-------------|-------------|---------------|
| 100 | 100ms | ~900,000 |
| 100 | 50ms | ~1,800,000 |

### Environment Variables

```bash
POLLER_POLL_INTERVAL=15m      # Polling interval
POLLER_REQUEST_TIMEOUT=30s    # Per-request timeout
POLLER_CONCURRENCY=100        # Max concurrent requests
KALSHI_REST_URL=https://api.elections.kalshi.com/trade-api/v2
```

### Example Config

```go
cfg := PollerConfig{
    PollInterval:   15 * time.Minute,
    RequestTimeout: 30 * time.Second,
    Concurrency:    100,
    BaseURL:        "https://api.elections.kalshi.com/trade-api/v2",
}
```

---

## Metrics

### PollerMetrics

```go
type PollerMetrics struct {
    // Counters
    SnapshotsFetched prometheus.Counter  // Total successful fetches
    FetchErrors      prometheus.Counter  // REST API errors
    WriteErrors      prometheus.Counter  // Writer errors

    // Gauges
    MarketsPolled prometheus.Gauge  // Markets in last poll cycle

    // Histograms
    PollDuration prometheus.Histogram  // Duration of poll cycle
}
```

### Metric Definitions

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `poller_snapshots_fetched_total` | Counter | - | Total REST snapshots successfully fetched |
| `poller_fetch_errors_total` | Counter | - | REST API fetch errors |
| `poller_write_errors_total` | Counter | - | Snapshot Writer errors |
| `poller_markets_polled` | Gauge | - | Number of markets in last poll |
| `poller_poll_duration_seconds` | Histogram | - | Time to complete one poll cycle |

### Prometheus Registration

```go
func newPollerMetrics() *PollerMetrics {
    return &PollerMetrics{
        SnapshotsFetched: promauto.NewCounter(prometheus.CounterOpts{
            Name: "poller_snapshots_fetched_total",
            Help: "Total number of REST snapshots fetched",
        }),
        FetchErrors: promauto.NewCounter(prometheus.CounterOpts{
            Name: "poller_fetch_errors_total",
            Help: "Total number of REST API fetch errors",
        }),
        WriteErrors: promauto.NewCounter(prometheus.CounterOpts{
            Name: "poller_write_errors_total",
            Help: "Total number of Snapshot Writer errors",
        }),
        MarketsPolled: promauto.NewGauge(prometheus.GaugeOpts{
            Name: "poller_markets_polled",
            Help: "Number of markets polled in last cycle",
        }),
        PollDuration: promauto.NewHistogram(prometheus.HistogramOpts{
            Name:    "poller_poll_duration_seconds",
            Help:    "Duration of poll cycle in seconds",
            Buckets: []float64{60, 120, 300, 600, 900, 1200},
        }),
    }
}
```

---

## Logging

### Log Fields

| Field | Type | Description |
|-------|------|-------------|
| `component` | string | Always `"snapshot-poller"` |
| `ticker` | string | Market ticker (on per-market logs) |
| `markets` | int | Number of markets polled |
| `fetched` | int | Successful fetches in cycle |
| `errors` | int | Errors in cycle |
| `duration` | duration | Poll cycle duration |
| `err` | error | Error details |

### Log Examples

```
level=INFO msg="snapshot poller started" component=snapshot-poller poll_interval=15m0s
level=DEBUG msg="poll cycle complete" component=snapshot-poller markets=500 fetched=498 errors=2 duration=45.2s
level=WARN msg="failed to fetch orderbook" component=snapshot-poller ticker=EXAMPLE-TICKER err="unexpected status: 404"
level=INFO msg="stopping snapshot poller" component=snapshot-poller
level=INFO msg="snapshot poller stopped" component=snapshot-poller
```

---

## Tuning Guidelines

### Poll Interval

| Scenario | Recommended | Notes |
|----------|-------------|-------|
| Standard | 15 minutes | Default, balances coverage and API load |
| High reliability | 5 minutes | More frequent backup |
| Low priority | 30 minutes | Reduced API load, coarser backup |

### Request Timeout

| Network | Recommended | Notes |
|---------|-------------|-------|
| Same region | 10s | Low latency |
| Cross-region | 30s | Default, handles variable latency |
| Unreliable | 60s | Prevents excessive timeouts |

---

## Monitoring Alerts

### Suggested Alerts

```yaml
# High fetch error rate
- alert: PollerHighErrorRate
  expr: rate(poller_fetch_errors_total[5m]) > 0.1
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "Snapshot Poller fetch error rate high"

# Poll cycle too slow
- alert: PollerSlowCycle
  expr: poller_poll_duration_seconds > 900
  for: 2m
  labels:
    severity: warning
  annotations:
    summary: "Poll cycle approaching 15-minute interval limit"

# No markets being polled
- alert: PollerNoMarkets
  expr: poller_markets_polled == 0
  for: 5m
  labels:
    severity: critical
  annotations:
    summary: "No markets being polled"
```
