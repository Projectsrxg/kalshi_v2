# Configuration

Configuration options for Market Registry.

---

## Config Struct

```go
type RegistryConfig struct {
    // REST client config
    RESTBaseURL string        // https://api.elections.kalshi.com/trade-api/v2
    RESTTimeout time.Duration // 30s

    // Polling intervals
    ExchangeCheckInterval time.Duration // 1 min
    ReconcileInterval     time.Duration // 5 min
    EventSyncInterval     time.Duration // 10 min

    // Pagination
    PageSize int // 1000 (max)

    // Change notification
    ChangeBufferSize int // 1000
}
```

---

## Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `RESTBaseURL` | string | `https://api.elections.kalshi.com/trade-api/v2` | Kalshi REST API base URL |
| `RESTTimeout` | Duration | 30s | Timeout for REST requests |
| `ExchangeCheckInterval` | Duration | 1 min | How often to poll exchange status |
| `ReconcileInterval` | Duration | 5 min | How often to run full reconciliation |
| `EventSyncInterval` | Duration | 10 min | How often to sync events table |
| `PageSize` | int | 1000 | Markets per page (max 1000) |
| `ChangeBufferSize` | int | 1000 | Buffer size for change notification channel |

---

## Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `market_registry_markets_total` | Gauge | Total markets in cache |
| `market_registry_active_markets` | Gauge | Active markets count |
| `market_registry_lifecycle_events_total` | Counter | Lifecycle events by type |
| `market_registry_reconcile_duration_seconds` | Histogram | Reconciliation duration |
| `market_registry_rest_errors_total` | Counter | REST errors by endpoint |
| `market_registry_last_sync_timestamp` | Gauge | Unix timestamp of last successful sync |
