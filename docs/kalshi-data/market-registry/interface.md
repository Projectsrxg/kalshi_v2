# Interface

Public methods and types for Market Registry.

---

## Registry Interface

```go
// Registry manages market discovery and lifecycle
type Registry interface {
    // Start begins market discovery in background, returns immediately.
    // Emits MarketChange events as markets are discovered.
    Start(ctx context.Context) error

    // Stop gracefully shuts down
    Stop(ctx context.Context) error

    // GetActiveMarkets returns all markets currently open for trading
    GetActiveMarkets() []Market

    // GetMarket returns a specific market by ticker
    GetMarket(ticker string) (Market, bool)

    // SubscribeChanges returns a channel of market state changes.
    // Connection Manager uses this to know when to subscribe/unsubscribe.
    // Channel is buffered (ChangeBufferSize = 1000).
    SubscribeChanges() <-chan MarketChange

    // SetLifecycleSource sets the channel from which lifecycle messages are received.
    // Connection Manager calls this to provide market_lifecycle WebSocket messages.
    SetLifecycleSource(ch <-chan []byte)
}
```

**Design Decision**: Channel-based interface for async notification to Connection Manager. Go-idiomatic and non-blocking.

**Constants:**
```go
const ChangeBufferSize = 1000  // Buffered channel for MarketChange events
```

If Connection Manager is slow (blocking on subscribes), the buffer fills. Connection Manager uses a worker pool to prevent this.

---

## Types

### MarketChange

```go
// MarketChange represents a market state transition
type MarketChange struct {
    Ticker    string
    EventType string  // "created", "status_change", "settled"
    OldStatus string
    NewStatus string
    Market    *Market // Full market data (nil for "settled")
}
```

| Field | Type | Description |
|-------|------|-------------|
| `Ticker` | string | Market ticker |
| `EventType` | string | `created`, `status_change`, or `settled` |
| `OldStatus` | string | Previous status (for `status_change`) |
| `NewStatus` | string | New status |
| `Market` | *Market | Full market data (nil for `settled`) |

### Market

```go
type Market struct {
    Ticker        string
    EventTicker   string
    Title         string
    MarketStatus  string  // initialized, inactive, active, closed, determined, disputed, amended, finalized
    TradingStatus string
    MarketType    string  // binary, scalar
    Result        string  // yes, no, empty
    YesBid        int
    YesAsk        int
    LastPrice     int
    Volume        int64
    OpenTs        int64   // microseconds
    CloseTs       int64
    ExpirationTs  int64
    UpdatedAt     int64
}
```

---

## Internal State

### In-Memory Cache

```go
type registryState struct {
    mu sync.RWMutex

    // All known markets indexed by ticker
    markets map[string]*Market

    // Markets currently active (open for trading)
    activeSet map[string]struct{}

    // Last successful REST sync timestamp
    lastSyncAt time.Time

    // Exchange status
    exchangeActive bool
    tradingActive  bool

    // Output channel for Connection Manager
    changes chan MarketChange  // buffered, ChangeBufferSize

    // Input channel from Connection Manager (market_lifecycle messages)
    lifecycle <-chan []byte
}
```

**Design Decision**: Cache ALL markets in memory. At 1M markets × ~500 bytes = 500MB, which is acceptable for a 32GB instance. Simpler than LRU and avoids DB lookups.

---

## Concurrency Model

```mermaid
flowchart TD
    subgraph Goroutines
        SYNC[Initial Sync]
        RECON[Reconciliation Loop]
        STATUS[Exchange Status Loop]
        EVENTS[Event Sync Loop]
        HANDLER[Lifecycle Handler]
    end

    subgraph Shared State
        CACHE[Market Cache<br/>sync.RWMutex]
        CHANGECH[Change Channel<br/>buffered 1000]
    end

    subgraph External
        LIFECYCLE[lifecycle chan<br/>from Connection Manager]
    end

    SYNC --> CACHE
    SYNC --> CHANGECH
    RECON --> CACHE
    RECON --> CHANGECH
    STATUS --> CACHE
    EVENTS --> CACHE
    LIFECYCLE --> HANDLER
    HANDLER --> CACHE
    HANDLER --> CHANGECH
```

| Goroutine | Purpose | Access |
|-----------|---------|--------|
| Initial Sync | Fetch all markets on startup | Write lock per page |
| Reconciliation Loop | Periodic REST sync | Read + Write lock |
| Exchange Status Loop | Poll exchange status | Write lock |
| Event Sync Loop | Sync events table | Write lock |
| Lifecycle Handler | Process WS events | Write lock |

All access to `registryState` protected by `sync.RWMutex`.

### Concurrency Safety

The `sync.RWMutex` ensures safe concurrent access:

- **Read operations** (`GetActiveMarkets`, `GetMarket`): Acquire read lock, allowing concurrent reads
- **Write operations** (sync, reconciliation, lifecycle updates): Acquire write lock, exclusive access

Since all timestamps are server-provided (no clock skew concerns), ordering is determined by Kalshi's `exchange_ts`. The mutex prevents data races but does not guarantee event ordering across goroutines—this is acceptable because each goroutine processes independent data sources.
