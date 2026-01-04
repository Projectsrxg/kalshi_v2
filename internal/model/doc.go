// Package model defines shared data types used across the Kalshi Data Platform.
//
// All types mirror the database schema defined in docs/kalshi-data/architecture/data-model.md.
//
// Conventions:
//   - Prices: integer hundred-thousandths (0-100,000 = $0.00-$1.00)
//   - Timestamps: int64 microseconds since Unix epoch
//   - IDs: string for tickers, uuid.UUID for trade IDs
package model
