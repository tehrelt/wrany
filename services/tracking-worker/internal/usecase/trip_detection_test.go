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

// A single isolated jitter point in the middle of real movement must not reset
// the motion candidate. Before the fix any non-moving point sent the state back
// to IDLE, so a genuine walk peppered with GPS noise never started a trip with
// the correct start time.
func TestIsolatedJitterDoesNotResetCandidate(t *testing.T) {
	useCase := NewTripDetectionUseCase(testCfg())
	start := time.Now()

	mk := func(i int, jitter bool) domain.ProcessedLocationPoint {
		p := processedPoint(55.75+float64(i)*0.0002, 37.62, 11,
			1.5, start.Add(time.Duration(i)*12*time.Second))
		if jitter {
			p.DistanceDeltaM = 0
			p.NoiseReason = domain.NoiseJitter
		}
		return p
	}
	var pts []domain.ProcessedLocationPoint
	for i := 0; i < 8; i++ {
		pts = append(pts, mk(i, i == 1)) // point #1 is an isolated jitter
	}

	result := useCase.ProcessBatch(idleState(), pts, start.Add(3*time.Minute))

	assert.Equal(t, domain.StateTripActive, result.NewState.State)
	require.NotEmpty(t, result.Commands)
	assert.Equal(t, CmdCreateTrip, result.Commands[0].Kind)
	assert.Equal(t, pts[0].RecordedAt.UTC(), result.Commands[0].Trip.StartedAt.UTC(),
		"trip must start at the first moving point, not after the jitter reset")
}

// A long time gap (real stop / app killed) between candidate points must abandon
// the unconfirmed candidate instead of carrying a stale start time forward.
func TestLongGapResetsStaleCandidate(t *testing.T) {
	useCase := NewTripDetectionUseCase(testCfg())
	start := time.Now()

	first := processedPoint(55.75, 37.62, 11, 1.5, start)
	// Same place, 30 min later, then a real walk.
	var pts []domain.ProcessedLocationPoint
	pts = append(pts, first)
	for i := 0; i < 6; i++ {
		pts = append(pts, processedPoint(
			55.80+float64(i)*0.0002, 37.62, 11, 1.5,
			start.Add(30*time.Minute).Add(time.Duration(i)*12*time.Second),
		))
	}

	result := useCase.ProcessBatch(idleState(), pts, start.Add(2*time.Hour))

	require.NotEmpty(t, result.Commands)
	require.Equal(t, CmdCreateTrip, result.Commands[0].Kind)
	assert.True(t, result.Commands[0].Trip.StartedAt.After(start.Add(20*time.Minute)),
		"trip must start in the second segment, not at the stale pre-gap point")
}

// A rejected outlier/teleport point (IsAccepted == false) must never contribute
// to trip distance even if it carries a non-zero DistanceDeltaM.
func TestOutlierDoesNotIncreaseTripDistance(t *testing.T) {
	useCase := NewTripDetectionUseCase(testCfg())
	start := time.Now()
	active := useCase.ProcessBatch(idleState(), movement(start, 12), start.Add(time.Minute))
	require.Equal(t, domain.StateTripActive, active.NewState.State)

	moving := processedPoint(55.7513, 37.62, 11, 1.5, start.Add(13*time.Second))

	teleLat, teleLon := 56.75, 37.62
	outlier := domain.ProcessedLocationPoint{
		UserID: testUserID, DeviceID: testDeviceID, EventID: "teleport",
		FilteredLat: &teleLat, FilteredLon: &teleLon, AccuracyM: 10,
		// Pretend a buggy upstream stamped a huge delta; detection must ignore it
		// because the point was not accepted.
		DistanceDeltaM: 9999, IsAccepted: false, IsOutlier: true,
		NoiseReason: domain.NoiseTeleport, RecordedAt: start.Add(14 * time.Second),
	}

	result := useCase.ProcessBatch(active.NewState,
		[]domain.ProcessedLocationPoint{moving, outlier}, start.Add(2*time.Minute))

	assert.Equal(t, domain.StateTripActive, result.NewState.State)
	require.Len(t, result.Commands, 1)
	assert.Equal(t, CmdUpdateTrip, result.Commands[0].Kind)
	assert.Equal(t, 11.0, result.Commands[0].DeltaDistanceM,
		"only the accepted moving point should add distance; outlier excluded")
	assert.Equal(t, 1, result.Commands[0].DeltaPointsCount)
}

// A long silence (app killed / user stopped sending) during an active trip must
// finalize the trip at the last known point, not silently absorb a point that
// arrives much later. Before the fix the trip stayed active across the gap.
func TestLongGapCompletesActiveTrip(t *testing.T) {
	useCase := NewTripDetectionUseCase(testCfg())
	start := time.Now()
	active := useCase.ProcessBatch(idleState(), movement(start, 12), start.Add(time.Minute))
	require.Equal(t, domain.StateTripActive, active.NewState.State)

	// One point 30 min later, standing still.
	far := processedPoint(55.80, 37.62, 11, 0, start.Add(30*time.Minute))
	far.DistanceDeltaM = 0
	result := useCase.ProcessBatch(active.NewState,
		[]domain.ProcessedLocationPoint{far}, start.Add(31*time.Minute))

	var completed bool
	for _, c := range result.Commands {
		if c.Kind == CmdCompleteTrip {
			completed = true
		}
	}
	assert.True(t, completed, "long gap must complete the active trip")
	assert.NotEqual(t, domain.StateTripActive, result.NewState.State)
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
