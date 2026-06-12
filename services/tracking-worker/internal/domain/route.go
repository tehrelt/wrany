package domain

import (
	"time"

	"github.com/google/uuid"
)

type GeoPoint struct {
	Lat float64
	Lon float64
}

type Route struct {
	ID       uuid.UUID
	UserID   uuid.UUID
	DeviceID *uuid.UUID

	Name   *string
	Status string

	StartLat float64
	StartLon float64
	EndLat   float64
	EndLon   float64

	DistanceM  float64
	TripsCount int

	Template []GeoPoint

	FirstTripID uuid.UUID
	LastTripID  uuid.UUID

	CreatedAt time.Time
	UpdatedAt time.Time
}

type RouteTrip struct {
	RouteID     uuid.UUID
	TripID      uuid.UUID
	UserID      uuid.UUID
	DeviceID    uuid.UUID
	MatchScore  float64
	MatchedAt   time.Time
	DurationSec int64
	DistanceM   float64
}

type RouteMatchConfig struct {
	StartRadiusM             float64
	EndRadiusM               float64
	DistanceToleranceRatio   float64
	PathSimilarityThresholdM float64
	MinTripPoints            int
	NormalizePointsN         int
}

func DefaultRouteMatchConfig() RouteMatchConfig {
	return RouteMatchConfig{
		StartRadiusM:             75,
		EndRadiusM:               75,
		DistanceToleranceRatio:   0.25,
		PathSimilarityThresholdM: 50,
		MinTripPoints:            5,
		NormalizePointsN:         50,
	}
}
