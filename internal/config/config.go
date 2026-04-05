package config

import "os"

type Config struct {
	Port                 string
	RedisURL             string
	IdleTimeoutSecs      int
	HealthCheckInterval  int
	SnapshotIntervalSecs int
}

func Load() *Config {
	return &Config{
		Port:                 envOr("PORT", "4000"),
		RedisURL:             envOr("REDIS_URL", "localhost:6379"),
		IdleTimeoutSecs:      envInt("IDLE_TIMEOUT_SECS", 1800),
		HealthCheckInterval:  envInt("HEALTH_CHECK_INTERVAL", 10),
		SnapshotIntervalSecs: envInt("SNAPSHOT_INTERVAL_SECS", 3600),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n := 0
	for _, c := range v {
		n = n*10 + int(c-'0')
	}
	return n
}
