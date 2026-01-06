# Get Positions

```
GET /portfolio/positions
```

**Auth**: Required

## Query Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `cursor` | string | - | Pagination cursor |
| `limit` | int | 100 | Results per page (1-1000) |
| `count_filter` | string | - | `position`, `total_traded` |
| `ticker` | string | - | Filter by market ticker |
| `event_ticker` | string | - | Filter by event (comma-sep, max 10) |

## Response

| Field | Type | Description |
|-------|------|-------------|
| `market_positions` | array | Position objects |
| `event_positions` | array | Event-level positions |
| `cursor` | string | Next page cursor |

### Market Position Object

| Field | Type | Description |
|-------|------|-------------|
| `ticker` | string | Market ID |
| `position` | int | Contract count (+ = YES, - = NO) |
| `total_traded` | int | Total spent (cents) |
| `total_traded_dollars` | string | Total spent |
| `market_exposure` | int | Position cost (cents) |
| `market_exposure_dollars` | string | Position cost |
| `realized_pnl` | int | Realized P&L (cents) |
| `realized_pnl_dollars` | string | Realized P&L |
| `resting_orders_count` | int | Resting order size |
| `fees_paid` | int | Fees paid (cents) |
| `fees_paid_dollars` | string | Fees paid |
| `last_updated_ts` | int | Last update timestamp |

### Event Position Object

| Field | Type | Description |
|-------|------|-------------|
| `event_ticker` | string | Event ID |
| `event_exposure` | int | Event exposure (cents) |
| `realized_pnl` | int | Realized P&L (cents) |
| `total_cost_shares` | int | Total shares traded |
