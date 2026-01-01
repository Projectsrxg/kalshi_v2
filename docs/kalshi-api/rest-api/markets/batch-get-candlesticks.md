# Batch Get Candlesticks

```
GET /markets/candlesticks
```

**Auth**: None

## Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `tickers` | string | Yes | Comma-sep tickers (max 100) |
| `period_interval` | int | Yes | `1`, `60`, or `1440` |
| `start_ts` | int | No | Start timestamp |
| `end_ts` | int | No | End timestamp |
| `include_latest_before_start` | bool | No | Include synthetic initial candle |

## Response

```json
{
  "candlesticks": {
    "MARKET1": [...],
    "MARKET2": [...]
  }
}
```

Max 10,000 total candlesticks across all markets.
