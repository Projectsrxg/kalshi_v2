# Market Registry

Discovers and tracks all Kalshi markets. Entry point for the gatherer's data collection pipeline.

---

## Responsibilities

| Responsibility | Details |
|----------------|---------|
| Exchange status | Check `GET /exchange/status` before connecting |
| Initial sync | Fetch all markets/events via REST on startup |
| Live updates | Subscribe to `market_lifecycle` WebSocket channel |
| Reconciliation | Periodic REST poll as backup |
| Metadata storage | Write to local `markets`, `events`, `series` tables |
| Subscription control | Tell Connection Manager which markets to subscribe |

---

## Deployment Model

Market Registry runs as a goroutine within a single Go binary alongside Connection Manager, Message Router, and Writers. All components communicate via Go channels and shared memory.

```mermaid
flowchart TD
    subgraph Container["Gatherer (Docker Container)"]
        subgraph Binary["Go Binary"]
            MR[Market Registry<br/>goroutine]
            CM[Connection Manager<br/>goroutine]
            RT[Message Router<br/>goroutine]
            WR[Writers<br/>goroutines]
        end
    end

    MR <-->|Go channels| CM
    CM <-->|Go channels| RT
    RT <-->|Go channels| WR
```

---

## Dependencies

```mermaid
flowchart LR
    MR[Market Registry] --> REST[REST Client]
    MR --> DB[(PostgreSQL)]
    MR --> CM[Connection Manager]
    WS[WebSocket Pool] -->|market_lifecycle| MR
```

| Dependency | Purpose |
|------------|---------|
| REST Client | Fetch markets, events, series, exchange status |
| PostgreSQL | Persist market/event/series metadata |
| Connection Manager | Receives subscription instructions via channel |
| WebSocket Pool | Receives `market_lifecycle` messages |

---

## Data Sources

### REST Endpoints

| Endpoint | Purpose | Frequency |
|----------|---------|-----------|
| `GET /exchange/status` | Exchange operational status | On startup, every 1 min |
| `GET /markets` | All markets (paginated) | On startup, reconciliation |
| `GET /markets/{ticker}` | Single market detail | On `created` lifecycle event |
| `GET /events` | All events (paginated) | On startup, every 10 min |
| `GET /series/{ticker}` | Series metadata | On demand (when new series seen) |

### WebSocket Channel

| Channel | Events | Action |
|---------|--------|--------|
| `market_lifecycle` | `created` | Fetch full market via REST, notify CM to subscribe |
| `market_lifecycle` | `status_change` | Update cache + DB, notify CM if status changed |
| `market_lifecycle` | `settled` | Update cache + DB, notify CM to unsubscribe |

---

## Related Docs

- [Interface](./interface.md) - Public methods and types
- [Lifecycle](./lifecycle.md) - Startup and shutdown sequences
- [Behaviors](./behaviors.md) - Event handling, reconciliation
- [Configuration](./configuration.md) - Config options
