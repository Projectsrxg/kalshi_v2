package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	t.Run("basic loading", func(t *testing.T) {
		yaml := `
instance:
  id: test-gatherer
  az: us-east-1a
api:
  rest_url: https://demo-api.kalshi.co/trade-api/v2
database:
  postgres:
    host: localhost
    port: 5432
    name: test_db
    user: testuser
    password: testpass
  timescale:
    host: localhost
    port: 5432
    name: test_ts
    user: testuser
    password: testpass
`
		path := writeTempFile(t, yaml)

		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}

		if cfg.Instance.ID != "test-gatherer" {
			t.Errorf("Instance.ID = %q, want %q", cfg.Instance.ID, "test-gatherer")
		}
		if cfg.Instance.AZ != "us-east-1a" {
			t.Errorf("Instance.AZ = %q, want %q", cfg.Instance.AZ, "us-east-1a")
		}
		if cfg.API.RestURL != "https://demo-api.kalshi.co/trade-api/v2" {
			t.Errorf("API.RestURL = %q, want %q", cfg.API.RestURL, "https://demo-api.kalshi.co/trade-api/v2")
		}
		if cfg.Database.Postgres.Host != "localhost" {
			t.Errorf("Database.Postgres.Host = %q, want %q", cfg.Database.Postgres.Host, "localhost")
		}
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := Load("/nonexistent/path/config.yaml")
		if err == nil {
			t.Fatal("expected error for nonexistent file")
		}
		if !strings.Contains(err.Error(), "read config file") {
			t.Errorf("error should mention 'read config file', got %v", err)
		}
	})

	t.Run("invalid yaml", func(t *testing.T) {
		yaml := `
instance:
  id: test
  invalid yaml here: [
`
		path := writeTempFile(t, yaml)

		_, err := Load(path)
		if err == nil {
			t.Fatal("expected error for invalid YAML")
		}
		if !strings.Contains(err.Error(), "parse config yaml") {
			t.Errorf("error should mention 'parse config yaml', got %v", err)
		}
	})

	t.Run("empty file", func(t *testing.T) {
		path := writeTempFile(t, "")

		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}
		// Empty config should still work, just with zero values
		if cfg.Instance.ID != "" {
			t.Errorf("Instance.ID = %q, want empty", cfg.Instance.ID)
		}
	})
}

func TestLoadWithEnvSubstitution(t *testing.T) {
	t.Run("single env var", func(t *testing.T) {
		t.Setenv("TEST_DB_PASSWORD", "secret123")

		yaml := `
instance:
  id: test-gatherer
database:
  postgres:
    host: localhost
    name: test_db
    user: testuser
    password: ${TEST_DB_PASSWORD}
  timescale:
    host: localhost
    name: test_ts
    user: testuser
    password: ${TEST_DB_PASSWORD}
`
		path := writeTempFile(t, yaml)

		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}

		if cfg.Database.Postgres.Password != "secret123" {
			t.Errorf("Database.Postgres.Password = %q, want %q", cfg.Database.Postgres.Password, "secret123")
		}
	})

	t.Run("multiple env vars", func(t *testing.T) {
		t.Setenv("TEST_HOST", "db.example.com")
		t.Setenv("TEST_USER", "admin")
		t.Setenv("TEST_PASS", "securepass")

		yaml := `
instance:
  id: test
database:
  postgres:
    host: ${TEST_HOST}
    name: db
    user: ${TEST_USER}
    password: ${TEST_PASS}
`
		path := writeTempFile(t, yaml)

		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}

		if cfg.Database.Postgres.Host != "db.example.com" {
			t.Errorf("Host = %q, want %q", cfg.Database.Postgres.Host, "db.example.com")
		}
		if cfg.Database.Postgres.User != "admin" {
			t.Errorf("User = %q, want %q", cfg.Database.Postgres.User, "admin")
		}
		if cfg.Database.Postgres.Password != "securepass" {
			t.Errorf("Password = %q, want %q", cfg.Database.Postgres.Password, "securepass")
		}
	})

	t.Run("unset env var results in empty", func(t *testing.T) {
		// Make sure this env var is not set
		os.Unsetenv("UNSET_VAR_FOR_TEST")

		yaml := `
instance:
  id: ${UNSET_VAR_FOR_TEST}
`
		path := writeTempFile(t, yaml)

		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}

		if cfg.Instance.ID != "" {
			t.Errorf("Instance.ID = %q, want empty for unset env var", cfg.Instance.ID)
		}
	})
}

