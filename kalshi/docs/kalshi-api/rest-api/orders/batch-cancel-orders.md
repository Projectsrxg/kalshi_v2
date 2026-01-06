# Batch Cancel Orders

```
DELETE /portfolio/orders/batched
```

**Auth**: Required

## Request Body

| Field | Type | Description |
|-------|------|-------------|
| `order_ids` | array | Array of order IDs to cancel |

## Response

| Field | Type | Description |
|-------|------|-------------|
| `results` | array | Array of result objects |

### Result Object

| Field | Type | Description |
|-------|------|-------------|
| `order_id` | string | Order ID |
| `order` | object \| null | Canceled order |
| `reduced_by` | int | Contracts reduced |
| `error` | object \| null | Error if failed |

Each cancellation counts as 0.2 against write rate limit.
