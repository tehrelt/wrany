package domain

import (
	"time"

	"github.com/google/uuid"
)

// TripStatus is the stored status of a trips row.
type TripStatus string

const (
	TripStatusActive    TripStatus = "TRIP_ACTIVE"
	TripStatusCompleted TripStatus = "TRIP_COMPLETED"
)

// TripState is the state machine state for detection — persisted per (user, device).
type TripState string

const (
	StateIdle            TripState = "IDLE"
	StateMotionCandidate TripState = "MOTION_CANDIDATE"
	StateTripActive      TripState = "TRIP_ACTIVE"
	StateStopCandidate   TripState = "STOP_CANDIDATE"
)

// Trip represents a detected trip row in the trips table.
type Trip struct {
	ID       uuid.UUID
	UserID   uuid.UUID
	DeviceID uuid.UUID

	Status    TripStatus
	StartedAt time.Time
	EndedAt   *time.Time

	StartLat float64
	StartLon float64
	EndLat   *float64
	EndLon   *float64

	DistanceM   float64
	DurationSec int64
	PointsCount int

	CreatedAt time.Time
	UpdatedAt time.Time
}

// TripPoint links a raw_location_points row to a trip.
type TripPoint struct {
	TripID     uuid.UUID
	UserID     uuid.UUID
	DeviceID   uuid.UUID
	EventID    string
	RecordedAt time.Time
}

// TripDetectionState is the persisted state machine state per (user_id, device_id).
// It survives worker restarts so in-flight trips are not lost.
type TripDetectionState struct {
	UserID   uuid.UUID
	DeviceID uuid.UUID

	State        TripState
	ActiveTripID *uuid.UUID

	// MOTION_CANDIDATE phase: opened on first sustained-motion point.
	CandidateStartedAt    *time.Time
	CandidateStartPointID *string  // event_id of the first candidate point
	CandidateDistanceM    float64  // cumulative distance since candidate start
	CandidateStartLat     *float64 // geographic start of the candidate (= trip start)
	CandidateStartLon     *float64

	// STOP_CANDIDATE phase: opened when the device appears to have halted.
	StopStartedAt *time.Time
	StopCenterLat *float64 // centre of the stop zone (first stop point)
	StopCenterLon *float64

	// Last accepted (non-filtered) point — needed for cross-batch distance and jump detection.
	LastPointLat    *float64
	LastPointLon    *float64
	LastProcessedAt *time.Time // recorded_at of the last accepted point

	// Watermark-based checkpoint.
	// Worker query: recorded_at >= LastWatermarkAt AND recorded_at < now()-LateArrivalWindowSec.
	// After each run: LastWatermarkAt = now()-LateArrivalWindowSec.
	LastWatermarkAt      *time.Time
	LateArrivalWindowSec int

	UpdatedAt time.Time
}

// TripDetectionConfig holds the configurable thresholds for the state machine.
type TripDetectionConfig struct {
	MotionMinDurationSec int     // sustained motion duration to confirm a trip (default 45)
	MotionMinDistanceM   float64 // minimum distance to confirm a trip (default 60)
	MaxAccuracyM         float64 // maximum GPS accuracy radius to accept a point (default 50)
	StopMinDurationSec   int     // minimum stop duration to end a trip (default 180)
	StopRadiusM          float64 // stop-zone radius in metres (default 40)
	MaxSpeedJumpMps      float64 // above this speed a point is a GPS jump (default 83.3 ≈ 300 km/h)
	LateArrivalWindowSec int     // seconds to keep behind now() for the watermark (default 300)
}

// DefaultTripDetectionConfig returns production-ready defaults.
func DefaultTripDetectionConfig() TripDetectionConfig {
	return TripDetectionConfig{
		MotionMinDurationSec: 45,
		MotionMinDistanceM:   60,
		MaxAccuracyM:         50,
		StopMinDurationSec:   180,
		StopRadiusM:          40,
		MaxSpeedJumpMps:      83.3,
		LateArrivalWindowSec: 300,
	}
}
