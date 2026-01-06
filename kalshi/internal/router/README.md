# Router Package

Message Router - routes WebSocket messages to appropriate writers.

## Message Types

| Channel | Writer |
|---------|--------|
| `orderbook_delta` | Orderbook Delta Writer |
| `trade` | Trade Writer |
| `ticker` | Ticker Writer |
| `market_lifecycle` | Market Writer |

## Features

- Non-blocking buffered channels
- Buffer overflow handling (drops oldest)
- Per-channel metrics
- Fan-out support

## Usage

```go
r := router.New(cfg.Router, writers)
r.Route(&Message{
    Channel: "orderbook_delta",
    Data:    deltaPayload,
})
```
