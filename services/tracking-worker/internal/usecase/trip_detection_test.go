package usecase

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wrany/tracking-worker/internal/domain"
)

var (
	testUserID   = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	testDeviceID = uuid.MustParse("00000000-0000-0000-0000-000000000002")
)

func testCfg() domain.TripDetectionConfig {
	return domain.TripDetectionConfig{
		MotionMinDurationSec: 10, MotionMinDistanceM: 20,
		MotionMinGoodPoints: 3, MovementMinSpeedMps: 0.6,
		MovementMaxSpeedMps: 7, ActivityConfidence: 0.6,
		StopMinDurationSec: 20, StopRadiusM: 40,
		LateArrivalWindowSec: 45,
	}
}

func processedPoint(lat, lon, distance, speed float64, at time.Time) domain.ProcessedLocationPoint {
	return domain.ProcessedLocationPoint{
		UserID: testUserID, DeviceID: testDeviceID, EventID: at.String(),
		FilteredLat: &lat, FilteredLon: &lon, AccuracyM: 10,
		SpeedMps: &speed, ImpliedSpeedMps: speed, DistanceDeltaM: distance,
		IsAccepted: true, ActivityType: "walking", RecordedAt: at, ReceivedAt: at,
	}
}

func idleState() domain.TripDetectionState {
	return domain.TripDetectionState{
		UserID: testUserID, DeviceID: testDeviceID,
		State: domain.StateIdle, LateArrivalWindowSec: 45,
	}
}

func movement(start time.Time, count int) []domain.ProcessedLocationPoint {
	points := make([]domain.ProcessedLocationPoint, count)
	for index := range points {
		points[index] = processedPoint(
			55.75+float64(index)*0.0001, 37.62,
			10, 1.5, start.Add(time.Duration(index)*time.Second),
		)
	}
	return points
}

func TestMovementWindowStartsTrip(t *testing.T) {
	useCase := NewTripDetectionUseCase(testCfg())
	start := time.Now()

	result := useCase.ProcessBatch(idleState(), movement(start, 12), start.Add(time.Minute))

	assert.Equal(t, domain.StateTripActive, result.NewState.State)
	require.NotEmpty(t, result.Commands)
	assert.Equal(t, CmdCreateTrip, result.Commands[0].Kind)
	assert.GreaterOrEqual(t, result.Commands[0].Trip.DistanceM, 20.0)
}

func TestStationaryJitterAddsNoDistance(t *testing.T) {
	useCase := NewTripDetectionUseCase(testCfg())
	start := time.Now()
	active := useCase.ProcessBatch(idleState(), movement(start, 12), start.Add(time.Minute))
	require.Equal(t, domain.StateTripActive, active.NewState.State)

	lat, lon := 55.7512, 37.62
	stopped := domain.ProcessedLocationPoint{
		UserID: testUserID, DeviceID: testDeviceID, EventID: "stop",
		FilteredLat: &lat, FilteredLon: &lon, AccuracyM: 10,
		IsAccepted: true, IsStationary: true, NoiseReason: domain.NoiseStationary,
		RecordedAt: start.Add(15 * time.Second),
	}
	result := useCase.ProcessBatch(active.NewState, []domain.ProcessedLocationPoint{stopped}, start.Add(time.Minute))

	assert.Equal(t, domain.StateStopCandidate, result.NewState.State)
	assert.Empty(t, result.Commands)
}

func TestTrafficLightStopKeepsTripActive(t *testing.T) {
	useCase := NewTripDetectionUseCase(testCfg())
	start := time.Now()
	active := useCase.ProcessBatch(idleState(), movement(start, 12), start.Add(time.Minute))

	lat, lon := 55.7512, 37.62
	speed := 0.0
	paused := domain.ProcessedLocationPoint{
		UserID: testUserID, DeviceID: testDeviceID, EventID: "traffic-light",
		FilteredLat: &lat, FilteredLon: &lon, AccuracyM: 10,
		SpeedMps: &speed, IsAccepted: true, DistanceDeltaM: 0,
		RecordedAt: start.Add(20 * time.Second),
	}
	result := useCase.ProcessBatch(active.NewState, []domain.ProcessedLocationPoint{paused}, start.Add(time.Minute))

	assert.Equal(t, domain.StateTripActive, result.NewState.State)
	require.Len(t, result.Commands, 1)
	assert.Zero(t, result.Commands[0].DeltaDistanceM)
}

func TestLongStopCompletesTrip(t *testing.T) {
	useCase := NewTripDetectionUseCase(testCfg())
	start := time.Now()
	active := useCase.ProcessBatch(idleState(), movement(start, 12), start.Add(time.Minute))

	lat, lon := 55.7512, 37.62
	stop := func(seconds int) domain.ProcessedLocationPoint {
		return domain.ProcessedLocationPoint{
			UserID: testUserID, DeviceID: testDeviceID,
			EventID:     time.Duration(seconds).String(),
			FilteredLat: &lat, FilteredLon: &lon, AccuracyM: 10,
			IsAccepted: true, IsStationary: true,
			RecordedAt: start.Add(time.Duration(seconds) * time.Second),
		}
	}
	candidate := useCase.ProcessBatch(active.NewState, []domain.ProcessedLocationPoint{stop(15)}, start.Add(time.Minute))
	result := useCase.ProcessBatch(candidate.NewState, []domain.ProcessedLocationPoint{stop(36)}, start.Add(2*time.Minute))

	assert.Equal(t, domain.StateIdle, result.NewState.State)
	require.Len(t, result.Commands, 1)
	assert.Equal(t, CmdCompleteTrip, result.Commands[0].Kind)
}

func TestAcceptedLoopKeepsEndpoints(t *testing.T) {
	useCase := NewTripDetectionUseCase(testCfg())
	start := time.Now()
	points := movement(start, 12)
	points[len(points)-1].FilteredLat = points[0].FilteredLat
	points[len(points)-1].FilteredLon = points[0].FilteredLon

	result := useCase.ProcessBatch(idleState(), points, start.Add(time.Minute))

	assert.Equal(t, domain.StateTripActive, result.NewState.State)
	require.NotNil(t, result.Commands[0].Trip)
	assert.InDelta(t, *points[0].FilteredLat, result.Commands[0].Trip.StartLat, 0.000001)
}
