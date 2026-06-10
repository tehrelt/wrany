package config

import (
	"log"
	"os"
	"time"
)

type Config struct {
	Port           string
	DatabaseURL    string
	JWTSecret      string
	JWTAccessTTL   time.Duration
	JWTRefreshTTL  time.Duration
	MigrationsPath string
}

func Load() Config {
	return Config{
		Port:           getEnv("GATEWAY_PORT", "8080"),
		DatabaseURL:    mustEnv("DATABASE_URL"),
		JWTSecret:      mustEnv("JWT_SECRET"),
		JWTAccessTTL:   parseDuration(getEnv("JWT_ACCESS_TTL", "15m")),
		JWTRefreshTTL:  parseDuration(getEnv("JWT_REFRESH_TTL", "168h")),
		MigrationsPath: getEnv("MIGRATIONS_PATH", "./infra/migrations"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable %q is not set", key)
	}
	return v
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		log.Fatalf("invalid duration %q: %v", s, err)
	}
	return d
}
