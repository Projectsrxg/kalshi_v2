package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rickgao/kalshi-data/internal/config"
)

// Pools holds database connections for a gatherer.
// Note: Gatherers only use TimescaleDB for time-series data.
// Market metadata lives in-memory (Market Registry).
type Pools struct {
	// Timescale holds trades, orderbook deltas, snapshots (time-series data).
	Timescale *pgxpool.Pool
}

// NewPools creates connection pool for TimescaleDB.
func NewPools(ctx context.Context, cfg config.DatabaseConfig) (*Pools, error) {
	ts, err := Connect(ctx, cfg.Timescale)
	if err != nil {
		return nil, fmt.Errorf("connect timescale: %w", err)
	}

	return &Pools{
		Timescale: ts,
	}, nil
}

// Connect creates a single connection pool.
func Connect(ctx context.Context, cfg config.DBConfig) (*pgxpool.Pool, error) {
	connStr := BuildConnString(cfg)

	poolCfg, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("parse connection string: %w", err)
	}

	poolCfg.MinConns = int32(cfg.MinConns)
	poolCfg.MaxConns = int32(cfg.MaxConns)

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}

// Close closes the connection pool.
func (p *Pools) Close() {
	if p.Timescale != nil {
		p.Timescale.Close()
	}
}

// Ping verifies the connection is healthy.
func (p *Pools) Ping(ctx context.Context) error {
	if err := p.Timescale.Ping(ctx); err != nil {
		return fmt.Errorf("ping timescale: %w", err)
	}
	return nil
}
