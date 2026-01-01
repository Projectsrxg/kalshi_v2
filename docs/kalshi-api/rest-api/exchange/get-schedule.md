# Get Exchange Schedule

```
GET /exchange/schedule
```

**Auth**: None

## Response

| Field | Type | Description |
|-------|------|-------------|
| `schedule` | object | Schedule configuration |
| `schedule.standard_hours` | object | Regular trading hours by day |
| `schedule.closures` | array | Scheduled closure periods |

```json
{
  "schedule": {
    "standard_hours": {
      "timezone": "America/New_York",
      "days": {
        "monday": {"open": "00:00", "close": "23:59"},
        "tuesday": {"open": "00:00", "close": "23:59"},
        "wednesday": {"open": "00:00", "close": "23:59"},
        "thursday": {"open": "00:00", "close": "23:59"},
        "friday": {"open": "00:00", "close": "23:59"},
        "saturday": {"open": "00:00", "close": "23:59"},
        "sunday": {"open": "00:00", "close": "23:59"}
      }
    },
    "closures": []
  }
}
```
