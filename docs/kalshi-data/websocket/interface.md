# Interface

Public methods and types for WebSocket Client.

---

## Client Interface

```go
// Client represents a single WebSocket connection to Kalshi
type Client interface {
    // Connect establishes the WebSocket connection
    Connect(ctx context.Context) error

    // Close gracefully closes the connection
    Close() error

    // Subscribe sends a subscribe command, blocks until confirmation
    // Returns subscription results with SIDs for later unsubscribe
    Subscribe(ctx context.Context, channels []string, tickers []string) ([]SubscriptionResult, error)

    // Unsubscribe removes subscriptions by ID, blocks until confirmation
    Unsubscribe(ctx context.Context, sids []int64) error

    // UpdateSubscription adds/removes markets from existing subscription
    UpdateSubscription(ctx context.Context, sid int64, action string, tickers []string) error

    // Messages returns a channel of raw messages (unparsed bytes)
    Messages() <-chan []byte

    // Errors returns a channel of connection errors
    Errors() <-chan error

    // IsConnected returns current connection state
    IsConnected() bool
}
```

---

## Types

### SubscriptionResult

```go
// SubscriptionResult contains the confirmed subscription details
type SubscriptionResult struct {
    SID     int64  // Subscription ID (needed for unsubscribe)
    Channel string // Channel name
}
```

### Command Types

```go
type Command struct {
    ID     int64       `json:"id"`
    Cmd    string      `json:"cmd"`
    Params interface{} `json:"params"`
}

type SubscribeParams struct {
    Channels      []string `json:"channels"`
    MarketTicker  string   `json:"market_ticker,omitempty"`
    MarketTickers []string `json:"market_tickers,omitempty"`
}

type UnsubscribeParams struct {
    SIDs []int64 `json:"sids"`
}

type UpdateParams struct {
    SID           int64    `json:"sid"`
    Action        string   `json:"action"` // "add_markets" or "remove_markets"
    MarketTickers []string `json:"market_tickers"`
}
```

### Response Types

```go
type Response struct {
    ID   int64           `json:"id"`   // Matches command ID
    Type string          `json:"type"` // "subscribed", "unsubscribed", "error"
    Msg  json.RawMessage `json:"msg"`
}

type SubscribedMsg struct {
    SID     int64  `json:"sid"`
    Channel string `json:"channel"`
}

type UnsubscribedMsg struct {
    SIDs []int64 `json:"sids"`
}

type ErrorMsg struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}
```

---

## Internal State

```go
type client struct {
    cfg    ClientConfig
    conn   *websocket.Conn
    logger *slog.Logger

    // Command ID counter (atomic)
    cmdID int64

    // Output channels
    messages chan []byte
    errors   chan error
    done     chan struct{}

    // Pending command responses
    pendingMu sync.Mutex
    pending   map[int64]*pendingRequest

    // Write serialization
    writeMu sync.Mutex

    // State
    mu         sync.RWMutex
    connected  bool
    lastPingAt time.Time
}
```

---

## Concurrency Model

```mermaid
flowchart TD
    subgraph Goroutines
        READ[Read Loop]
        PING[Heartbeat Monitor]
    end

    subgraph Channels
        MSGS[messages chan []byte]
        ERRS[errors chan error]
        RESP[pending response chans]
    end

    READ -->|data messages| MSGS
    READ -->|command responses| RESP
    READ -->|read errors| ERRS
    PING -->|stale connection| ERRS
```

| Goroutine | Lifetime | Purpose |
|-----------|----------|---------|
| Read Loop | Connect to Close | Read messages, route responses |
| Heartbeat Monitor | Connect to Close | Detect stale connections |

**Thread Safety:**
- `writeMu` serializes all writes to connection
- `pendingMu` protects pending request map
- `mu` protects connection state
