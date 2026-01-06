# Get Market Candlesticks

```
GET /series/{series_ticker}/markets/{ticker}/candlesticks
```

**Auth**: None

## Path Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `series_ticker` | string | Series ticker |
| `ticker` | string | Market ticker |

## Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `period_interval` | int | Yes | `1` (1m), `60` (1h), `1440` (1d) |
| `start_ts` | int | No | Start timestamp (Unix) |
| `end_ts` | int | No | End timestamp (Unix) |
| `limit` | int | No | Max candlesticks (default 100) |

## Response

| Field | Type | Description |
|-------|------|-------------|
| `candlesticks` | array | Candlestick objects |
| `cursor` | string | Next page cursor |

### Candlestick Object

| Field | Type | Description |
|-------|------|-------------|
| `ticker` | string | Market ticker |
| `period_interval` | int | Period minutes |
| `open_ts` | int | Open timestamp |
| `close_ts` | int | Close timestamp |
| `open` | int | Open price (cents) |
| `high` | int | High price (cents) |
| `low` | int | Low price (cents) |
| `close` | int | Close price (cents) |
| `open_dollars` | string | Open price |
| `high_dollars` | string | High price |
| `low_dollars` | string | Low price |
| `close_dollars` | string | Close price |
| `volume` | int | Period volume |
| `open_interest` | int | Period end OI |
