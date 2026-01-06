# Configuration

Config options, error handling, and metrics for WebSocket Client.

---

## Config Struct

```go
type ClientConfig struct {
    // Connection
    URL        string        // wss://api.elections.kalshi.com/trade-api/ws/v2
    KeyID      string        // API key ID for KALSHI-ACCESS-KEY header
    PrivateKey *rsa.PrivateKey // RSA private key for signing

    // Timeouts
    DialTimeout  time.Duration // 10s
    WriteTimeout time.Duration // 5s

    // Buffers
    MessageBufferSize int // 10000
    ErrorBufferSize   int // 10

    // Heartbeat
    PingTimeout time.Duration // 60s (consider stale if no ping)
}
```

---

## Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `URL` | string | `wss://api.elections.kalshi.com/trade-api/ws/v2` | WebSocket endpoint |
| `KeyID` | string | - | API key ID for auth header |
| `PrivateKey` | *rsa.PrivateKey | - | RSA private key for signing |
| `DialTimeout` | Duration | 10s | Connection timeout |
| `WriteTimeout` | Duration | 5s | Write deadline |
| `MessageBufferSize` | int | 10000 | Buffer for message channel |
| `ErrorBufferSize` | int | 10 | Buffer for error channel |
| `PingTimeout` | Duration | 60s | Mark stale if no ping received |

---

## Error Handling

| Error | Behavior |
|-------|----------|
| Dial failure | Return error from `Connect()` |
| Read error | Send to `Errors()` channel, exit read loop |
| Write error | Return error from `Send()` |
| Ping timeout | Send `ErrStaleConnection` to `Errors()` channel |

### Error Types

```go
var (
    ErrNotConnected    = errors.New("not connected")
    ErrStaleConnection = errors.New("connection stale: no ping received")
)
```

**Note**: Client does NOT attempt reconnection. Connection Manager handles reconnection policy.

---

## Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `ws_client_messages_received_total` | Counter | Raw messages received |
| `ws_client_messages_dropped_total` | Counter | Messages dropped due to full buffer |
| `ws_client_bytes_received_total` | Counter | Bytes received |
| `ws_client_bytes_sent_total` | Counter | Bytes sent |
| `ws_client_errors_total` | Counter | Errors by type |
| `ws_client_last_ping_timestamp` | Gauge | Last ping received |
| `ws_client_connected` | Gauge | 1 if connected, 0 otherwise |
