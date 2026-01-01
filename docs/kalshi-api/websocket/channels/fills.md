# Fills Channel

Channel: `fill`

User fill notifications.

## Subscription

```json
{
  "id": 1,
  "cmd": "subscribe",
  "params": {
    "channels": ["fill"]
  }
}
```

Market specification ignored. Always receives all user fills.

**Auth**: Required

## Message

```json
{
  "type": "fill",
  "sid": 1,
  "msg": {
    "trade_id": "trd_abc123",
    "order_id": "ord_xyz789",
    "market_ticker": "MARKET-TICKER",
    "side": "yes",
    "action": "buy",
    "count": 50,
    "yes_price": 52,
    "no_price": 48,
    "yes_price_dollars": "0.52",
    "no_price_dollars": "0.48",
    "is_taker": true,
    "ts": 1705328200
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | `fill` |
| `sid` | int | Subscription ID |
| `msg.trade_id` | string | Trade ID |
| `msg.order_id` | string | Your order ID |
| `msg.market_ticker` | string | Market ID |
| `msg.side` | string | `yes` or `no` |
| `msg.action` | string | `buy` or `sell` |
| `msg.count` | int | Contracts filled |
| `msg.yes_price` | int | YES price (cents) |
| `msg.no_price` | int | NO price (cents) |
| `msg.yes_price_dollars` | string | YES price |
| `msg.no_price_dollars` | string | NO price |
| `msg.is_taker` | bool | Was taker side |
| `msg.ts` | int | Unix timestamp (seconds) |

Sent immediately when your orders fill.
