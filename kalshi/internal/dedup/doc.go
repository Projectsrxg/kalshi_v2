// Package dedup implements the Deduplicator component.
//
// The Deduplicator:
//   - Polls all 3 gatherers via cursor-based sync
//   - Deduplicates records using composite keys:
//   - trades: trade_id (UUID from Kalshi)
//   - orderbook_deltas: (ticker, exchange_ts, price, side)
//   - orderbook_snapshots: (ticker, snapshot_ts, source)
//   - tickers: (ticker, exchange_ts)
//   - Writes deduplicated data to production RDS
//   - Optionally exports to S3 for archival
package dedup
