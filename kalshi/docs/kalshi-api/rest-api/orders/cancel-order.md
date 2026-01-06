# Cancel Order

```
DELETE /portfolio/orders/{order_id}
```

**Auth**: Required

## Path Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `order_id` | string | Order ID |

## Response

| Field | Type | Description |
|-------|------|-------------|
| `order` | object | Canceled order object |
| `reduced_by` | int | Contracts reduced |

```json
{
  "order": {
    "order_id": "abc123",
    "status": "canceled",
    "remaining_count": 0,
    "fill_count": 50
  },
  "reduced_by": 50
}
```
