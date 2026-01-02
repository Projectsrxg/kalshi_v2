# WebSocket Client

Low-level WebSocket client for connecting to Kalshi's WebSocket API. Used by Connection Manager.

---

## Responsibilities

| Responsibility | Details |
|----------------|---------|
| Connection | Establish WebSocket connection with auth headers |
| Heartbeat | Respond to server pings (every 10s) |
| Commands | Send subscribe/unsubscribe/update commands |
| Message reading | Read raw messages, send to channel |
| Connection state | Track connection health |

**Not responsible for** (handled by Connection Manager):
- Deciding which markets to subscribe to
- Managing multiple connections
- Reconnection policy and backoff
- Parsing message bodies
- Tracking ticker-to-sid mappings

---

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Message parsing | Minimal (raw bytes) | Let Message Router handle parsing |
| Authentication | Headers during handshake | Kalshi's documented approach |
| Subscribe response | Wait for confirmation | Need subscription IDs for unsubscribe |

---

## Related Docs

- [Interface](./interface.md) - Public methods and types
- [Lifecycle](./lifecycle.md) - Connection states and sequences
- [Behaviors](./behaviors.md) - Read loop, heartbeat, subscribe/unsubscribe
- [Configuration](./configuration.md) - Config options and metrics
- [Kalshi WebSocket API](../../kalshi-api/websocket/connection.md) - Protocol reference
- [Connection Manager](../connection-manager/) - Manages pool of clients
