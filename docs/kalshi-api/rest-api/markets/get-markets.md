# Get Markets

```
GET /markets
```

**Auth**: None

## Query Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | 100 | Results per page (1-1000) |
| `cursor` | string | - | Pagination cursor |
| `event_ticker` | string | - | Filter by event (comma-sep, max 10) |
| `series_ticker` | string | - | Filter by series |
| `tickers` | string | - | Comma-separated market tickers |
| `status` | string | - | `unopened`, `open`, `paused`, `closed`, `settled` |
| `min_created_ts` | int | - | Unix timestamp filter |
| `max_created_ts` | int | - | Unix timestamp filter |
| `min_close_ts` | int | - | Unix timestamp filter |
| `max_close_ts` | int | - | Unix timestamp filter |
| `min_settled_ts` | int | - | Unix timestamp filter |
| `max_settled_ts` | int | - | Unix timestamp filter |
| `mve_filter` | string | - | `only` or `exclude` |

## Filter Constraints

| Timestamp Filters | Compatible Status |
|-------------------|-------------------|
| `min/max_created_ts` | `unopened`, `open`, or none |
| `min/max_close_ts` | `closed` or none |
| `min/max_settled_ts` | `settled` or none |

## Response

| Field | Type | Description |
|-------|------|-------------|
| `markets` | array | Market objects |
| `cursor` | string | Next page cursor |

### Market Object

| Field | Type | Description |
|-------|------|-------------|
| `ticker` | string | Market ID |
| `event_ticker` | string | Parent event |
| `market_type` | string | `binary` or `scalar` |
| `title` | string | Market title |
| `status` | string | Market status |
| `yes_bid` | int | Best YES bid (cents) |
| `yes_ask` | int | Best YES ask (cents) |
| `no_bid` | int | Best NO bid (cents) |
| `no_ask` | int | Best NO ask (cents) |
| `yes_bid_dollars` | string | Best YES bid |
| `yes_ask_dollars` | string | Best YES ask |
| `no_bid_dollars` | string | Best NO bid |
| `no_ask_dollars` | string | Best NO ask |
| `last_price` | int | Last price (cents) |
| `last_price_dollars` | string | Last price |
| `volume` | int | Total volume |
| `volume_24h` | int | 24h volume |
| `open_interest` | int | Open interest |
| `settlement_value` | int \| null | Settlement (cents) |
| `settlement_value_dollars` | string \| null | Settlement |
