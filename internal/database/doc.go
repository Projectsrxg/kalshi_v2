// Package database provides connection pool management for PostgreSQL and TimescaleDB.
//
// Each gatherer maintains local storage:
//   - PostgreSQL: markets, events (relational data)
//   - TimescaleDB: trades, orderbook deltas, snapshots (time-series data)
//
// Production RDS uses TimescaleDB for the deduplicated dataset.
package database
