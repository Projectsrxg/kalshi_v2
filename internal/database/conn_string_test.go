package database

import (
	"testing"

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