func TestLoadWithDefaults(t *testing.T) {
	yaml := `
instance:
  id: test-gatherer
database:
  postgres:
    host: localhost
    name: test_db
    user: testuser
    password: testpass
  timescale:
    host: localhost
    name: test_ts
    user: testuser
    password: testpass
`
	path := writeTempFile(t, yaml)

	cfg, err := LoadWithDefaults(path)
	if err != nil {
		t.Fatalf("LoadWithDefaults failed: %v", err)
	}

	// Check API defaults
	if cfg.API.RestURL != DefaultRestURL {
		t.Errorf("API.RestURL = %q, want default %q", cfg.API.RestURL, DefaultRestURL)
	}
	if cfg.API.WSURL != DefaultWSURL {
		t.Errorf("API.WSURL = %q, want default %q", cfg.API.WSURL, DefaultWSURL)
	}
	if cfg.API.Timeout != DefaultAPITimeout {
		t.Errorf("API.Timeout = %v, want default %v", cfg.API.Timeout, DefaultAPITimeout)
	}
	if cfg.API.MaxRetries != DefaultMaxRetries {
		t.Errorf("API.MaxRetries = %d, want default %d", cfg.API.MaxRetries, DefaultMaxRetries)
	}

	// Check database defaults
	if cfg.Database.Postgres.Port != DefaultDBPort {
		t.Errorf("Database.Postgres.Port = %d, want default %d", cfg.Database.Postgres.Port, DefaultDBPort)
	}
	if cfg.Database.Postgres.SSLMode != DefaultDBSSLMode {
		t.Errorf("Database.Postgres.SSLMode = %q, want default %q", cfg.Database.Postgres.SSLMode, DefaultDBSSLMode)
	}
	if cfg.Database.Postgres.MaxConns != DefaultMaxConns {
		t.Errorf("Database.Postgres.MaxConns = %d, want default %d", cfg.Database.Postgres.MaxConns, DefaultMaxConns)
	}
	if cfg.Database.Postgres.MinConns != DefaultMinConns {
		t.Errorf("Database.Postgres.MinConns = %d, want default %d", cfg.Database.Postgres.MinConns, DefaultMinConns)
	}
	if cfg.Database.Timescale.Port != DefaultDBPort {
		t.Errorf("Database.Timescale.Port = %d, want default %d", cfg.Database.Timescale.Port, DefaultDBPort)
	}

	// Check connections defaults
	if cfg.Connections.OrderbookCount != DefaultOrderbookCount {
		t.Errorf("Connections.OrderbookCount = %d, want default %d", cfg.Connections.OrderbookCount, DefaultOrderbookCount)
	}
	if cfg.Connections.MarketsPerConnection != DefaultMarketsPerConnection {
		t.Errorf("Connections.MarketsPerConnection = %d, want default %d", cfg.Connections.MarketsPerConnection, DefaultMarketsPerConnection)
	}
	if cfg.Connections.GlobalCount != DefaultGlobalCount {
		t.Errorf("Connections.GlobalCount = %d, want default %d", cfg.Connections.GlobalCount, DefaultGlobalCount)
	}
	if cfg.Connections.ReconnectBaseDelay != DefaultReconnectBaseDelay {
		t.Errorf("Connections.ReconnectBaseDelay = %v, want default %v", cfg.Connections.ReconnectBaseDelay, DefaultReconnectBaseDelay)
	}
	if cfg.Connections.ReconnectMaxDelay != DefaultReconnectMaxDelay {
		t.Errorf("Connections.ReconnectMaxDelay = %v, want default %v", cfg.Connections.ReconnectMaxDelay, DefaultReconnectMaxDelay)
	}
	if cfg.Connections.PingInterval != DefaultPingInterval {
		t.Errorf("Connections.PingInterval = %v, want default %v", cfg.Connections.PingInterval, DefaultPingInterval)
	}
	if cfg.Connections.ReadTimeout != DefaultReadTimeout {
		t.Errorf("Connections.ReadTimeout = %v, want default %v", cfg.Connections.ReadTimeout, DefaultReadTimeout)
	}

	// Check writers defaults
	if cfg.Writers.BatchSize != DefaultBatchSize {
		t.Errorf("Writers.BatchSize = %d, want default %d", cfg.Writers.BatchSize, DefaultBatchSize)
	}
	if cfg.Writers.FlushInterval != DefaultFlushInterval {
		t.Errorf("Writers.FlushInterval = %v, want default %v", cfg.Writers.FlushInterval, DefaultFlushInterval)
	}
	if cfg.Writers.BufferSize != DefaultBufferSize {
		t.Errorf("Writers.BufferSize = %d, want default %d", cfg.Writers.BufferSize, DefaultBufferSize)
	}

	// Check poller defaults
	if cfg.Poller.Interval != DefaultPollInterval {
		t.Errorf("Poller.Interval = %v, want default %v", cfg.Poller.Interval, DefaultPollInterval)
	}
	if cfg.Poller.Concurrency != DefaultPollConcurrency {
		t.Errorf("Poller.Concurrency = %d, want default %d", cfg.Poller.Concurrency, DefaultPollConcurrency)
	}

	// Check metrics defaults
	if cfg.Metrics.Port != DefaultMetricsPort {
		t.Errorf("Metrics.Port = %d, want default %d", cfg.Metrics.Port, DefaultMetricsPort)
	}
	if cfg.Metrics.Path != DefaultMetricsPath {
		t.Errorf("Metrics.Path = %q, want default %q", cfg.Metrics.Path, DefaultMetricsPath)
	}
}

