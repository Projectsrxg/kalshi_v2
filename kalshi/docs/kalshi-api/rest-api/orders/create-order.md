# Create Order

```
POST /portfolio/orders
```

**Auth**: Required

## Request Body

### Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `ticker` | string | Market ticker |
| `side` | string | `yes` or `no` |
| `action` | string | `buy` or `sell` |
| `count` | int | Contracts (min 1) |

### Optional Fields

| Field | Type | Description |
|-------|------|-------------|
| `client_order_id` | string | Custom order ID |
| `type` | string | `limit` or `market` (default: `limit`) |
| `yes_price` | int | YES price 1-99 (cents) |
| `no_price` | int | NO price 1-99 (cents) |
| `yes_price_dollars` | string | YES price (e.g., `"0.56"`) |
| `no_price_dollars` | string | NO price |
| `expiration_ts` | int64 | Expiration timestamp |
| `time_in_force` | string | `fill_or_kill`, `good_till_canceled`, `immediate_or_cancel` |
| `buy_max_cost` | int | Max cost (cents), enables FoK |
| `post_only` | bool | Maker only |
| `reduce_only` | bool | Position reduction only |
| `self_trade_prevention_type` | string | `taker_at_cross` or `maker` |
| `order_group_id` | string | Group ID |
| `cancel_order_on_pause` | bool | Cancel on exchange pause |

## Response (201)

| Field | Type | Description |
|-------|------|-------------|
| `order` | object | Created order object |

```json
{
  "order": {
    "order_id": "abc123",
    "ticker": "MARKET-TICKER",
    "side": "yes",
    "action": "buy",
    "type": "limit",
    "status": "resting",
    "yes_price": 56,
    "initial_count": 100,
    "remaining_count": 100,
    "fill_count": 0
  }
}
```
