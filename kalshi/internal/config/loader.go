package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Load reads a YAML config file and expands environment variables.
func Load(path string) (*GathererConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	// Expand ${VAR} environment variables
	expanded := os.ExpandEnv(string(data))

	var cfg GathererConfig
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("parse config yaml: %w", err)
	}

	return &cfg, nil
}

// LoadWithDefaults loads config and applies default values.
func LoadWithDefaults(path string) (*GathererConfig, error) {
	cfg, err := Load(path)
	if err != nil {
		return nil, err
	}
	cfg.applyDefaults()
	return cfg, nil
}

// LoadAndValidate loads config, applies defaults, and validates.
func LoadAndValidate(path string) (*GathererConfig, error) {
	cfg, err := LoadWithDefaults(path)
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return cfg, nil
}