func TestLoadWithDefaultsPreservesSetValues(t *testing.T) {
	yaml := `
instance:
  id: test-gatherer
api:
  rest_url: https://custom.api.com
  timeout: 60s
  max_retries: 5
database:
  postgres:
    host: customhost
    port: 5433
    name: test_db
    user: testuser
    password: testpass
    ssl_mode: require
    max_conns: 20
    min_conns: 5
  timescale:
    host: localhost
    name: test_ts
    user: testuser
    password: testpass
connections:
  orderbook_count: 100
writers:
  batch_size: 500
poller:
  interval: 10m
  concurrency: 50
metrics:
  port: 8080
  path: /health
`
	path := writeTempFile(t, yaml)

	cfg, err := LoadWithDefaults(path)
	if err != nil {
		t.Fatalf("LoadWithDefaults failed: %v", err)
	}

	// Verify custom values are preserved
	if cfg.API.RestURL != "https://custom.api.com" {
		t.Errorf("API.RestURL = %q, want custom value", cfg.API.RestURL)
	}
	if cfg.API.Timeout != 60*time.Second {
		t.Errorf("API.Timeout = %v, want 60s", cfg.API.Timeout)
	}
	if cfg.API.MaxRetries != 5 {
		t.Errorf("API.MaxRetries = %d, want 5", cfg.API.MaxRetries)
	}
	if cfg.Database.Postgres.Port != 5433 {
		t.Errorf("Database.Postgres.Port = %d, want 5433", cfg.Database.Postgres.Port)
	}
	if cfg.Database.Postgres.SSLMode != "require" {
		t.Errorf("Database.Postgres.SSLMode = %q, want 'require'", cfg.Database.Postgres.SSLMode)
	}
	if cfg.Connections.OrderbookCount != 100 {
		t.Errorf("Connections.OrderbookCount = %d, want 100", cfg.Connections.OrderbookCount)
	}
	if cfg.Writers.BatchSize != 500 {
		t.Errorf("Writers.BatchSize = %d, want 500", cfg.Writers.BatchSize)
	}
	if cfg.Poller.Interval != 10*time.Minute {
		t.Errorf("Poller.Interval = %v, want 10m", cfg.Poller.Interval)
	}
	if cfg.Metrics.Port != 8080 {
		t.Errorf("Metrics.Port = %d, want 8080", cfg.Metrics.Port)
	}
}

