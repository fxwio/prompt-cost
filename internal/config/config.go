// Package config loads YAML configuration with environment variable resolution.
package config

import (
	"fmt"
	"os"
	"time"

	yaml "go.yaml.in/yaml/v2"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Postgres PostgresConfig `yaml:"postgres"`
	Pricing  PricingConfig  `yaml:"pricing"`
}

type ServerConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

type PostgresConfig struct {
	DSNEnv   string `yaml:"dsn_env"`
	DSN      string `yaml:"-"`
	MaxConns int32  `yaml:"max_conns"`
	MinConns int32  `yaml:"min_conns"`
}

// PricingConfig allows overriding built-in model prices via config.
type PricingConfig struct {
	Overrides []PricingOverride `yaml:"overrides"`
}

type PricingOverride struct {
	Model       string  `yaml:"model"`
	InputPer1M  float64 `yaml:"input_per_1m_usd"`
	OutputPer1M float64 `yaml:"output_per_1m_usd"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.resolve(); err != nil {
		return nil, err
	}
	cfg.setDefaults()
	return &cfg, nil
}

func (c *Config) resolve() error {
	if c.Postgres.DSNEnv != "" {
		c.Postgres.DSN = os.Getenv(c.Postgres.DSNEnv)
		if c.Postgres.DSN == "" {
			return fmt.Errorf("env %q is not set", c.Postgres.DSNEnv)
		}
	}
	return nil
}

func (c *Config) setDefaults() {
	if c.Server.Port == 0 {
		c.Server.Port = 8092
	}
	if c.Server.ReadTimeout == 0 {
		c.Server.ReadTimeout = 30 * time.Second
	}
	if c.Server.WriteTimeout == 0 {
		c.Server.WriteTimeout = 60 * time.Second
	}
	if c.Server.ShutdownTimeout == 0 {
		c.Server.ShutdownTimeout = 10 * time.Second
	}
	if c.Postgres.MaxConns == 0 {
		c.Postgres.MaxConns = 10
	}
	if c.Postgres.MinConns == 0 {
		c.Postgres.MinConns = 2
	}
}
