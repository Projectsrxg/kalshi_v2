# Ticker Channel

Channel: `ticker`

Market price, volume, and open interest updates.

## Subpenny Pricing

API responses include both cent and dollar formats. Use `*_dollars` fields for subpenny precision (4+ decimal places).

## Subscription

```json
{
  "id": 1,
  "cmd": "subscribe",
  "params": {
    "channels": ["ticker"],
    "market_ticker": "MARKET-TICKER"
  }
}
```

Market specification optional. Omit to receive all markets.

## Message

```json
{
  "type": "ticker",
  "sid": 1,
  "msg": {
    "market_ticker": "MARKET-TICKER",
    "price": 52,
    "price_dollars": "0.52",
    "yes_bid": 51,
    "yes_ask": 53,
    "yes_bid_dollars": "0.51",
    "yes_ask_dollars": "0.53",
    "no_bid_dollars": "0.47",
    "volume": 125000,
    "open_interest": 32000,
    "dollar_volume": 65000,
    "dollar_open_interest": 16000,
    "ts": 1705328200
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | `ticker` |
| `sid` | int | Subscription ID |
| `msg.market_ticker` | string | Market ID |
| `msg.price` | int | Last price (cents) |
| `msg.price_dollars` | string | Last price, e.g. `"0.5250"` |
| `msg.yes_bid` | int | Best YES bid (cents) |
| `msg.yes_ask` | int | Best YES ask (cents) |
| `msg.yes_bid_dollars` | string | Best YES bid |
| `msg.yes_ask_dollars` | string | Best YES ask |
| `msg.no_bid_dollars` | string | Best NO bid |
| `msg.volume` | int | Contracts traded |
| `msg.open_interest` | int | Open contracts |
| `msg.dollar_volume` | int | Dollar volume |
| `msg.dollar_open_interest` | int | Dollar open interest |
| `msg.ts` | int | Unix timestamp (seconds) |

Updates sent when any field changes.
