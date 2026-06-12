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

	TripDetectionIntervalSec int
	RouteMatchingIntervalSec int

	GPSGoodAccuracyM            float64
	GPSUsableAccuracyM          float64
	GPSGarbageAccuracyM         float64
	GPSWalkingMaxSpeedMps       float64
	GPSRunningMaxSpeedMps       float64
	GPSBikeMaxSpeedMps          float64
	GPSVehicleMaxSpeedMps       float64
	GPSNoiseMinRadiusM          float64
	GPSNoiseMaxRadiusM          float64
	GPSSmoothingPoints          int
	GPSStationaryWindowSec      int
	GPSStationaryMinDurationSec int
	GPSStationaryRadiusM        float64
	GPSStationaryMaxSpeedMps    float64
	GPSStationaryMinPoints      int
	GPSMovementMinDurationSec   int
	GPSMovementMinDistanceM     float64
	GPSMovementMinSpeedMps      float64
	GPSMovementGoodPoints       int
	GPSActivityConfidence       float64
	GPSLateArrivalWindowSec     int
	TripStopMinDurationSec      int
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
		GPSGoodAccuracyM:            envFloat("GPS_GOOD_ACCURACY_M", 30),
		GPSUsableAccuracyM:          envFloat("GPS_USABLE_ACCURACY_M", 50),
		GPSGarbageAccuracyM:         envFloat("GPS_GARBAGE_ACCURACY_M", 100),
		GPSWalkingMaxSpeedMps:       envFloat("GPS_WALK_MAX_SPEED_MPS", 3.5),
		GPSRunningMaxSpeedMps:       envFloat("GPS_RUN_MAX_SPEED_MPS", 7),
		GPSBikeMaxSpeedMps:          envFloat("GPS_BIKE_MAX_SPEED_MPS", 15),
		GPSVehicleMaxSpeedMps:       envFloat("GPS_VEHICLE_MAX_SPEED_MPS", 60),
		GPSNoiseMinRadiusM:          envFloat("GPS_NOISE_MIN_RADIUS_M", 8),
		GPSNoiseMaxRadiusM:          envFloat("GPS_NOISE_MAX_RADIUS_M", 30),
		GPSSmoothingPoints:          envInt("GPS_SMOOTHING_POINTS", 5),
		GPSStationaryWindowSec:      envInt("GPS_STATIONARY_WINDOW_SEC", 60),
		GPSStationaryMinDurationSec: envInt("GPS_STATIONARY_MIN_DURATION_SEC", 45),
		GPSStationaryRadiusM:        envFloat("GPS_STATIONARY_RADIUS_M", 35),
		GPSStationaryMaxSpeedMps:    envFloat("GPS_STATIONARY_MAX_SPEED_MPS", 0.5),
		GPSStationaryMinPoints:      envInt("GPS_STATIONARY_MIN_POINTS", 4),
		GPSMovementMinDurationSec:   envInt("GPS_MOVEMENT_MIN_DURATION_SEC", 45),
		GPSMovementMinDistanceM:     envFloat("GPS_MOVEMENT_MIN_DISTANCE_M", 60),
		GPSMovementMinSpeedMps:      envFloat("GPS_MOVEMENT_MIN_SPEED_MPS", 0.6),
		GPSMovementGoodPoints:       envInt("GPS_MOVEMENT_GOOD_POINTS", 3),
		GPSActivityConfidence:       envFloat("GPS_ACTIVITY_CONFIDENCE", 0.6),
		GPSLateArrivalWindowSec:     envInt("GPS_LATE_ARRIVAL_WINDOW_SEC", 45),
		TripStopMinDurationSec:      envInt("TRIP_STOP_MIN_DURATION_SEC", 180),
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

func envFloat(key string, def float64) float64 {
	s := os.Getenv(key)
	if s == "" {
		return def
	}
	value, err := strconv.ParseFloat(s, 64)
	if err != nil || value <= 0 {
		return def
	}
	return value
}
