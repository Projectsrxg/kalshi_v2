# Configuration

Config options, error handling, and metrics for WebSocket Client.

---

## Config Struct

```go
type ClientConfig struct {
    // Connection
    URL    string // wss://api.elections.kalshi.com
    APIKey string

    // Timeouts
    DialTimeout     time.Duration // 10s
    WriteTimeout    time.Duration // 5s
    ResponseTimeout time.Duration // 5s - max wait for command response

    // Buffers
    MessageBufferSize int // 10000
    ErrorBufferSize   int // 10

    // Heartbeat
    PingTimeout time.Duration // 30s (consider stale if no ping)
}
```

---

## Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `URL` | string | `wss://api.elections.kalshi.com` | WebSocket endpoint |
| `APIKey` | string | - | API key for auth header |
| `DialTimeout` | Duration | 10s | Connection timeout |
| `WriteTimeout` | Duration | 5s | Write deadline |
| `ResponseTimeout` | Duration | 5s | Max wait for subscribe/unsubscribe response |
| `MessageBufferSize` | int | 10000 | Buffer for message channel |
| `ErrorBufferSize` | int | 10 | Buffer for error channel |
| `PingTimeout` | Duration | 30s | Mark stale if no ping received |

---

## Error Handling

| Error | Behavior |
|-------|----------|
| Dial failure | Return error from `Connect()` |
| Read error | Send to `Errors()` channel, exit read loop |
| Write error | Return error from command method |
| Ping timeout | Send `ErrStaleConnection` to `Errors()` channel |
| Response timeout | Return `ErrTimeout` from command method |
| Command rejected | Return error with Kalshi's error code/message |

### Error Types

```go
var (
    ErrNotConnected   = errors.New("not connected")
    ErrTimeout        = errors.New("response timeout")
    ErrStaleConnection = errors.New("connection stale: no ping received")
)
```

**Note**: Client does NOT attempt reconnection. That's Connection Manager's job.

---

## Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `ws_client_messages_received_total` | Counter | Raw messages received |
| `ws_client_bytes_received_total` | Counter | Bytes received |
| `ws_client_commands_sent_total` | Counter | Commands by type (subscribe, unsubscribe) |
| `ws_client_command_duration_seconds` | Histogram | Time to get command response |
| `ws_client_errors_total` | Counter | Errors by type |
| `ws_client_last_ping_timestamp` | Gauge | Last ping received |
| `ws_client_connected` | Gauge | 1 if connected, 0 otherwise |
