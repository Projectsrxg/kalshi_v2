# Database Package

Connection pool management for PostgreSQL and TimescaleDB.

## Databases

Each gatherer maintains two local databases:

| Database | Engine | Purpose |
|----------|--------|---------|
| PostgreSQL | PostgreSQL | Markets, events (relational data) |
| TimescaleDB | PostgreSQL + TimescaleDB | Trades, orderbook deltas, snapshots (time-series) |

## Features

- Connection pooling with configurable limits
- Automatic reconnection
- Health checks
- Metrics exposition

## Usage

```go
pool, err := database.NewPool(cfg.Database.Postgres)
if err != nil {
    log.Fatal(err)
}
defer pool.Close()
```
