package config

import (
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	// Clear env to test defaults
	os.Unsetenv("PORT")
	os.Unsetenv("REDIS_URL")
	os.Unsetenv("IDLE_TIMEOUT_SECS")

	cfg := Load()

	if cfg.Port != "4000" {
		t.Errorf("expected port 4000, got %s", cfg.Port)
	}
	if cfg.RedisURL != "localhost:6379" {
		t.Errorf("expected localhost:6379, got %s", cfg.RedisURL)
	}
	if cfg.IdleTimeoutSecs != 1800 {
		t.Errorf("expected 1800, got %d", cfg.IdleTimeoutSecs)
	}
	if cfg.SnapshotIntervalSecs != 3600 {
		t.Errorf("expected 3600, got %d", cfg.SnapshotIntervalSecs)
	}
}

func TestConfigFromEnv(t *testing.T) {
	os.Setenv("PORT", "5000")
	os.Setenv("REDIS_URL", "redis.example.com:6379")
	os.Setenv("IDLE_TIMEOUT_SECS", "900")
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("REDIS_URL")
		os.Unsetenv("IDLE_TIMEOUT_SECS")
	}()

	cfg := Load()

	if cfg.Port != "5000" {
		t.Errorf("expected 5000, got %s", cfg.Port)
	}
	if cfg.RedisURL != "redis.example.com:6379" {
		t.Errorf("expected redis.example.com:6379, got %s", cfg.RedisURL)
	}
	if cfg.IdleTimeoutSecs != 900 {
		t.Errorf("expected 900, got %d", cfg.IdleTimeoutSecs)
	}
}
