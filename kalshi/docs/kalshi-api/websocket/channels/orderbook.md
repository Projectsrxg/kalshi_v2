# Orderbook Channel

Channel: `orderbook_delta`

Real-time orderbook updates. Sends initial snapshot, then incremental deltas.

## Subpenny Pricing

API responses include both cent and dollar formats. Use `*_dollars` fields for subpenny precision (4+ decimal places).

## Subscription

```json
{
  "id": 1,
  "cmd": "subscribe",
  "params": {
    "channels": ["orderbook_delta"],
    "market_ticker": "MARKET-TICKER"
  }
}
```

Market specification required (`market_ticker` or `market_tickers`).

## Messages

### Orderbook Snapshot

Sent once after subscription.

```json
{
  "type": "orderbook_snapshot",
  "sid": 1,
  "seq": 1,
  "msg": {
    "market_ticker": "MARKET-TICKER",
    "yes": [[52, 1500], [51, 3200]],
    "no": [[48, 1800], [47, 2500]],
    "yes_dollars": [["0.52", 1500], ["0.51", 3200]],
    "no_dollars": [["0.48", 1800], ["0.47", 2500]]
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | `orderbook_snapshot` |
| `sid` | int | Subscription ID |
| `seq` | int | Sequence number |
| `msg.market_ticker` | string | Market ID |
| `msg.yes` | array | `[[price_cents, qty], ...]` |
| `msg.no` | array | `[[price_cents, qty], ...]` |
| `msg.yes_dollars` | array | `[["price", qty], ...]` |
| `msg.no_dollars` | array | `[["price", qty], ...]` |

### Orderbook Delta

Sent on each orderbook change.

```json
{
  "type": "orderbook_delta",
  "sid": 1,
  "seq": 2,
  "msg": {
    "market_ticker": "MARKET-TICKER",
    "price": 52,
    "price_dollars": "0.52",
    "delta": -50,
    "side": "yes",
    "ts": 1705328200
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | `orderbook_delta` |
| `sid` | int | Subscription ID |
| `seq` | int | Sequence number |
| `msg.market_ticker` | string | Market ID |
| `msg.price` | int | Price level (cents) |
| `msg.price_dollars` | string | Price level, e.g. `"0.5250"` |
| `msg.delta` | int | Quantity change (+ add, - remove) |
| `msg.side` | string | `yes` or `no` |
| `msg.ts` | int or string | Unix timestamp (seconds) or ISO 8601 string with µs precision |
| `msg.client_order_id` | string | Present if your order caused change |

**Note:** The `ts` field may be returned as either:
- Integer: Unix timestamp in seconds (e.g., `1705328200`)
- String: ISO 8601 format with microsecond precision (e.g., `"2026-01-06T15:24:59.504579Z"`)

## Sequence Numbers

- `seq` increments per message per subscription
- Use to detect missed messages
- On gap, resubscribe for fresh snapshot
