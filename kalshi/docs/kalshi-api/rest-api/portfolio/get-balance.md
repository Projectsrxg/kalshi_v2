# Get Balance

```
GET /portfolio/balance
```

**Auth**: Required

## Response

| Field | Type | Description |
|-------|------|-------------|
| `balance` | int64 | Available balance (cents) |
| `portfolio_value` | int64 | Portfolio value (cents) |
| `updated_ts` | int64 | Last update timestamp (Unix) |

```json
{
  "balance": 50000,
  "portfolio_value": 25000,
  "updated_ts": 1705328200
}
```
