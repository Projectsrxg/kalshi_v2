# Get Events

```
GET /events
```

**Auth**: None

## Query Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | 100 | Results per page |
| `cursor` | string | - | Pagination cursor |
| `series_ticker` | string | - | Filter by series |
| `status` | string | - | Filter by status |

## Response

| Field | Type | Description |
|-------|------|-------------|
| `events` | array | Event objects |
| `cursor` | string | Next page cursor |

### Event Object

| Field | Type | Description |
|-------|------|-------------|
| `event_ticker` | string | Event ID |
| `series_ticker` | string | Parent series |
| `title` | string | Event title |
| `subtitle` | string | Subtitle |
| `category` | string | Category |
| `mutually_exclusive` | bool | Markets mutually exclusive |
| `markets` | array | Associated market tickers |
| `strike_date` | string | Strike date |
| `status` | string | Event status |