func TestLoadAndValidate(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		yaml := `
instance:
  id: test-gatherer
database:
  postgres:
    host: localhost
    name: test_db
    user: testuser
    password: testpass
  timescale:
    host: localhost
    name: test_ts
    user: testuser
    password: testpass
`
		path := writeTempFile(t, yaml)

		cfg, err := LoadAndValidate(path)
		if err != nil {
			t.Fatalf("LoadAndValidate failed: %v", err)
		}

		if cfg.Instance.ID != "test-gatherer" {
			t.Errorf("Instance.ID = %q, want %q", cfg.Instance.ID, "test-gatherer")
		}
	})

	t.Run("invalid config returns validation error", func(t *testing.T) {
		yaml := `
instance:
  id: ""
`
		path := writeTempFile(t, yaml)

		_, err := LoadAndValidate(path)
		if err == nil {
			t.Fatal("expected validation error")
		}
		if !strings.Contains(err.Error(), "validate config") {
			t.Errorf("error should mention 'validate config', got %v", err)
		}
	})

	t.Run("load error propagates", func(t *testing.T) {
		_, err := LoadAndValidate("/nonexistent/path/config.yaml")
		if err == nil {
			t.Fatal("expected load error")
		}
	})
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     GathererConfig
		wantErr string
	}{
		{
			name:    "missing instance id",
			cfg:     GathererConfig{},
			wantErr: "instance.id is required",
		},
		{
			name: "missing postgres host",
			cfg: GathererConfig{
				Instance: InstanceConfig{ID: "test"},
			},
			wantErr: "database.postgres.host is required",
		},
		{
			name: "missing postgres name",
			cfg: GathererConfig{
				Instance: InstanceConfig{ID: "test"},
				Database: DatabaseConfig{
					Postgres: DBConfig{Host: "localhost"},
				},
			},
			wantErr: "database.postgres.name is required",
		},
		{
			name: "missing postgres user",
			cfg: GathererConfig{
				Instance: InstanceConfig{ID: "test"},
				Database: DatabaseConfig{
					Postgres: DBConfig{Host: "localhost", Name: "db"},
				},
			},
			wantErr: "database.postgres.user is required",
		},
		{
			name: "missing postgres password",
			cfg: GathererConfig{
				Instance: InstanceConfig{ID: "test"},
				Database: DatabaseConfig{
					Postgres: DBConfig{Host: "localhost", Name: "db", User: "user"},
				},
			},
			wantErr: "database.postgres.password is required",
		},
		{
			name: "postgres max_conns < 1",
			cfg: GathererConfig{
				Instance: InstanceConfig{ID: "test"},
				Database: DatabaseConfig{
					Postgres: DBConfig{Host: "localhost", Name: "db", User: "user", Password: "pass", MaxConns: 0},
				},
			},
			wantErr: "database.postgres.max_conns must be >= 1",
		},
		{
			name: "postgres min_conns < 0",
			cfg: GathererConfig{
				Instance: InstanceConfig{ID: "test"},
				Database: DatabaseConfig{
					Postgres: DBConfig{Host: "localhost", Name: "db", User: "user", Password: "pass", MaxConns: 5, MinConns: -1},
				},
			},
			wantErr: "database.postgres.min_conns must be >= 0",
		},
		{
			name: "postgres min_conns exceeds max_conns",
			cfg: GathererConfig{
				Instance: InstanceConfig{ID: "test"},
				Database: DatabaseConfig{
					Postgres:  DBConfig{Host: "localhost", Name: "db", User: "user", Password: "pass", MaxConns: 5, MinConns: 10},
					Timescale: DBConfig{Host: "localhost", Name: "db", User: "user", Password: "pass", MaxConns: 5},
				},
			},
			wantErr: "database.postgres.min_conns (10) cannot exceed max_conns (5)",
		},
		{
			name: "missing timescale host",
			cfg: GathererConfig{
				Instance: InstanceConfig{ID: "test"},
				Database: DatabaseConfig{
					Postgres:  DBConfig{Host: "localhost", Name: "db", User: "user", Password: "pass", MaxConns: 5},
					Timescale: DBConfig{},
				},
			},
			wantErr: "database.timescale.host is required",
		},
		{
			name: "connections orderbook_count < 1",
			cfg: GathererConfig{
				Instance: InstanceConfig{ID: "test"},
				Database: DatabaseConfig{
					Postgres:  DBConfig{Host: "localhost", Name: "db", User: "user", Password: "pass", MaxConns: 5},
					Timescale: DBConfig{Host: "localhost", Name: "db", User: "user", Password: "pass", MaxConns: 5},
				},
				Connections: ConnectionsConfig{
					OrderbookCount: 0,
				},
			},
			wantErr: "connections.orderbook_count must be >= 1",
		},
		{
			name: "connections markets_per_connection < 1",
			cfg: GathererConfig{
				Instance: InstanceConfig{ID: "test"},
				Database: DatabaseConfig{
					Postgres:  DBConfig{Host: "localhost", Name: "db", User: "user", Password: "pass", MaxConns: 5},
					Timescale: DBConfig{Host: "localhost", Name: "db", User: "user", Password: "pass", MaxConns: 5},
				},
				Connections: ConnectionsConfig{
					OrderbookCount:       100,
					MarketsPerConnection: 0,
				},
			},
			wantErr: "connections.markets_per_connection must be >= 1",
		},
		{
			name: "writers batch_size < 1",
			cfg: GathererConfig{
				Instance: InstanceConfig{ID: "test"},
				Database: DatabaseConfig{
					Postgres:  DBConfig{Host: "localhost", Name: "db", User: "user", Password: "pass", MaxConns: 5},
					Timescale: DBConfig{Host: "localhost", Name: "db", User: "user", Password: "pass", MaxConns: 5},
				},
				Connections: ConnectionsConfig{
					OrderbookCount:       100,
					MarketsPerConnection: 250,
				},
				Writers: WritersConfig{
					BatchSize: 0,
				},
			},
			wantErr: "writers.batch_size must be >= 1",
		},
		{
			name: "writers buffer_size < 1",
			cfg: GathererConfig{
				Instance: InstanceConfig{ID: "test"},
				Database: DatabaseConfig{
					Postgres:  DBConfig{Host: "localhost", Name: "db", User: "user", Password: "pass", MaxConns: 5},
					Timescale: DBConfig{Host: "localhost", Name: "db", User: "user", Password: "pass", MaxConns: 5},
				},
				Connections: ConnectionsConfig{
					OrderbookCount:       100,
					MarketsPerConnection: 250,
				},
				Writers: WritersConfig{
					BatchSize:  1000,
					BufferSize: 0,
				},
			},
			wantErr: "writers.buffer_size must be >= 1",
		},
		{
			name: "poller concurrency < 1",
			cfg: GathererConfig{
				Instance: InstanceConfig{ID: "test"},
				Database: DatabaseConfig{
					Postgres:  DBConfig{Host: "localhost", Name: "db", User: "user", Password: "pass", MaxConns: 5},
					Timescale: DBConfig{Host: "localhost", Name: "db", User: "user", Password: "pass", MaxConns: 5},
				},
				Connections: ConnectionsConfig{
					OrderbookCount:       100,
					MarketsPerConnection: 250,
				},
				Writers: WritersConfig{
					BatchSize:  1000,
					BufferSize: 10000,
				},
				Poller: PollerConfig{
					Concurrency: 0,
				},
			},
			wantErr: "poller.concurrency must be >= 1",
		},
		{
			name: "metrics port < 1",
			cfg: GathererConfig{
				Instance: InstanceConfig{ID: "test"},
				Database: DatabaseConfig{
					Postgres:  DBConfig{Host: "localhost", Name: "db", User: "user", Password: "pass", MaxConns: 5},
					Timescale: DBConfig{Host: "localhost", Name: "db", User: "user", Password: "pass", MaxConns: 5},
				},
				Connections: ConnectionsConfig{
					OrderbookCount:       100,
					MarketsPerConnection: 250,
				},
				Writers: WritersConfig{
					BatchSize:  1000,
					BufferSize: 10000,
				},
				Poller: PollerConfig{
					Concurrency: 10,
				},
				Metrics: MetricsConfig{
					Port: 0,
				},
			},
			wantErr: "metrics.port must be between 1 and 65535, got 0",
		},
		{
			name: "metrics port > 65535",
			cfg: GathererConfig{
				Instance: InstanceConfig{ID: "test"},
				Database: DatabaseConfig{
					Postgres:  DBConfig{Host: "localhost", Name: "db", User: "user", Password: "pass", MaxConns: 5},
					Timescale: DBConfig{Host: "localhost", Name: "db", User: "user", Password: "pass", MaxConns: 5},
				},
				Connections: ConnectionsConfig{
					OrderbookCount:       100,
					MarketsPerConnection: 250,
				},
				Writers: WritersConfig{
					BatchSize:  1000,
					BufferSize: 10000,
				},
				Poller: PollerConfig{
					Concurrency: 10,
				},
				Metrics: MetricsConfig{
					Port: 70000,
				},
			},
			wantErr: "metrics.port must be between 1 and 65535, got 70000",
		},
		{
			name: "valid config",
			cfg: GathererConfig{
				Instance: InstanceConfig{ID: "test"},
				Database: DatabaseConfig{
					Postgres:  DBConfig{Host: "localhost", Name: "db", User: "user", Password: "pass", MaxConns: 10, MinConns: 2},
					Timescale: DBConfig{Host: "localhost", Name: "db", User: "user", Password: "pass", MaxConns: 10, MinConns: 2},
				},
				Connections: ConnectionsConfig{
					OrderbookCount:       144,
					MarketsPerConnection: 250,
				},
				Writers: WritersConfig{
					BatchSize:     1000,
					FlushInterval: time.Second,
					BufferSize:    10000,
				},
				Poller: PollerConfig{
					Concurrency: 10,
				},
				Metrics: MetricsConfig{
					Port: 9090,
				},
			},
			wantErr: "",
		},
		{
			name: "valid config with min values",
			cfg: GathererConfig{
				Instance: InstanceConfig{ID: "t"},
				Database: DatabaseConfig{
					Postgres:  DBConfig{Host: "h", Name: "n", User: "u", Password: "p", MaxConns: 1, MinConns: 0},
					Timescale: DBConfig{Host: "h", Name: "n", User: "u", Password: "p", MaxConns: 1, MinConns: 0},
				},
				Connections: ConnectionsConfig{
					OrderbookCount:       1,
					MarketsPerConnection: 1,
				},
				Writers: WritersConfig{
					BatchSize:  1,
					BufferSize: 1,
				},
				Poller: PollerConfig{
					Concurrency: 1,
				},
				Metrics: MetricsConfig{
					Port: 1,
				},
			},
			wantErr: "",
		},
		{
			name: "valid config with max port",
			cfg: GathererConfig{
				Instance: InstanceConfig{ID: "test"},
				Database: DatabaseConfig{
					Postgres:  DBConfig{Host: "h", Name: "n", User: "u", Password: "p", MaxConns: 1},
					Timescale: DBConfig{Host: "h", Name: "n", User: "u", Password: "p", MaxConns: 1},
				},
				Connections: ConnectionsConfig{
					OrderbookCount:       1,
					MarketsPerConnection: 1,
				},
				Writers: WritersConfig{
					BatchSize:  1,
					BufferSize: 1,
				},
				Poller: PollerConfig{
					Concurrency: 1,
				},
				Metrics: MetricsConfig{
					Port: 65535,
				},
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.wantErr)
				} else if err.Error() != tt.wantErr {
					t.Errorf("Validate() error = %q, want %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

func TestDefaultConstants(t *testing.T) {
	// Verify default constant values are as documented
	if DefaultRestURL != "https://api.elections.kalshi.com/trade-api/v2" {
		t.Errorf("DefaultRestURL = %q, want production URL", DefaultRestURL)
	}
	if DefaultWSURL != "wss://api.elections.kalshi.com" {
		t.Errorf("DefaultWSURL = %q, want production URL", DefaultWSURL)
	}
	if DefaultAPITimeout != 30*time.Second {
		t.Errorf("DefaultAPITimeout = %v, want 30s", DefaultAPITimeout)
	}
	if DefaultMaxRetries != 3 {
		t.Errorf("DefaultMaxRetries = %d, want 3", DefaultMaxRetries)
	}
	if DefaultDBPort != 5432 {
		t.Errorf("DefaultDBPort = %d, want 5432", DefaultDBPort)
	}
	if DefaultDBSSLMode != "prefer" {
		t.Errorf("DefaultDBSSLMode = %q, want 'prefer'", DefaultDBSSLMode)
	}
	if DefaultMaxConns != 10 {
		t.Errorf("DefaultMaxConns = %d, want 10", DefaultMaxConns)
	}
	if DefaultMinConns != 2 {
		t.Errorf("DefaultMinConns = %d, want 2", DefaultMinConns)
	}
	if DefaultOrderbookCount != 144 {
		t.Errorf("DefaultOrderbookCount = %d, want 144", DefaultOrderbookCount)
	}
	if DefaultMarketsPerConnection != 250 {
		t.Errorf("DefaultMarketsPerConnection = %d, want 250", DefaultMarketsPerConnection)
	}
	if DefaultGlobalCount != 6 {
		t.Errorf("DefaultGlobalCount = %d, want 6", DefaultGlobalCount)
	}
	if DefaultBatchSize != 1000 {
		t.Errorf("DefaultBatchSize = %d, want 1000", DefaultBatchSize)
	}
	if DefaultFlushInterval != 1*time.Second {
		t.Errorf("DefaultFlushInterval = %v, want 1s", DefaultFlushInterval)
	}
	if DefaultBufferSize != 10000 {
		t.Errorf("DefaultBufferSize = %d, want 10000", DefaultBufferSize)
	}
	if DefaultPollInterval != 15*time.Minute {
		t.Errorf("DefaultPollInterval = %v, want 15m", DefaultPollInterval)
	}
	if DefaultPollConcurrency != 10 {
		t.Errorf("DefaultPollConcurrency = %d, want 10", DefaultPollConcurrency)
	}
	if DefaultMetricsPort != 9090 {
		t.Errorf("DefaultMetricsPort = %d, want 9090", DefaultMetricsPort)
	}
	if DefaultMetricsPath != "/metrics" {
		t.Errorf("DefaultMetricsPath = %q, want '/metrics'", DefaultMetricsPath)
	}
}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}
