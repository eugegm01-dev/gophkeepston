package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad_RequiredFields(t *testing.T) {
	// Очистить окружение для теста
	_ = os.Unsetenv("DATABASE_DSN")
	_ = os.Unsetenv("JWT_SECRET")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing required fields")
	}
}

func TestLoad_Valid(t *testing.T) {
	_ = os.MkdirAll("certs", 0755)
	_ = os.WriteFile("certs/server.crt", []byte("dummy"), 0644)
	_ = os.WriteFile("certs/server.key", []byte("dummy"), 0644)
	defer os.RemoveAll("certs")
	_ = os.Setenv("DATABASE_DSN", "postgres://test:test@localhost:5432/test?sslmode=disable")
	_ = os.Setenv("JWT_SECRET", "test-secret")
	defer func() {
		_ = os.Unsetenv("DATABASE_DSN")
		_ = os.Unsetenv("JWT_SECRET")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AccessTokenTTL != 15*time.Minute {
		t.Errorf("expected default AccessTokenTTL, got %v", cfg.AccessTokenTTL)
	}
}
