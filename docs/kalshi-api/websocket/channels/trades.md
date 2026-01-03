# Trades Channel

Channel: `trade`

Public trade notifications.

## Subpenny Pricing

API responses include both cent and dollar formats. Use `*_dollars` fields for subpenny precision (4+ decimal places).

## Subscription

```json
{
  "id": 1,
  "cmd": "subscribe",
  "params": {
    "channels": ["trade"],
    "market_ticker": "MARKET-TICKER"
  }
}
```

Market specification optional. Omit to receive all trades.

**Auth**: Not required

## Message

```json
{
  "type": "trade",
  "sid": 1,
  "seq": 5,
  "msg": {
    "market_ticker": "MARKET-TICKER",
    "trade_id": "trd_abc123",
    "count": 100,
    "yes_price": 52,
    "no_price": 48,
    "yes_price_dollars": "0.52",
    "no_price_dollars": "0.48",
    "taker_side": "yes",
    "ts": 1705328200
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | `trade` |
| `sid` | int | Subscription ID |
| `seq` | int | Sequence number (per subscription) |
| `msg.market_ticker` | string | Market ID |
| `msg.trade_id` | string | Trade ID |
| `msg.count` | int | Contracts |
| `msg.yes_price` | int | YES price (cents) |
| `msg.no_price` | int | NO price (cents) |
| `msg.yes_price_dollars` | string | YES price, e.g. `"0.5250"` |
| `msg.no_price_dollars` | string | NO price, e.g. `"0.4750"` |
| `msg.taker_side` | string | `yes` or `no` |
| `msg.ts` | int | Unix timestamp (seconds) |

Sent immediately after trade execution.

## Sequence Numbers

- `seq` increments per message per subscription
- Use to detect missed messages
- On gap, resubscribe to ensure no trades were missed
- Note: `trade_id` provides deduplication, but sequence gaps indicate potential missed trades
