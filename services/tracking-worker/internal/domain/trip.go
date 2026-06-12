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
	CandidateGoodPoints   int

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
	MotionMinGoodPoints  int
	MovementMinSpeedMps  float64
	MovementMaxSpeedMps  float64
	ActivityConfidence   float64
	StopMinDurationSec   int     // minimum stop duration to end a trip (default 180)
	StopRadiusM          float64 // stop-zone radius in metres (default 40)
	LateArrivalWindowSec int
}

// DefaultTripDetectionConfig returns production-ready defaults.
func DefaultTripDetectionConfig() TripDetectionConfig {
	return TripDetectionConfig{
		MotionMinDurationSec: 45,
		MotionMinDistanceM:   60,
		MotionMinGoodPoints:  3,
		MovementMinSpeedMps:  0.6,
		MovementMaxSpeedMps:  7,
		ActivityConfidence:   0.6,
		StopMinDurationSec:   180,
		StopRadiusM:          40,
		LateArrivalWindowSec: 45,
	}
}

// UserDevicePair identifies a tracked device session.
type UserDevicePair struct {
	UserID   uuid.UUID
	DeviceID uuid.UUID
}

// TripStatsDelta carries incremental stats for an ongoing trip update.
type TripStatsDelta struct {
	TripID     uuid.UUID
	DeltaDistM float64
	DeltaPts   int
	LastPtAt   *time.Time
	LastLat    *float64
	LastLon    *float64
}

// TripCompletion carries the end-of-trip fields.
type TripCompletion struct {
	TripID  uuid.UUID
	EndedAt time.Time
	EndLat  float64
	EndLon  float64
}

// TripDetectionBatch describes all persistence operations for one detection cycle.
// Applied atomically by the repository inside a single transaction.
type TripDetectionBatch struct {
	NewState        TripDetectionState
	ProcessedPoints []ProcessedLocationPoint

	// Trips to INSERT.
	NewTrips []*Trip
	// BackfillSince: for each new trip, the repository must also link raw_location_points
	// with recorded_at in [BackfillSince[tripID], first NewPoint for that trip)
	// into trip_points (ON CONFLICT DO NOTHING for idempotency).
	BackfillSince map[uuid.UUID]time.Time

	// Incremental stats for existing trips.
	UpdatedTrips []TripStatsDelta

	// Trips to mark as TRIP_COMPLETED.
	CompletedTrips []TripCompletion

	// All TripPoints to INSERT (ON CONFLICT DO NOTHING).
	NewPoints []TripPoint
}
