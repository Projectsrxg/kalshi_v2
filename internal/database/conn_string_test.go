package database

import (
	"context"
	"testing"
	"time"

	"github.com/rickgao/kalshi-data/internal/config"
)

func TestBuildConnString(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.DBConfig
		want string
	}{
		{
			name: "basic",
			cfg: config.DBConfig{
				Host:     "localhost",
				Port:     5432,
				Name:     "testdb",
				User:     "testuser",
				Password: "testpass",
				SSLMode:  "disable",
			},
			want: "postgres://testuser:testpass@localhost:5432/testdb?sslmode=disable",
		},
		{
			name: "password with special chars",
			cfg: config.DBConfig{
				Host:     "localhost",
				Port:     5432,
				Name:     "testdb",
				User:     "testuser",
				Password: "p@ss:word/test",
				SSLMode:  "require",
			},
			want: "postgres://testuser:p%40ss%3Aword%2Ftest@localhost:5432/testdb?sslmode=require",
		},
		{
			name: "default ssl mode",
			cfg: config.DBConfig{
				Host:     "db.example.com",
				Port:     5433,
				Name:     "proddb",
				User:     "produser",
				Password: "secret",
				SSLMode:  "",
			},
			want: "postgres://produser:secret@db.example.com:5433/proddb?sslmode=prefer",
		},
		{
			name: "password with spaces and symbols",
			cfg: config.DBConfig{
				Host:     "localhost",
				Port:     5432,
				Name:     "mydb",
				User:     "admin",
				Password: "my secret p@ss#word!",
				SSLMode:  "disable",
			},
			want: "postgres://admin:my+secret+p%40ss%23word%21@localhost:5432/mydb?sslmode=disable",
		},
		{
			name: "empty password",
			cfg: config.DBConfig{
				Host:     "localhost",
				Port:     5432,
				Name:     "mydb",
				User:     "admin",
				Password: "",
				SSLMode:  "disable",
			},
			want: "postgres://admin:@localhost:5432/mydb?sslmode=disable",
		},
		{
			name: "ssl mode verify-full",
			cfg: config.DBConfig{
				Host:     "secure-db.aws.com",
				Port:     5432,
				Name:     "proddb",
				User:     "produser",
				Password: "secret",
				SSLMode:  "verify-full",
			},
			want: "postgres://produser:secret@secure-db.aws.com:5432/proddb?sslmode=verify-full",
		},
		{
			name: "non-standard port",
			cfg: config.DBConfig{
				Host:     "localhost",
				Port:     15432,
				Name:     "testdb",
				User:     "test",
				Password: "pass",
				SSLMode:  "disable",
			},
			want: "postgres://test:pass@localhost:15432/testdb?sslmode=disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildConnString(tt.cfg)
			if got != tt.want {
				t.Errorf("BuildConnString() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestConnect_InvalidConnString tests Connect with an invalid connection string.
func TestConnect_InvalidConnString(t *testing.T) {
	cfg := config.DBConfig{
		Host:     "nonexistent-host-that-does-not-exist.invalid",
		Port:     5432,
		Name:     "testdb",
		User:     "testuser",
		Password: "testpass",
		SSLMode:  "disable",
		MinConns: 1,
		MaxConns: 5,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := Connect(ctx, cfg)
	if err == nil {
		t.Error("Connect() should fail with invalid host")
	}
}

// TestNewPools_ConnectFails tests NewPools when TimescaleDB connection fails.
func TestNewPools_ConnectFails(t *testing.T) {
	cfg := config.DatabaseConfig{
		Timescale: config.DBConfig{
			Host:     "nonexistent-host.invalid",
			Port:     5432,
			Name:     "testts",
			User:     "testuser",
			Password: "testpass",
			SSLMode:  "disable",
			MinConns: 1,
			MaxConns: 5,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := NewPools(ctx, cfg)
	if err == nil {
		t.Error("NewPools() should fail when TimescaleDB connection fails")
	}
}

// TestPools_Close tests the Close method with nil pool.
func TestPools_Close(t *testing.T) {
	t.Run("nil pool", func(t *testing.T) {
		p := &Pools{
			Timescale: nil,
		}
		// Should not panic
		p.Close()
	})
}

// TestBuildConnStringSSLModes tests all SSL modes.
func TestBuildConnStringSSLModes(t *testing.T) {
	sslModes := []string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full"}

	for _, mode := range sslModes {
		t.Run(mode, func(t *testing.T) {
			cfg := config.DBConfig{
				Host:     "localhost",
				Port:     5432,
				Name:     "testdb",
				User:     "testuser",
				Password: "testpass",
				SSLMode:  mode,
			}
			got := BuildConnString(cfg)
			expected := "postgres://testuser:testpass@localhost:5432/testdb?sslmode=" + mode
			if got != expected {
				t.Errorf("BuildConnString() = %q, want %q", got, expected)
			}
		})
	}
}
