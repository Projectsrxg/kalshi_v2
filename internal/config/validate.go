package config

import (
	"errors"
	"fmt"
)

// Validate checks that all required fields are set and values are valid.
func (c *GathererConfig) Validate() error {
	if c.Instance.ID == "" {
		return errors.New("instance.id is required")
	}

	if err := c.Database.Timescale.validate("database.timescale"); err != nil {
		return err
	}

	if c.Connections.OrderbookCount < 1 {
		return errors.New("connections.orderbook_count must be >= 1")
	}
	if c.Connections.MarketsPerConnection < 1 {
		return errors.New("connections.markets_per_connection must be >= 1")
	}

	if c.Writers.BatchSize < 1 {
		return errors.New("writers.batch_size must be >= 1")
	}
	if c.Writers.BufferSize < 1 {
		return errors.New("writers.buffer_size must be >= 1")
	}

	if c.Poller.Concurrency < 1 {
		return errors.New("poller.concurrency must be >= 1")
	}

	if c.Metrics.Port < 1 || c.Metrics.Port > 65535 {
		return fmt.Errorf("metrics.port must be between 1 and 65535, got %d", c.Metrics.Port)
	}

	return nil
}

func (db *DBConfig) validate(prefix string) error {
	if db.Host == "" {
		return fmt.Errorf("%s.host is required", prefix)
	}
	if db.Name == "" {
		return fmt.Errorf("%s.name is required", prefix)
	}
	if db.User == "" {
		return fmt.Errorf("%s.user is required", prefix)
	}
	if db.Password == "" {
		return fmt.Errorf("%s.password is required", prefix)
	}
	if db.MaxConns < 1 {
		return fmt.Errorf("%s.max_conns must be >= 1", prefix)
	}
	if db.MinConns < 0 {
		return fmt.Errorf("%s.min_conns must be >= 0", prefix)
	}
	if db.MinConns > db.MaxConns {
		return fmt.Errorf("%s.min_conns (%d) cannot exceed max_conns (%d)", prefix, db.MinConns, db.MaxConns)
	}
	return nil
}
