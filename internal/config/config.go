package config

import "time"

// GathererConfig is the root configuration for a gatherer instance.
type GathererConfig struct {
	Instance    InstanceConfig    `yaml:"instance"`
	API         APIConfig         `yaml:"api"`
	Database    DatabaseConfig    `yaml:"database"`
	Connections ConnectionsConfig `yaml:"connections"`
	Writers     WritersConfig     `yaml:"writers"`
	Poller      PollerConfig      `yaml:"poller"`
	Metrics     MetricsConfig     `yaml:"metrics"`
}

// InstanceConfig identifies this gatherer.
type InstanceConfig struct {
	ID string `yaml:"id"`
	AZ string `yaml:"az"`
}

// APIConfig holds Kalshi API settings.
type APIConfig struct {
	RestURL        string        `yaml:"rest_url"`
	WSURL          string        `yaml:"ws_url"`
	APIKey         string        `yaml:"api_key"`          // API key ID (for KALSHI-ACCESS-KEY header)
	PrivateKeyPath string        `yaml:"private_key_path"` // Path to RSA private key PEM file
	Timeout        time.Duration `yaml:"timeout"`
	MaxRetries     int           `yaml:"max_retries"`
}

// DatabaseConfig holds the TimescaleDB connection for time-series data.
// Note: Gatherers only use TimescaleDB. Market metadata lives in-memory (Market Registry).
type DatabaseConfig struct {
	Timescale DBConfig `yaml:"timescale"`
}

// DBConfig holds a single database connection.
type DBConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Name     string `yaml:"name"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	SSLMode  string `yaml:"ssl_mode"`
	MaxConns int    `yaml:"max_conns"`
	MinConns int    `yaml:"min_conns"`
}

// ConnectionsConfig holds WebSocket connection manager settings.
type ConnectionsConfig struct {
	OrderbookCount       int           `yaml:"orderbook_count"`
	MarketsPerConnection int           `yaml:"markets_per_connection"`
	GlobalCount          int           `yaml:"global_count"`
	ReconnectBaseDelay   time.Duration `yaml:"reconnect_base_delay"`
	ReconnectMaxDelay    time.Duration `yaml:"reconnect_max_delay"`
	PingInterval         time.Duration `yaml:"ping_interval"`
	ReadTimeout          time.Duration `yaml:"read_timeout"`
}

// WritersConfig holds batch writer settings.
type WritersConfig struct {
	BatchSize     int           `yaml:"batch_size"`
	FlushInterval time.Duration `yaml:"flush_interval"`
	BufferSize    int           `yaml:"buffer_size"`
}

// PollerConfig holds snapshot poller settings.
type PollerConfig struct {
	Interval    time.Duration `yaml:"interval"`
	Concurrency int           `yaml:"concurrency"`
}

// MetricsConfig holds Prometheus metrics settings.
type MetricsConfig struct {
	Port int    `yaml:"port"`
	Path string `yaml:"path"`
}
