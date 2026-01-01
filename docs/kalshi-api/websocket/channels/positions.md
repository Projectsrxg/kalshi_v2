# Positions Channel

Channel: `market_positions`

User position updates.

## Subscription

```json
{
  "id": 1,
  "cmd": "subscribe",
  "params": {
    "channels": ["market_positions"]
  }
}
```

Market specification optional.

**Auth**: Required

## Message

```json
{
  "type": "market_positions",
  "sid": 1,
  "msg": {
    "market_ticker": "MARKET-TICKER",
    "position": 100,
    "market_exposure": 5200,
    "market_exposure_dollars": "52.00",
    "realized_pnl": 150,
    "realized_pnl_dollars": "1.50",
    "resting_orders_count": 50,
    "fees_paid": 25,
    "fees_paid_dollars": "0.25",
    "ts": 1705328200
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | `market_positions` |
| `sid` | int | Subscription ID |
| `msg.market_ticker` | string | Market ID |
| `msg.position` | int | Contract count (+ = YES, - = NO) |
| `msg.market_exposure` | int | Position cost (cents) |
| `msg.market_exposure_dollars` | string | Position cost |
| `msg.realized_pnl` | int | Realized P&L (cents) |
| `msg.realized_pnl_dollars` | string | Realized P&L |
| `msg.resting_orders_count` | int | Resting order size |
| `msg.fees_paid` | int | Fees paid (cents) |
| `msg.fees_paid_dollars` | string | Fees paid |
| `msg.ts` | int | Unix timestamp (seconds) |

Sent when position changes (fills, settlements).
