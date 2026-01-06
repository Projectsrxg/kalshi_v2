# Get Exchange Status

```
GET /exchange/status
```

**Auth**: None

## Response

| Field | Type | Description |
|-------|------|-------------|
| `exchange_active` | bool | Exchange operational |
| `trading_active` | bool | Trading enabled |
| `exchange_estimated_resume_time` | string \| null | ISO 8601 maintenance end estimate |

```json
{
  "exchange_active": true,
  "trading_active": true,
  "exchange_estimated_resume_time": null
}
```
