package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rickgao/kalshi-data/internal/config"
)

// Pools holds database connections for a gatherer.
type Pools struct {
	// Postgres holds markets, events, series (relational data).
	Postgres *pgxpool.Pool

	// Timescale holds trades, orderbook deltas, snapshots (time-series data).
	Timescale *pgxpool.Pool
}

// NewPools creates connection pools for both databases.
func NewPools(ctx context.Context, cfg config.DatabaseConfig) (*Pools, error) {
	pg, err := Connect(ctx, cfg.Postgres)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}

	ts, err := Connect(ctx, cfg.Timescale)
	if err != nil {
		pg.Close()
		return nil, fmt.Errorf("connect timescale: %w", err)
	}

	return &Pools{
		Postgres:  pg,
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

// Close closes both connection pools.
func (p *Pools) Close() {
	if p.Postgres != nil {
		p.Postgres.Close()
	}
	if p.Timescale != nil {
		p.Timescale.Close()
	}
}

// Ping verifies both connections are healthy.
func (p *Pools) Ping(ctx context.Context) error {
	if err := p.Postgres.Ping(ctx); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}
	if err := p.Timescale.Ping(ctx); err != nil {
		return fmt.Errorf("ping timescale: %w", err)
	}
	return nil
}
