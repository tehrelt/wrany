package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all runtime configuration for tracking-worker.
// All fields are populated from environment variables; required fields
// cause Load to return an error when absent.
type Config struct {
	Port        string
	DatabaseURL string

	NatsURL                     string
	NatsStream                  string
	NatsLocationSubject         string
	NatsLocationConsumerDurable string
	NatsConsumerAckWaitSec      int
	NatsConsumerMaxDeliver      int
	NatsConsumerBatchSize       int
	NatsConsumerPollTimeoutSec  int

	TripDetectionIntervalSec  int
	RouteMatchingIntervalSec  int
}

// Load reads configuration from environment variables.
// Returns an error if any required variable is missing.
func Load() (Config, error) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		return Config{}, fmt.Errorf("config: NATS_URL is required")
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return Config{}, fmt.Errorf("config: DATABASE_URL is required")
	}

	return Config{
		Port:                        envDefault("WORKER_PORT", "8081"),
		DatabaseURL:                 dbURL,
		NatsURL:                     natsURL,
		NatsStream:                  envDefault("NATS_STREAM", "WRANY_EVENTS"),
		NatsLocationSubject:         envDefault("NATS_LOCATION_SUBJECT", "location.events.v1"),
		NatsLocationConsumerDurable: envDefault("NATS_LOCATION_CONSUMER_DURABLE", "tracking-worker-location-consumer"),
		NatsConsumerAckWaitSec:      envInt("NATS_CONSUMER_ACK_WAIT_SEC", 30),
		NatsConsumerMaxDeliver:      envInt("NATS_CONSUMER_MAX_DELIVER", 5),
		NatsConsumerBatchSize:       envInt("NATS_CONSUMER_BATCH_SIZE", 100),
		NatsConsumerPollTimeoutSec:  envInt("NATS_CONSUMER_POLL_TIMEOUT_SEC", 5),
		TripDetectionIntervalSec:    envInt("TRIP_DETECTION_INTERVAL_SEC", 30),
		RouteMatchingIntervalSec:    envInt("ROUTE_MATCHING_INTERVAL_SEC", 60),
	}, nil
}

func envDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	s := os.Getenv(key)
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return def
	}
	return v
}
