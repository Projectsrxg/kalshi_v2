# Decrease Order

```
POST /portfolio/orders/{order_id}/decrease
```

**Auth**: Required

## Path Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `order_id` | string | Order ID |

## Request Body

| Field | Type | Description |
|-------|------|-------------|
| `reduce_by` | int | Contracts to reduce |

## Response

| Field | Type | Description |
|-------|------|-------------|
| `order` | object | Updated order object |
| `reduced_by` | int | Actual contracts reduced |

```json
{
  "order": {
    "order_id": "abc123",
    "remaining_count": 50
  },
  "reduced_by": 50
}
```
