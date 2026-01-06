# Writer Package

Batch writers for all data types.

## Writers

| Writer | Table | Database |
|--------|-------|----------|
| Orderbook Delta | `orderbook_deltas` | TimescaleDB |
| Trade | `trades` | TimescaleDB |
| Ticker | `tickers` | TimescaleDB |
| Snapshot | `orderbook_snapshots` | TimescaleDB |
| Market | `markets` | PostgreSQL |
| Event | `events` | PostgreSQL |

## Design Principles

- **Append-only**: Never update, only insert
- **Batch writes**: Configurable batch size and flush interval
- **Integer pricing**: Prices as hundred-thousandths (0-100,000 = $0.00-$1.00) for 5-digit sub-penny precision
- **Microsecond timestamps**: All timestamps as `BIGINT` (µs since epoch)

## Price Conversion

| API Value | Internal | Notes |
|-----------|----------|-------|
| `"0.52"` | 52000 | Standard penny |
| `"0.5250"` | 52500 | Sub-penny (0.1 cent) |
| `"0.52505"` | 52505 | Sub-penny (0.01 cent) |

## Usage

```go
w := writer.NewOrderbookDeltaWriter(cfg.Writers, db)
w.Write(&OrderbookDelta{...})
```
