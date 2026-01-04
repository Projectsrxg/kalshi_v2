// Package writer implements batch writers for all data types.
//
// Writers:
//   - Orderbook delta writer (TimescaleDB)
//   - Trade writer (TimescaleDB)
//   - Ticker writer (TimescaleDB)
//   - Snapshot writer (TimescaleDB)
//   - Market/event writer (PostgreSQL)
//
// All writers use append-only semantics (never update, only insert).
// Prices are stored as integer hundred-thousandths (0-100,000 = $0.00-$1.00) for 5-digit sub-penny precision.
package writer
