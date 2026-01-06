# Batch Create Orders

```
POST /portfolio/orders/batched
```

**Auth**: Required

## Request Body

| Field | Type | Description |
|-------|------|-------------|
| `orders` | array | Array of order objects (max 20) |

Each order object has same fields as Create Order.

## Response (201)

| Field | Type | Description |
|-------|------|-------------|
| `results` | array | Array of result objects |

### Result Object

| Field | Type | Description |
|-------|------|-------------|
| `client_order_id` | string \| null | Client order ID |
| `order` | object \| null | Created order |
| `error` | object \| null | Error if failed |

```json
{
  "results": [
    {
      "client_order_id": "my-order-1",
      "order": {...},
      "error": null
    },
    {
      "client_order_id": "my-order-2",
      "order": null,
      "error": {
        "code": "INSUFFICIENT_BALANCE",
        "message": "Insufficient balance"
      }
    }
  ]
}
```

Each order counts as 1 against write rate limit.
