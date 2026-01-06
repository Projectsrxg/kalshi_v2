# Configuration

Config options and metrics for Connection Manager.

---

## Config Struct

```go
type ManagerConfig struct {
    // API credentials
    WSURL      string        // WebSocket URL
    KeyID      string        // API key ID
    PrivateKey interface{}   // *rsa.PrivateKey for signing

    // Timeouts
    SubscribeTimeout time.Duration // 10s

    // Reconnection
    ReconnectBaseWait time.Duration // 1s
    ReconnectMaxWait  time.Duration // 60s

    // Buffers
    MessageBufferSize int // 1000000 (1M messages)

    // Workers
    WorkerCount int // 10

    // Batching
    SubscribeBatchSize int // 1000 (max markets per batch subscribe)
}
```

---

## Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `APIKey` | string | - | Kalshi API key (unique per gatherer) |
| `ConnectTimeout` | Duration | 10s | Timeout for WebSocket connection |
| `SubscribeTimeout` | Duration | 5s | Timeout for subscribe/unsubscribe response |
| `InitialBackoff` | Duration | 1s | Initial reconnection delay |
| `MaxBackoff` | Duration | 5min | Maximum reconnection delay |
| `BackoffFactor` | float64 | 2.0 | Backoff multiplier |
| `MessageBufferSize` | int | 10000 | Output channel buffer size |
| `SubscribeBatchSize` | int | 1000 | Max markets per batch subscribe command (0 = no batching) |

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `WS_RECONNECT_INITIAL` | `1s` | Initial reconnection backoff |
| `WS_RECONNECT_MAX` | `5m` | Maximum reconnection backoff |
| `WS_RECONNECT_FACTOR` | `2.0` | Backoff multiplier |

See [WebSocket Recovery](../recovery/websocket-recovery.md) for reconnection behavior details.

**Fixed allocation (not configurable):**
- 2 ticker connections (1-2)
- 2 trade connections (3-4)
- 2 lifecycle connections (5-6)
- 144 orderbook connections (7-150)

**Constants:**
```go
const MinHealthyConnections = 100  // Startup fails if fewer healthy
```

---

## Error Handling

| Error | Behavior |
|-------|----------|
| Connection failure | Retry with exponential backoff |
| Subscribe timeout | Return error, caller decides |
| Subscribe rejected | Return error with Kalshi error code |
| Sequence gap | Log warning, continue |
| Message buffer full | Drop message, increment counter |

### Error Types

```go
var (
    ErrTimeout         = errors.New("response timeout")
    ErrNoHealthyConns  = errors.New("no healthy connections available")
)
```

---

## Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `conn_manager_connections_total` | Gauge | Total connections (should be 150) |
| `conn_manager_connections_healthy` | Gauge | Healthy connections |
| `conn_manager_subscriptions_total` | Gauge | Active subscriptions |
| `conn_manager_markets_total` | Gauge | Markets with orderbook subscriptions |
| `conn_manager_messages_received_total` | Counter | Messages received |
| `conn_manager_messages_forwarded_total` | Counter | Messages forwarded to router |
| `conn_manager_messages_dropped_total` | Counter | Messages dropped (buffer full) |
| `conn_manager_sequence_gaps_total` | Counter | Sequence gaps detected |
| `conn_manager_reconnects_total` | Counter | Reconnection attempts by connection |
| `conn_manager_subscribe_errors_total` | Counter | Subscribe failures by error type |

### Labels

| Metric | Labels |
|--------|--------|
| `connections_healthy` | `role` (ticker, trade, lifecycle, orderbook) |
| `reconnects_total` | `conn_id`, `role` |
| `subscribe_errors_total` | `channel`, `error_code` |
