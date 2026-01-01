# Amend Order

```
POST /portfolio/orders/{order_id}/amend
```

**Auth**: Required

## Path Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `order_id` | string | Order ID |

## Request Body

### Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `ticker` | string | Market ticker |
| `side` | string | `yes` or `no` |
| `action` | string | `buy` or `sell` |
| `client_order_id` | string | Original client order ID |
| `updated_client_order_id` | string | New client order ID |

### Optional Fields (at least one required)

| Field | Type | Description |
|-------|------|-------------|
| `yes_price` | int | New YES price (cents) |
| `no_price` | int | New NO price (cents) |
| `yes_price_dollars` | string | New YES price |
| `no_price_dollars` | string | New NO price |
| `count` | int | New quantity |

Max fillable = `remaining_count` + `fill_count` from original order.

## Response

| Field | Type | Description |
|-------|------|-------------|
| `old_order` | object | Previous order state |
| `order` | object | Amended order state |
