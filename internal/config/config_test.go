package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
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
	if cfg.API.RestURL != "https://demo-api.kalshi.co/trade-api/v2" {
		t.Errorf("API.RestURL = %q, want %q", cfg.API.RestURL, "https://demo-api.kalshi.co/trade-api/v2")
	}
	if cfg.Database.Postgres.Host != "localhost" {
		t.Errorf("Database.Postgres.Host = %q, want %q", cfg.Database.Postgres.Host, "localhost")
	}
}

func TestLoadWithEnvSubstitution(t *testing.T) {
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

	// Check defaults were applied
	if cfg.API.RestURL != DefaultRestURL {
		t.Errorf("API.RestURL = %q, want default %q", cfg.API.RestURL, DefaultRestURL)
	}
	if cfg.API.Timeout != DefaultAPITimeout {
		t.Errorf("API.Timeout = %v, want default %v", cfg.API.Timeout, DefaultAPITimeout)
	}
	if cfg.Database.Postgres.Port != DefaultDBPort {
		t.Errorf("Database.Postgres.Port = %d, want default %d", cfg.Database.Postgres.Port, DefaultDBPort)
	}
	if cfg.Database.Postgres.MaxConns != DefaultMaxConns {
		t.Errorf("Database.Postgres.MaxConns = %d, want default %d", cfg.Database.Postgres.MaxConns, DefaultMaxConns)
	}
	if cfg.Metrics.Port != DefaultMetricsPort {
		t.Errorf("Metrics.Port = %d, want default %d", cfg.Metrics.Port, DefaultMetricsPort)
	}
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
			name: "min_conns exceeds max_conns",
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

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}
