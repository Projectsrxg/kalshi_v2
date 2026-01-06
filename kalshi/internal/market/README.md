# Market Package

Market Registry - discovers markets and tracks lifecycle.

## Responsibilities

1. Discover all markets via REST API on startup
2. Subscribe to `market_lifecycle` WebSocket channel
3. Maintain in-memory registry of active markets
4. Notify Connection Manager of market changes

## Market States

- `open` - Trading active
- `closed` - Trading halted
- `settled` - Final outcome determined

## Usage

```go
registry := market.NewRegistry(apiClient, db)
registry.OnMarketAdded(func(m *Market) {
    connectionManager.Subscribe(m.Ticker)
})
registry.Start(ctx)
```
