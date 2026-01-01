# Get Market

```
GET /markets/{ticker}
```

**Auth**: None

## Path Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `ticker` | string | Market ticker |

## Response

| Field | Type | Description |
|-------|------|-------------|
| **Core** | | |
| `ticker` | string | Market ID |
| `event_ticker` | string | Parent event |
| `market_type` | string | `binary` or `scalar` |
| `title` | string | Full title |
| `subtitle` | string | Subtitle |
| `yes_sub_title` | string | YES label |
| `no_sub_title` | string | NO label |
| **Timing** | | |
| `created_time` | string | ISO 8601 |
| `open_time` | string | Trading start |
| `close_time` | string | Trading end |
| `expiration_time` | string | Expiration |
| `latest_expiration_time` | string | Max expiration |
| `expected_expiration_time` | string \| null | Expected expiration |
| `settlement_timer_seconds` | int | Settlement delay |
| `settlement_ts` | int \| null | Settlement timestamp |
| **Pricing** | | |
| `yes_bid` | int | Best YES bid (cents) |
| `yes_ask` | int | Best YES ask (cents) |
| `no_bid` | int | Best NO bid (cents) |
| `no_ask` | int | Best NO ask (cents) |
| `yes_bid_dollars` | string | Best YES bid |
| `yes_ask_dollars` | string | Best YES ask |
| `no_bid_dollars` | string | Best NO bid |
| `no_ask_dollars` | string | Best NO ask |
| `last_price` | int | Last YES price (cents) |
| `last_price_dollars` | string | Last YES price |
| `previous_yes_bid` | int | 24h ago YES bid |
| `previous_yes_ask` | int | 24h ago YES ask |
| `previous_price` | int | 24h ago price |
| **Volume** | | |
| `volume` | int | Total volume |
| `volume_24h` | int | 24h volume |
| `open_interest` | int | Open interest |
| `liquidity` | int | Liquidity (cents) |
| `liquidity_dollars` | string | Liquidity |
| `notional_value` | int | Notional (cents) |
| `notional_value_dollars` | string | Notional |
| **Status** | | |
| `status` | string | `initialized`, `inactive`, `active`, `closed`, `determined`, `disputed`, `amended`, `finalized` |
| `result` | string | `yes`, `no`, or empty |
| `can_close_early` | bool | Early closure allowed |
| **Settlement** | | |
| `settlement_value` | int \| null | YES settlement (cents) |
| `settlement_value_dollars` | string \| null | YES settlement |
| `expiration_value` | string \| null | Outcome value |
| **Strike** | | |
| `strike_type` | string | `greater`, `less`, `between`, `functional`, `custom`, `structured` |
| `floor_strike` | float \| null | Lower bound |
| `cap_strike` | float \| null | Upper bound |
| **Rules** | | |
| `rules_primary` | string | Primary rules |
| `rules_secondary` | string | Secondary rules |
| **Config** | | |
| `tick_size` | int | Min price movement |
| `risk_limit_cents` | int | Max position |
| `response_price_units` | string | `usd_cent` |
| **Multivariate** | | |
| `mve_collection_ticker` | string \| null | Collection ID |
| `mve_selected_legs` | array \| null | Selected legs |
