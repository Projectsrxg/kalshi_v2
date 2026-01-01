# Lifecycle

Startup and shutdown sequences for Market Registry.

---

## Startup Sequence

`Start()` returns immediately and syncs in the background. This allows Connection Manager to begin subscribing to markets as they are discovered, rather than waiting for the full sync to complete.

```mermaid
sequenceDiagram
    participant Main
    participant MR as Market Registry
    participant REST as Kalshi REST
    participant DB as PostgreSQL
    participant CM as Connection Manager

    Main->>MR: Start(ctx)
    MR-->>Main: returns immediately

    par Background sync
        MR->>REST: GET /exchange/status
        alt exchange_active: false
            MR->>MR: Wait for estimated_resume_time
            MR->>REST: Retry GET /exchange/status
        end

        MR->>REST: GET /series (paginated)
        MR->>DB: UPSERT series

        MR->>REST: GET /events (paginated)
        MR->>DB: UPSERT events

        loop Each page of markets
            MR->>REST: GET /markets (page N)
            REST-->>MR: 1000 markets
            MR->>DB: UPSERT markets
            MR->>MR: Add to cache
            MR->>CM: MarketChange events for active markets
        end

        MR->>MR: Start reconciliation loop
        MR->>MR: Start exchange status loop
    end
```

**Design Decision**: Non-blocking startup. Connection Manager can subscribe to markets incrementally as each page is fetched, rather than waiting 5-10 minutes for full sync.

---

## Shutdown Sequence

```mermaid
sequenceDiagram
    participant Main
    participant MR as Market Registry
    participant CM as Connection Manager

    Main->>MR: Stop(ctx)
    MR->>MR: Cancel context (stops all loops)
    MR->>MR: Close change channel
    MR-->>Main: returns
```

---

## Background Loops

After initial sync completes, these loops run continuously:

| Loop | Interval | Purpose |
|------|----------|---------|
| Exchange Status | 1 min | Check if exchange is operational |
| Reconciliation | 5 min | Fetch all markets, detect missed changes |
| Event Sync | 10 min | Sync events table |

All loops respect context cancellation for graceful shutdown.
