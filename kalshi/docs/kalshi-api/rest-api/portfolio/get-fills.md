# Get Fills

```
GET /portfolio/fills
```

**Auth**: Required

## Query Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `cursor` | string | - | Pagination cursor |
| `limit` | int | 100 | Results per page (1-1000) |
| `ticker` | string | - | Filter by market ticker |
| `order_id` | string | - | Filter by order ID |
| `min_ts` | int | - | Min fill time (Unix) |
| `max_ts` | int | - | Max fill time (Unix) |

## Response

| Field | Type | Description |
|-------|------|-------------|
| `fills` | array | Fill objects |
| `cursor` | string | Next page cursor |

### Fill Object

| Field | Type | Description |
|-------|------|-------------|
| `trade_id` | string | Trade ID |
| `order_id` | string | Order ID |
| `ticker` | string | Market ticker |
| `side` | string | `yes` or `no` |
| `action` | string | `buy` or `sell` |
| `count` | int | Contracts filled |
| `yes_price` | int | YES price (cents) |
| `no_price` | int | NO price (cents) |
| `yes_price_dollars` | string | YES price |
| `no_price_dollars` | string | NO price |
| `is_taker` | bool | Was taker side |
| `created_time` | string | ISO 8601 |
| `ts` | int | Unix seconds |
