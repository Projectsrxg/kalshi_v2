# Model Package

Shared data types used across the Kalshi Data Platform.

## Types

### Relational (PostgreSQL)

| Type | Description |
|------|-------------|
| `Series` | Collection of related events |
| `Event` | Specific event within a series |
| `Market` | Tradeable prediction market |

### Time-Series (TimescaleDB)

| Type | Description |
|------|-------------|
| `Trade` | Executed trade |
| `OrderbookDelta` | Change to orderbook at a price level |
| `OrderbookSnapshot` | Full orderbook state |
| `Ticker` | Price/volume update |
| `PriceLevel` | Single price level in orderbook |

## Conventions

### Prices

All prices are stored as **integer hundred-thousandths** (0-100,000 = $0.00-$1.00):

| Dollar Value | Internal |
|--------------|----------|
| $0.52 | 52000 |
| $0.5250 | 52500 |
| $1.00 | 100000 |

### Timestamps

All timestamps are **int64 microseconds since Unix epoch**:

| Field | Source |
|-------|--------|
| `ExchangeTS` | Kalshi server timestamp |
| `ReceivedAt` | Gatherer local timestamp |

## Usage

```go
import "github.com/rickgao/kalshi-data/internal/model"

trade := model.Trade{
    TradeID:    uuid.New(),
    ExchangeTS: time.Now().UnixMicro(),
    Ticker:     "PRES-2024-DEM",
    Price:      52000, // $0.52
    Size:       100,
    TakerSide:  true,  // YES taker
}
```
