# Get Trades

```
GET /markets/{ticker}/trades
```

**Auth**: None

## Path Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `ticker` | string | Market ticker |

## Query Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | 100 | Results per page (1-1000) |
| `cursor` | string | - | Pagination cursor |
| `min_ts` | int | - | Min trade time (Unix) |
| `max_ts` | int | - | Max trade time (Unix) |

## Response

| Field | Type | Description |
|-------|------|-------------|
| `trades` | array | Trade objects |
| `cursor` | string | Next page cursor |

### Trade Object

| Field | Type | Description |
|-------|------|-------------|
| `trade_id` | string | Trade ID |
| `ticker` | string | Market ticker |
| `count` | int | Contracts |
| `yes_price` | int | YES price (cents) |
| `no_price` | int | NO price (cents) |
| `yes_price_dollars` | string | YES price |
| `no_price_dollars` | string | NO price |
| `taker_side` | string | `yes` or `no` |
| `created_time` | string | ISO 8601 |
| `ts` | int | Unix seconds |
