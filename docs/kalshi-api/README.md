# Kalshi API Documentation

## Base URLs

| Environment | REST API | WebSocket |
|-------------|----------|-----------|
| Production | `https://api.elections.kalshi.com/trade-api/v2` | `wss://api.elections.kalshi.com/trade-api/ws/v2` |
| Demo | `https://demo-api.kalshi.co/trade-api/v2` | `wss://demo-api.kalshi.co/trade-api/ws/v2` |

---

## Pagination

Cursor-based pagination on list endpoints:

| Parameter | Type | Description |
|-----------|------|-------------|
| `limit` | integer | Results per page (default: 100) |
| `cursor` | string | Cursor from previous response |

Empty `cursor` in response indicates last page.

## Timestamps

- REST API: Unix seconds
- Auth headers: Unix milliseconds
- WebSocket `ts` fields: Unix seconds

## Price Formats

| Format | Type | Example | Description |
|--------|------|---------|-------------|
| Cents | integer | `56` | Price 1-99 |
| Dollars | string | `"0.5600"` | Fixed-point decimal |

YES price + NO price = 100 cents.

## Market Status Values

### Status Field (on market objects)

| Status | Description |
|--------|-------------|
| `initialized` | Created, not active |
| `inactive` | Temporarily inactive |
| `active` | Open for trading |
| `closed` | Awaiting settlement |
| `determined` | Outcome determined |
| `disputed` | Settlement disputed |
| `amended` | Settlement amended |
| `finalized` | Fully settled |

### Status Filter (GET /markets query parameter)

| Filter | Description |
|--------|-------------|
| `unopened` | Markets not yet open |
| `open` | Currently trading |
| `paused` | Temporarily paused |
| `closed` | Trading closed |
| `settled` | Fully settled |

## Order Status Values

| Status | Description |
|--------|-------------|
| `resting` | Active on orderbook |
| `canceled` | Canceled |
| `executed` | Fully filled |
| `pending` | Awaiting processing |

## Limits

| Limit | Value |
|-------|-------|
| Max open orders | 200,000 |
| Max batch size | 20 |
| Max event tickers per query | 10 |

---

## Authentication

### API Key Generation

Generate at https://kalshi.com/account/profile under "API Keys".

Returns:
- **Key ID**: UUID identifier
- **Private Key**: RSA private key (PEM format, shown once)

### Required Headers

| Header | Value |
|--------|-------|
| `KALSHI-ACCESS-KEY` | API Key ID |
| `KALSHI-ACCESS-TIMESTAMP` | Unix timestamp in milliseconds |
| `KALSHI-ACCESS-SIGNATURE` | Base64 RSA-PSS signature |

### Signature Generation

1. **Message**: `<timestamp><METHOD><path>` (path without query params)
2. **Sign**: RSA-PSS with SHA-256, salt length = digest length
3. **Encode**: Base64

Example message for `GET /trade-api/v2/portfolio/orders?limit=10`:
```
1703123456789GET/trade-api/v2/portfolio/orders
```

### Go Implementation

```go
package auth

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"time"
)

type Client struct {
	KeyID      string
	PrivateKey *rsa.PrivateKey
}

func NewClient(keyID, keyPath string) (*Client, error) {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not RSA key")
	}

	return &Client{KeyID: keyID, PrivateKey: rsaKey}, nil
}

func (c *Client) Sign(method, path string) (timestamp, signature string, err error) {
	ts := fmt.Sprintf("%d", time.Now().UnixMilli())

	if idx := strings.Index(path, "?"); idx != -1 {
		path = path[:idx]
	}

	msg := ts + method + path
	hash := sha256.Sum256([]byte(msg))

	sig, err := rsa.SignPSS(rand.Reader, c.PrivateKey, crypto.SHA256, hash[:],
		&rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash})
	if err != nil {
		return "", "", err
	}

	return ts, base64.StdEncoding.EncodeToString(sig), nil
}

func (c *Client) Headers(method, path string) (map[string]string, error) {
	ts, sig, err := c.Sign(method, path)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"KALSHI-ACCESS-KEY":       c.KeyID,
		"KALSHI-ACCESS-TIMESTAMP": ts,
		"KALSHI-ACCESS-SIGNATURE": sig,
	}, nil
}
```

---

## Rate Limits

### Tiers

| Tier | Read | Write |
|------|------|-------|
| Basic | 20/s | 10/s |
| Advanced | 30/s | 30/s |
| Premier | 100/s | 100/s |
| Prime | 400/s | 400/s |

### Write Operations

| Endpoint | Cost |
|----------|------|
| CreateOrder | 1 |
| CancelOrder | 1 |
| AmendOrder | 1 |
| DecreaseOrder | 1 |
| BatchCreateOrders | 1 per order |
| BatchCancelOrders | 0.2 per cancel |

### Tier Qualification

- **Basic**: Automatic
- **Advanced**: https://kalshi.typeform.com/advanced-api
- **Premier**: 3.75% monthly volume + technical review
- **Prime**: 7.5% monthly volume + technical review

### Rate Limit Response

HTTP 429:
```json
{
  "code": "RATE_LIMITED",
  "message": "Rate limit exceeded",
  "details": {"retry_after_ms": 1000}
}
```

---

## REST API

### Exchange

- [Get Status](./rest-api/exchange/get-status.md)
- [Get Announcements](./rest-api/exchange/get-announcements.md)
- [Get Schedule](./rest-api/exchange/get-schedule.md)

### Markets

- [Get Markets](./rest-api/markets/get-markets.md)
- [Get Market](./rest-api/markets/get-market.md)
- [Get Orderbook](./rest-api/markets/get-orderbook.md)
- [Get Trades](./rest-api/markets/get-trades.md)
- [Get Candlesticks](./rest-api/markets/get-candlesticks.md)
- [Batch Get Candlesticks](./rest-api/markets/batch-get-candlesticks.md)
- [Get Series](./rest-api/markets/get-series.md)
- [Get Events](./rest-api/markets/get-events.md)
- [Get Event](./rest-api/markets/get-event.md)

### Portfolio

- [Get Balance](./rest-api/portfolio/get-balance.md)
- [Get Positions](./rest-api/portfolio/get-positions.md)
- [Get Fills](./rest-api/portfolio/get-fills.md)
- [Get Settlements](./rest-api/portfolio/get-settlements.md)

### Orders

- [Get Orders](./rest-api/orders/get-orders.md)
- [Get Order](./rest-api/orders/get-order.md)
- [Create Order](./rest-api/orders/create-order.md)
- [Batch Create Orders](./rest-api/orders/batch-create-orders.md)
- [Cancel Order](./rest-api/orders/cancel-order.md)
- [Batch Cancel Orders](./rest-api/orders/batch-cancel-orders.md)
- [Amend Order](./rest-api/orders/amend-order.md)
- [Decrease Order](./rest-api/orders/decrease-order.md)

## WebSocket API

- [Connection](./websocket/connection.md)

### Channels

- [Orderbook](./websocket/channels/orderbook.md)
- [Ticker](./websocket/channels/ticker.md)
- [Trades](./websocket/channels/trades.md)
- [Fills](./websocket/channels/fills.md)
- [Positions](./websocket/channels/positions.md)
- [Market Lifecycle](./websocket/channels/market-lifecycle.md)
