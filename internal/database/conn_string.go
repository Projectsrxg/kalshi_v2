package database

import (
	"fmt"
	"net/url"

	"github.com/rickgao/kalshi-data/internal/config"
)

// BuildConnString builds a PostgreSQL connection string from config.
func BuildConnString(cfg config.DBConfig) string {
	// URL-encode password to handle special characters
	escapedPassword := url.QueryEscape(cfg.Password)

	sslMode := cfg.SSLMode
	if sslMode == "" {
		sslMode = "prefer"
	}

	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.User,
		escapedPassword,
		cfg.Host,
		cfg.Port,
		cfg.Name,
		sslMode,
	)
}
