# WebSocket Client

Low-level WebSocket client for connecting to Kalshi's WebSocket API. Used by Connection Manager.

---

## Responsibilities

| Responsibility | Details |
|----------------|---------|
| Connection | Establish WebSocket connection with auth headers |
| Heartbeat | Respond to server pings (every 10s) |
| Send | Write raw bytes to connection |
| Message reading | Read raw messages, send to channel |
| Connection state | Track connection health |

**Not responsible for** (handled by Connection Manager):
- Deciding which markets to subscribe to
- Managing multiple connections
- Reconnection policy and backoff
- Parsing message bodies
- Command/response correlation
- Subscribe/unsubscribe semantics
- Tracking subscription IDs
- Sequence number tracking and gap detection
- Resubscription on sequence gaps

---

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Message handling | Raw bytes only | No parsing, all messages to one channel |
| Authentication | Headers during handshake | Kalshi's documented approach |
| Command sending | Raw bytes via `Send()` | Connection Manager builds commands |

---

## Related Docs

- [Interface](./interface.md) - Public methods and types
- [Lifecycle](./lifecycle.md) - Connection states and sequences
- [Behaviors](./behaviors.md) - Read loop, heartbeat, send
- [Configuration](./configuration.md) - Config options and metrics
- [Kalshi WebSocket API](../../kalshi-api/websocket/connection.md) - Protocol reference
- [Connection Manager](../connection-manager/) - Manages pool of clients, handles subscriptions
