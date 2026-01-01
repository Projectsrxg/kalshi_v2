# Get Series

```
GET /series/{series_ticker}
```

**Auth**: None

## Path Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `series_ticker` | string | Series ticker |

## Response

| Field | Type | Description |
|-------|------|-------------|
| `series` | object | Series object |

### Series Object

| Field | Type | Description |
|-------|------|-------------|
| `ticker` | string | Series ticker |
| `title` | string | Series title |
| `category` | string | Category |
| `frequency` | string | Event frequency |
| `tags` | array | Associated tags |
| `settlement_sources` | array | Data sources for settlement |
| `contract_url` | string | Contract details URL |
