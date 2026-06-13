package domain

import (
	"time"

	"github.com/google/uuid"
)

// CurrentAlgorithmVersion is the version stamped on every processing result.
// Bump it whenever the noise/filtering algorithm changes in a way that should
// trigger reprocessing of previously stored processed_location_points rows.
// v2: distance/jitter measured on the raw track (not the smoothed one) and
// distance is not attributed across a SegmentMaxGapSec time gap.
const CurrentAlgorithmVersion int16 = 2

type NoiseReason string

const (
	NoiseNone            NoiseReason = ""
	NoisePoorAccuracy    NoiseReason = "poor_accuracy"
	NoiseGarbageAccuracy NoiseReason = "garbage_accuracy"
	NoiseInvalidPoint    NoiseReason = "invalid_point"
	NoiseTeleport        NoiseReason = "teleport"
	NoiseJitter          NoiseReason = "jitter"
	NoiseStationary      NoiseReason = "stationary"
	NoiseLateArrival     NoiseReason = "late_arrival"
	// NoiseSegmentBreak marks an accepted point that follows a long time gap.
	// Its position is valid but it starts a new segment, so no distance is
	// attributed between the previous point and this one.
	NoiseSegmentBreak NoiseReason = "segment_break"
)

type ProcessedLocationPoint struct {
	UserID   uuid.UUID
	DeviceID uuid.UUID
	EventID  string

	RawLat float64
	RawLon float64

	FilteredLat *float64
	FilteredLon *float64

	AccuracyM       float64
	SpeedMps        *float64
	ImpliedSpeedMps float64
	DistanceDeltaM  float64

	ActivityType       string
	ActivityConfidence *float64

	IsAccepted      bool
	IsOutlier       bool
	IsStationary    bool
	NoiseReason     NoiseReason
	StationarySince *time.Time

	AlgorithmVersion int16

	RecordedAt  time.Time
	ReceivedAt  time.Time
	ProcessedAt time.Time
}

func (p ProcessedLocationPoint) Coordinates() (float64, float64, bool) {
	if p.FilteredLat == nil || p.FilteredLon == nil {
		return 0, 0, false
	}
	return *p.FilteredLat, *p.FilteredLon, true
}

func (p ProcessedLocationPoint) IsMovementEvidence(minSpeed, maxSpeed, minConfidence float64) bool {
	if !p.IsAccepted || p.IsStationary || p.DistanceDeltaM <= 0 {
		return false
	}
	speed := p.ImpliedSpeedMps
	if p.SpeedMps != nil {
		speed = *p.SpeedMps
	}
	speedMatches := speed >= minSpeed && speed <= maxSpeed
	activityMatches := p.ActivityType == "walking" || p.ActivityType == "running"
	if p.ActivityConfidence != nil {
		activityMatches = activityMatches && *p.ActivityConfidence >= minConfidence
	}
	return activityMatches || speedMatches
}

type NoiseConfig struct {
	GoodAccuracyM            float64
	UsableAccuracyM          float64
	GarbageAccuracyM         float64
	WalkingMaxSpeedMps       float64
	RunningMaxSpeedMps       float64
	BikeMaxSpeedMps          float64
	VehicleMaxSpeedMps       float64
	NoiseMinRadiusM          float64
	NoiseMaxRadiusM          float64
	SmoothingPoints          int
	StationaryWindowSec      int
	StationaryMinDurationSec int
	StationaryRadiusM        float64
	StationaryMaxSpeedMps    float64
	StationaryMinPoints      int
	MovementMinSpeedMps      float64
	MovementGoodPoints       int
	ActivityConfidence       float64
	LateArrivalWindowSec     int
	// SegmentMaxGapSec is the maximum time gap between two consecutive accepted
	// points for them to be treated as one continuous track. Beyond it the points
	// belong to different segments: distance is NOT attributed across the gap and
	// smoothing does not mix the two sides.
	SegmentMaxGapSec int
}

func DefaultNoiseConfig() NoiseConfig {
	return NoiseConfig{
		GoodAccuracyM: 30, UsableAccuracyM: 50, GarbageAccuracyM: 100,
		WalkingMaxSpeedMps: 3.5, RunningMaxSpeedMps: 7,
		BikeMaxSpeedMps: 15, VehicleMaxSpeedMps: 60,
		NoiseMinRadiusM: 8, NoiseMaxRadiusM: 30, SmoothingPoints: 5,
		StationaryWindowSec: 60, StationaryMinDurationSec: 45,
		StationaryRadiusM: 35, StationaryMaxSpeedMps: 0.5,
		StationaryMinPoints: 4, MovementMinSpeedMps: 0.6,
		MovementGoodPoints: 3, ActivityConfidence: 0.6,
		LateArrivalWindowSec: 45, SegmentMaxGapSec: 120,
	}
}
