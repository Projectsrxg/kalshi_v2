# Connection Package

Connection Manager - maintains WebSocket connection pool.

## Connection Layout

Each gatherer maintains 150 WebSocket connections:

| Type | Count | Markets per Connection |
|------|-------|------------------------|
| Orderbook | 144 | 250 markets each |
| Global (trades, tickers, lifecycle) | 6 | All markets |

## Features

- Automatic reconnection with exponential backoff
- Connection health monitoring
- Dynamic market subscription updates
- Message routing to Message Router

## Usage

```go
mgr := connection.NewManager(cfg.Connections, apiClient)
mgr.OnMessage(func(msg *Message) {
    router.Route(msg)
})
mgr.Start(ctx)
```
