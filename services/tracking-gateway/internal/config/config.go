package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port           string
	DatabaseURL    string
	JWTSecret      string
	JWTAccessTTL   time.Duration
	JWTRefreshTTL  time.Duration
	MigrationsPath string

	// NATS
	NatsURL    string
	NatsStream string

	// WebSocket
	WSMaxMessageSizeBytes int64
	WSReadDeadlineSec     int
	WSWriteDeadlineSec    int
	WSPingIntervalSec     int
	WSMaxBatchSize        int
	// WSAllowedOrigins is a comma-separated list of allowed browser origins.
	// Empty string means only empty-origin (non-browser/mobile) clients are allowed.
	WSAllowedOrigins []string
}

func Load() Config {
	return Config{
		Port:           getEnv("GATEWAY_PORT", "8080"),
		DatabaseURL:    mustEnv("DATABASE_URL"),
		JWTSecret:      mustEnv("JWT_SECRET"),
		JWTAccessTTL:   parseDuration(getEnv("JWT_ACCESS_TTL", "15m")),
		JWTRefreshTTL:  parseDuration(getEnv("JWT_REFRESH_TTL", "168h")),
		MigrationsPath: getEnv("MIGRATIONS_PATH", "./infra/migrations"),

		NatsURL:    getEnv("NATS_URL", ""),
		NatsStream: getEnv("NATS_STREAM", "WRANY_EVENTS"),

		WSMaxMessageSizeBytes: int64(parseInt(getEnv("WS_MAX_MESSAGE_SIZE_BYTES", "262144"))),
		WSReadDeadlineSec:     parseInt(getEnv("WS_READ_DEADLINE_SEC", "60")),
		WSWriteDeadlineSec:    parseInt(getEnv("WS_WRITE_DEADLINE_SEC", "10")),
		WSPingIntervalSec:     parseInt(getEnv("WS_PING_INTERVAL_SEC", "30")),
		WSMaxBatchSize:        parseInt(getEnv("WS_MAX_BATCH_SIZE", "100")),
		WSAllowedOrigins:      parseOrigins(getEnv("WS_ALLOWED_ORIGINS", "")),
	}
}

func parseOrigins(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func parseInt(s string) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		log.Fatalf("invalid integer value %q: %v", s, err)
	}
	return v
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
