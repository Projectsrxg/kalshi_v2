# Get Event

```
GET /events/{event_ticker}
```

**Auth**: None

## Path Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `event_ticker` | string | Event ticker |

## Response

| Field | Type | Description |
|-------|------|-------------|
| `event` | object | Event object |

### Event Object

| Field | Type | Description |
|-------|------|-------------|
| `event_ticker` | string | Event ID |
| `series_ticker` | string | Parent series |
| `title` | string | Event title |
| `subtitle` | string | Subtitle |
| `category` | string | Category |
| `mutually_exclusive` | bool | Markets mutually exclusive |
| `markets` | array | Full market objects |
| `strike_date` | string | Strike date |
| `status` | string | Event status |
