# Get Settlements

```
GET /portfolio/settlements
```

**Auth**: Required

## Query Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `cursor` | string | - | Pagination cursor |
| `limit` | int | 100 | Results per page (1-1000) |
| `ticker` | string | - | Filter by market ticker |
| `min_ts` | int | - | Min settlement time (Unix) |
| `max_ts` | int | - | Max settlement time (Unix) |

## Response

| Field | Type | Description |
|-------|------|-------------|
| `settlements` | array | Settlement objects |
| `cursor` | string | Next page cursor |

### Settlement Object

| Field | Type | Description |
|-------|------|-------------|
| `ticker` | string | Market ticker |
| `market_result` | string | `yes` or `no` |
| `position` | int | Final position |
| `revenue` | int | Settlement revenue (cents) |
| `revenue_dollars` | string | Settlement revenue |
| `settled_time` | string | ISO 8601 |
| `ts` | int | Unix seconds |
