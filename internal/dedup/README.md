# Dedup Package

Deduplicator - merges data from all gatherers into production database.

## Deduplication Keys

| Table | Primary Key |
|-------|-------------|
| `trades` | `trade_id` (UUID from Kalshi) |
| `orderbook_deltas` | `(ticker, exchange_ts, price, side)` |
| `orderbook_snapshots` | `(ticker, snapshot_ts, source)` |
| `tickers` | `(ticker, exchange_ts)` |

**Note**: `seq` (sequence number) is NOT used for deduplication - it's per-subscription and differs across gatherers.

## Process

1. Poll all 3 gatherer databases via cursor-based sync
2. Deduplicate using composite keys
3. Write unique records to production RDS
4. Optionally export to S3

## Usage

```go
d := dedup.New(cfg.Dedup, sources, production)
d.Start(ctx)
```
