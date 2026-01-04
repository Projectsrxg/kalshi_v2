# Connection Manager

Orchestrates WebSocket connections and subscriptions. Bridges Market Registry (which discovers markets) with WebSocket Clients (which handle raw connections).

---

## Responsibilities

| Responsibility | Details |
|----------------|---------|
| Connection pool | Manage 150 WebSocket Client instances |
| Subscription orchestration | Subscribe/unsubscribe based on Market Registry events |
| Command/response correlation | Track pending commands, match responses |
| Reconnection | Reconnect with exponential backoff on errors |
| Message forwarding | Forward raw messages to Message Router |
| Sequence tracking | Track sequence numbers per subscription, detect gaps |

**Not responsible for** (handled by other components):
- Raw WebSocket I/O (WebSocket Client)
- Market discovery (Market Registry)
- Message parsing (Message Router)

**Important:** Connection Manager does not track markets independently. It receives `MarketChange` events from Market Registry and subscribes/unsubscribes accordingly. See [Market Registry: Single Source of Truth](../market-registry/README.md#single-source-of-truth).

---

## Connection Allocation

**150 connections per gatherer (fixed):**

| Connections | Channel | Count | Purpose |
|-------------|---------|-------|---------|
| 1-2 | `ticker` | 2 | Global ticker subscription (redundancy) |
| 3-4 | `trade` | 2 | Global trade subscription (redundancy) |
| 5-6 | `market_lifecycle` | 2 | Market lifecycle events → Market Registry |
| 7-150 | `orderbook_delta` | 144 | Per-market orderbook subscriptions |

Each gatherer has its own Kalshi API key.

```mermaid
flowchart LR
    subgraph Ticker[Ticker 1-2]
        C1[Conn 1]
        C2[Conn 2]
    end

    subgraph Trade[Trade 3-4]
        C3[Conn 3]
        C4[Conn 4]
    end

    subgraph Lifecycle[Lifecycle 5-6]
        C5[Conn 5]
        C6[Conn 6]
    end

    subgraph Orderbook[Orderbook 7-150]
        C7[Conn 7]
        C150[Conn 150]
    end

    C1 & C2 -->|all markets| TK[Ticker Data]
    C3 & C4 -->|all markets| TR[Trade Data]
    C5 & C6 -->|all markets| LC[Lifecycle Events]
    C7 & C150 -->|per-market| OB[Orderbook Data]
```

### Connection ID Assignment

Connection IDs are assigned at startup and remain fixed:

```go
type ConnectionManager struct {
    connections map[int]*WebSocketClient
}

func NewConnectionManager(cfg Config) *ConnectionManager {
    cm := &ConnectionManager{
        connections: make(map[int]*WebSocketClient, 150),
    }

    // Reserved connections (fixed assignment)
    cm.connections[1] = newClient("ticker-1")     // Ticker (primary)
    cm.connections[2] = newClient("ticker-2")     // Ticker (backup)
    cm.connections[3] = newClient("trade-1")      // Trade (primary)
    cm.connections[4] = newClient("trade-2")      // Trade (backup)
    cm.connections[5] = newClient("lifecycle-1")  // Lifecycle (primary)
    cm.connections[6] = newClient("lifecycle-2")  // Lifecycle (backup)

    // Orderbook connections (dynamic assignment per market)
    for i := 7; i <= 150; i++ {
        cm.connections[i] = newClient(fmt.Sprintf("orderbook-%d", i))
    }

    return cm
}
```

### Redundancy Strategy

| Channel | Connections | Strategy | Failure Handling |
|---------|-------------|----------|------------------|
| `ticker` | 2 | Both subscribe to all markets | If 1 fails, other has full data |
| `trade` | 2 | Both subscribe to all trades | If 1 fails, other has full data |
| `market_lifecycle` | 2 | Both subscribe to all markets | If 1 fails, other has full data |
| `orderbook_delta` | 144 | Each market assigned to 1 connection | Reconnect and re-subscribe |

**Orderbook capacity:**
- 144 connections × 7,500 markets/connection = **1,080,000 markets max per gatherer**
- Current active markets fluctuate between 200K-600K, well within capacity

---

## Subscription Model

| Channel | Market Param | Strategy |
|---------|--------------|----------|
| `ticker` | Omit (= all markets) | 1 global subscription per connection |
| `trade` | Omit (= all trades) | 1 global subscription per connection |
| `market_lifecycle` | Omit (= all markets) | 1 global subscription per connection |
| `orderbook_delta` | Required | 1 subscription per market, least-loaded assignment |

---

## Dependencies

```mermaid
flowchart LR
    MR[Market Registry] -->|MarketChange events| CM[Connection Manager]
    CM -->|Send commands| WS[WebSocket Clients]
    WS -->|Raw messages| CM
    CM -->|RawMessage| ROUTER[Message Router]
    CM -->|market_lifecycle| MR
```

| Dependency | Direction | Purpose |
|------------|-----------|---------|
| Market Registry | Bidirectional | Receives `MarketChange` events, sends `market_lifecycle` messages |
| WebSocket Client | Bidirectional | Sends commands, receives messages |
| Message Router | Output | Forwards raw messages (ticker, trade, orderbook) |

---

## Related Docs

- [Interface](./interface.md) - Public methods and types
- [Lifecycle](./lifecycle.md) - Startup, shutdown, reconnection
- [Behaviors](./behaviors.md) - Event handling, command correlation, sequence tracking
- [Configuration](./configuration.md) - Config options and metrics
- [WebSocket Client](../websocket/) - Low-level connection handling
- [Market Registry](../market-registry/) - Market discovery
