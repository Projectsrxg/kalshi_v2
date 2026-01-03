# WebSocket Connection

## Endpoint

```
wss://api.elections.kalshi.com
```

Demo: `wss://demo-api.kalshi.co`

## Authentication

API key auth required during handshake. Include auth headers in connection request.

## Heartbeat

Server sends ping frames every 10 seconds with body `heartbeat`. Respond with pong frames.

## Command Format

```json
{
  "id": 1,
  "cmd": "subscribe",
  "params": {...}
}
```

| Field | Type | Description |
|-------|------|-------------|
| `id` | int | Request ID (increment per command) |
| `cmd` | string | Command type |
| `params` | object | Command parameters |

## Commands

### Subscribe

```json
{
  "id": 1,
  "cmd": "subscribe",
  "params": {
    "channels": ["orderbook_delta", "ticker"],
    "market_ticker": "MARKET-TICKER"
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `channels` | array | Channel names |
| `market_ticker` | string | Single market |
| `market_tickers` | array | Multiple markets |

### Unsubscribe

```json
{
  "id": 2,
  "cmd": "unsubscribe",
  "params": {
    "sids": [1, 2]
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `sids` | array | Subscription IDs to remove |

### Update Subscription

Add or remove markets from an existing subscription.

**Add markets:**
```json
{
  "id": 3,
  "cmd": "update_subscription",
  "params": {
    "sids": [1],
    "action": "add_markets",
    "market_tickers": ["MARKET-2", "MARKET-3"]
  }
}
```

**Remove markets:**
```json
{
  "id": 4,
  "cmd": "update_subscription",
  "params": {
    "sids": [1],
    "action": "delete_markets",
    "market_tickers": ["MARKET-2"]
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `sids` | array | Subscription IDs (or use `sid` for single) |
| `action` | string | `add_markets` or `delete_markets` |
| `market_tickers` | array | Markets to add/remove |

**Response:**
```json
{
  "id": 3,
  "type": "ok",
  "msg": {}
}
```

Note: The subscription ID (`sid`) remains the same after update. Sequence numbers continue from where they were.

### List Subscriptions

```json
{
  "id": 4,
  "cmd": "list_subscriptions",
  "params": {}
}
```

**Response:** (TBD - format not yet documented by Kalshi)

## Response Types

### Subscribed

```json
{
  "id": 1,
  "type": "subscribed",
  "msg": {
    "sid": 1,
    "channel": "orderbook_delta"
  }
}
```

### Unsubscribed

```json
{
  "id": 2,
  "type": "unsubscribed",
  "msg": {
    "sids": [1]
  }
}
```

### Error

```json
{
  "id": 1,
  "type": "error",
  "msg": {
    "code": "INVALID_CHANNEL",
    "message": "Unknown channel"
  }
}
```

## Go Connection Example

```go
package ws

import (
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	conn  *websocket.Conn
	msgID int64
}

type Command struct {
	ID     int64       `json:"id"`
	Cmd    string      `json:"cmd"`
	Params interface{} `json:"params"`
}

type SubscribeParams struct {
	Channels     []string `json:"channels"`
	MarketTicker string   `json:"market_ticker,omitempty"`
}

func Connect(url string, headers map[string]string) (*Client, error) {
	h := http.Header{}
	for k, v := range headers {
		h.Set(k, v)
	}

	conn, _, err := websocket.DefaultDialer.Dial(url, h)
	if err != nil {
		return nil, err
	}

	// Server sends ping frames every 10s, client responds with pong
	conn.SetPingHandler(func(data string) error {
		return conn.WriteControl(
			websocket.PongMessage,
			[]byte(data),
			time.Now().Add(time.Second),
		)
	})

	return &Client{conn: conn}, nil
}

func (c *Client) Subscribe(channels []string, ticker string) error {
	cmd := Command{
		ID:  atomic.AddInt64(&c.msgID, 1),
		Cmd: "subscribe",
		Params: SubscribeParams{
			Channels:     channels,
			MarketTicker: ticker,
		},
	}
	return c.conn.WriteJSON(cmd)
}

func (c *Client) Read() ([]byte, error) {
	_, msg, err := c.conn.ReadMessage()
	return msg, err
}

func (c *Client) Close() error {
	return c.conn.Close()
}
```
