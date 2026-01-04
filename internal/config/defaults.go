package config

import "time"

// Default values for optional configuration fields.
const (
	DefaultRestURL              = "https://api.elections.kalshi.com/trade-api/v2"
	DefaultWSURL                = "wss://api.elections.kalshi.com"
	DefaultAPITimeout           = 30 * time.Second
	DefaultMaxRetries           = 3
	DefaultDBPort               = 5432
	DefaultDBSSLMode            = "prefer"
	DefaultMaxConns             = 10
	DefaultMinConns             = 2
	DefaultOrderbookCount       = 144
	DefaultMarketsPerConnection = 250
	DefaultGlobalCount          = 6
	DefaultReconnectBaseDelay   = 1 * time.Second
	DefaultReconnectMaxDelay    = 60 * time.Second
	DefaultPingInterval         = 15 * time.Second
	DefaultReadTimeout          = 30 * time.Second
	DefaultBatchSize            = 1000
	DefaultFlushInterval        = 1 * time.Second
	DefaultBufferSize           = 10000
	DefaultPollInterval         = 15 * time.Minute
	DefaultPollConcurrency      = 10
	DefaultMetricsPort          = 9090
	DefaultMetricsPath          = "/metrics"
)

func (c *GathererConfig) applyDefaults() {
	// API defaults
	if c.API.RestURL == "" {
		c.API.RestURL = DefaultRestURL
	}
	if c.API.WSURL == "" {
		c.API.WSURL = DefaultWSURL
	}
	if c.API.Timeout == 0 {
		c.API.Timeout = DefaultAPITimeout
	}
	if c.API.MaxRetries == 0 {
		c.API.MaxRetries = DefaultMaxRetries
	}

	// Database defaults
	applyDBDefaults(&c.Database.Postgres)
	applyDBDefaults(&c.Database.Timescale)

	// Connections defaults
	if c.Connections.OrderbookCount == 0 {
		c.Connections.OrderbookCount = DefaultOrderbookCount
	}
	if c.Connections.MarketsPerConnection == 0 {
		c.Connections.MarketsPerConnection = DefaultMarketsPerConnection
	}
	if c.Connections.GlobalCount == 0 {
		c.Connections.GlobalCount = DefaultGlobalCount
	}
	if c.Connections.ReconnectBaseDelay == 0 {
		c.Connections.ReconnectBaseDelay = DefaultReconnectBaseDelay
	}
	if c.Connections.ReconnectMaxDelay == 0 {
		c.Connections.ReconnectMaxDelay = DefaultReconnectMaxDelay
	}
	if c.Connections.PingInterval == 0 {
		c.Connections.PingInterval = DefaultPingInterval
	}
	if c.Connections.ReadTimeout == 0 {
		c.Connections.ReadTimeout = DefaultReadTimeout
	}

	// Writers defaults
	if c.Writers.BatchSize == 0 {
		c.Writers.BatchSize = DefaultBatchSize
	}
	if c.Writers.FlushInterval == 0 {
		c.Writers.FlushInterval = DefaultFlushInterval
	}
	if c.Writers.BufferSize == 0 {
		c.Writers.BufferSize = DefaultBufferSize
	}

	// Poller defaults
	if c.Poller.Interval == 0 {
		c.Poller.Interval = DefaultPollInterval
	}
	if c.Poller.Concurrency == 0 {
		c.Poller.Concurrency = DefaultPollConcurrency
	}

	// Metrics defaults
	if c.Metrics.Port == 0 {
		c.Metrics.Port = DefaultMetricsPort
	}
	if c.Metrics.Path == "" {
		c.Metrics.Path = DefaultMetricsPath
	}
}

func applyDBDefaults(db *DBConfig) {
	if db.Port == 0 {
		db.Port = DefaultDBPort
	}
	if db.SSLMode == "" {
		db.SSLMode = DefaultDBSSLMode
	}
	if db.MaxConns == 0 {
		db.MaxConns = DefaultMaxConns
	}
	if db.MinConns == 0 {
		db.MinConns = DefaultMinConns
	}
}
