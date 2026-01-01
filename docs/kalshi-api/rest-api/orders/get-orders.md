# Get Orders

```
GET /portfolio/orders
```

**Auth**: Required

## Query Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `ticker` | string | - | Filter by market ticker |
| `event_ticker` | string | - | Filter by event (comma-sep, max 10) |
| `min_ts` | int | - | Min order time (Unix) |
| `max_ts` | int | - | Max order time (Unix) |
| `status` | string | - | Filter by status |
| `limit` | int | 100 | Results per page (max 200) |
| `cursor` | string | - | Pagination cursor |

## Response

| Field | Type | Description |
|-------|------|-------------|
| `orders` | array | Order objects |
| `cursor` | string | Next page cursor |

### Order Object

| Field | Type | Description |
|-------|------|-------------|
| `order_id` | string | Order ID |
| `user_id` | string | User ID |
| `client_order_id` | string | Client order ID |
| `ticker` | string | Market ticker |
| `side` | string | `yes` or `no` |
| `action` | string | `buy` or `sell` |
| `type` | string | `limit` or `market` |
| `status` | string | `resting`, `canceled`, `executed`, `pending` |
| `yes_price` | int | YES price (cents) |
| `no_price` | int | NO price (cents) |
| `yes_price_dollars` | string | YES price |
| `no_price_dollars` | string | NO price |
| `initial_count` | int | Initial contracts |
| `remaining_count` | int | Remaining contracts |
| `fill_count` | int | Filled contracts |
| `taker_fees` | int | Taker fees (cents) |
| `maker_fees` | int | Maker fees (cents) |
| `taker_fill_cost` | int | Taker cost (cents) |
| `maker_fill_cost` | int | Maker cost (cents) |
| `queue_position` | int | Queue position |
| `created_time` | string | ISO 8601 |
| `last_update_time` | string | ISO 8601 |
| `expiration_time` | string \| null | ISO 8601 |
| `cancel_order_on_pause` | bool | Cancel on pause |
