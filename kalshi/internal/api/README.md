# API Package

Kalshi API client for REST and WebSocket communication.

## Endpoints

| Environment | REST | WebSocket |
|-------------|------|-----------|
| Production | `https://api.elections.kalshi.com/trade-api/v2` | `wss://api.elections.kalshi.com` |
| Demo | `https://demo-api.kalshi.co/trade-api/v2` | `wss://demo-api.kalshi.co` |

## Components

### REST Client

HTTP client for Kalshi's REST API. Used for:
- Market discovery (`GetMarkets`, `GetEvents`)
- Orderbook snapshots (`GetOrderbook`)
- Historical trades (`GetTrades`)

### WebSocket Client

Low-level WebSocket client for real-time data. Handles:
- Connection with auth headers
- Heartbeat (ping/pong)
- Raw message reading
- Connection state tracking

> **Design docs:** See `docs/kalshi-data/websocket/` for detailed interface and behavior specifications.

Used by Connection Manager (`internal/connection/`) which manages the pool of 150 connections.

## WebSocket Channels

- `orderbook_delta` - Real-time orderbook changes
- `trade` - Executed trades
- `ticker` - Price/volume updates
- `market_lifecycle` - Market state changes

## Usage

```go
// REST client
client := api.NewRESTClient(cfg.API)
markets, err := client.GetMarkets(ctx)

// WebSocket client
ws := api.NewWebSocketClient(cfg.API)
ws.Subscribe("orderbook_delta", tickers)
```
