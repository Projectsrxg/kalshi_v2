# Get Market Orderbook

```
GET /markets/{ticker}/orderbook
```

**Auth**: None

## Path Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `ticker` | string | Market ticker |

## Query Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `depth` | int | 0 | Levels to return (0 = all, 1-100) |

## Response

| Field | Type | Description |
|-------|------|-------------|
| `orderbook.yes` | array | `[[price_cents, qty], ...]` |
| `orderbook.no` | array | `[[price_cents, qty], ...]` |
| `orderbook.yes_dollars` | array | `[["price", qty], ...]` |
| `orderbook.no_dollars` | array | `[["price", qty], ...]` |

Orderbook contains **bids only**. YES bid at X = NO ask at (100-X).

```json
{
  "orderbook": {
    "yes": [[52, 1500], [51, 3200]],
    "no": [[48, 1800], [47, 2500]],
    "yes_dollars": [["0.52", 1500], ["0.51", 3200]],
    "no_dollars": [["0.48", 1800], ["0.47", 2500]]
  }
}
```
