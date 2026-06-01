// internal/config/config.go
package config

import (
	"fmt"
	"os"
	"time"

	"github.com/caarlos0/env/v10"
)

type Config struct {
	// Server
	ServerAddr string `env:"SERVER_ADDR" envDefault:":50051"`

	// Database
	DatabaseDSN string `env:"DATABASE_DSN" envRequired:"true"`

	// JWT
	JWTSecret       string        `env:"JWT_SECRET" envRequired:"true"`
	AccessTokenTTL  time.Duration `env:"JWT_ACCESS_TTL" envDefault:"15m"`
	RefreshTokenTTL time.Duration `env:"JWT_REFRESH_TTL" envDefault:"72h"`

	// TLS
	TLSCertPath string `env:"TLS_CERT_PATH" envDefault:"certs/server.crt"`
	TLSKeyPath  string `env:"TLS_KEY_PATH" envDefault:"certs/server.key"`
	TLSEnabled  bool   `env:"TLS_ENABLED" envDefault:"true"`
	// Sync
	SyncPolicy    string        `env:"SYNC_POLICY" envDefault:"last-write-wins"` // last-write-wins | server-authoritative | client-authoritative
	SyncRetryMax  int           `env:"SYNC_RETRY_MAX" envDefault:"3"`
	SyncRetryBase time.Duration `env:"SYNC_RETRY_BASE" envDefault:"100ms"`
}

func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parse env: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return cfg, nil
}

func (c *Config) Validate() error {
	if c.DatabaseDSN == "" {
		return fmt.Errorf("DATABASE_DSN is required")
	}
	if c.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}
	if c.AccessTokenTTL <= 0 {
		return fmt.Errorf("JWT_ACCESS_TTL must be positive")
	}
	if c.RefreshTokenTTL <= 0 {
		return fmt.Errorf("JWT_REFRESH_TTL must be positive")
	}
	if c.TLSEnabled {
		if _, err := os.Stat(c.TLSCertPath); err != nil {
			return fmt.Errorf("TLS_CERT_PATH invalid: %w", err)
		}
		if _, err := os.Stat(c.TLSKeyPath); err != nil {
			return fmt.Errorf("TLS_KEY_PATH invalid: %w", err)
		}
	}
	return nil
}
