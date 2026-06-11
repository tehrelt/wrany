package usecase

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wrany/tracking-worker/internal/domain"
)

// testCfg returns tight thresholds to keep test point sequences short.
func testCfg() domain.TripDetectionConfig {
	return domain.TripDetectionConfig{
		MotionMinDurationSec: 10,
		MotionMinDistanceM:   20,
		MaxAccuracyM:         30,
		StopMinDurationSec:   20,
		StopRadiusM:          40,
		MaxSpeedJumpMps:      83.3,
		LateArrivalWindowSec: 300,
	}
}

var (
	testUserID   = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	testDeviceID = uuid.MustParse("00000000-0000-0000-0000-000000000002")
)

// makePoint builds a RawLocationPoint at the given time.
// speedMps nil means no sensor speed (speed inferred from distance).
func makePoint(lat, lon, accuracyM float64, speedMps *float64, t time.Time) domain.RawLocationPoint {
	return domain.RawLocationPoint{
		UserID:       testUserID,
		DeviceID:     testDeviceID,
		EventID:      t.Format("20060102T150405.000"),
		RecordedAt:   t,
		ReceivedAt:   t,
		Lat:          lat,
		Lon:          lon,
		AccuracyM:    accuracyM,
		SpeedMps:     speedMps,
		ActivityType: "unknown",
		Source:       "test",
	}
}

func ptr[T any](v T) *T { return &v }

// idleState returns a fresh IDLE state with no previous history.
func idleState() domain.TripDetectionState {
	return domain.TripDetectionState{
		UserID:               testUserID,
		DeviceID:             testDeviceID,
		State:                domain.StateIdle,
		LateArrivalWindowSec: 300,
	}
}

// stateWithLastPoint returns an IDLE state that has a "previous accepted point".
func stateWithLastPoint(lat, lon float64, at time.Time) domain.TripDetectionState {
	s := idleState()
	s.LastPointLat = &lat
	s.LastPointLon = &lon
	s.LastProcessedAt = &at
	return s
}

// movingBatch produces n points separated by dt seconds, each 10 metres north of the previous.
// Starting at (lat, lon) with sensor speed s m/s and accuracy a m.
func movingBatch(n int, lat, lon float64, speedMps float64, accuracyM float64, start time.Time, dt time.Duration) []domain.RawLocationPoint {
	pts := make([]domain.RawLocationPoint, n)
	for i := range pts {
		// ~10 m north per point at roughly 0.0001° per 11 m
		pts[i] = makePoint(lat+float64(i)*0.0001, lon, accuracyM, ptr(speedMps), start.Add(time.Duration(i)*dt))
	}
	return pts
}

// stoppedBatch produces n points at the same position (speed 0).
func stoppedBatch(n int, lat, lon float64, accuracyM float64, start time.Time, dt time.Duration) []domain.RawLocationPoint {
	pts := make([]domain.RawLocationPoint, n)
	for i := range pts {
		pts[i] = makePoint(lat, lon, accuracyM, ptr(0.0), start.Add(time.Duration(i)*dt))
	}
	return pts
}

// --- Tests ---

func TestIDLE_StayIdleOnStoppedPoint(t *testing.T) {
	uc := NewTripDetectionUseCase(testCfg())
	base := time.Now()
	pts := []domain.RawLocationPoint{makePoint(10, 20, 10, ptr(0.0), base)}

	r := uc.ProcessBatch(idleState(), pts, base.Add(time.Minute))

	assert.Equal(t, domain.StateIdle, r.NewState.State)
	assert.Empty(t, r.Commands)
}

func TestIDLE_TransitionToMotionCandidateOnMovingPoint(t *testing.T) {
	uc := NewTripDetectionUseCase(testCfg())
	base := time.Now()
	pts := []domain.RawLocationPoint{makePoint(10, 20, 10, ptr(2.0), base)}

	r := uc.ProcessBatch(idleState(), pts, base.Add(time.Minute))

	assert.Equal(t, domain.StateMotionCandidate, r.NewState.State)
	assert.NotNil(t, r.NewState.CandidateStartedAt)
	assert.Equal(t, base.Unix(), r.NewState.CandidateStartedAt.Unix())
	assert.Empty(t, r.Commands)
}

func TestMotionCandidate_ResetToIdleOnStop(t *testing.T) {
	uc := NewTripDetectionUseCase(testCfg())
	base := time.Now()

	// Enter MOTION_CANDIDATE with one moving point.
	r1 := uc.ProcessBatch(idleState(), []domain.RawLocationPoint{
		makePoint(10, 20, 10, ptr(2.0), base),
	}, base.Add(time.Minute))
	require.Equal(t, domain.StateMotionCandidate, r1.NewState.State)

	// Stopped point: should reset to IDLE.
	r2 := uc.ProcessBatch(r1.NewState, []domain.RawLocationPoint{
		makePoint(10, 20, 10, ptr(0.0), base.Add(5*time.Second)),
	}, base.Add(time.Minute))

	assert.Equal(t, domain.StateIdle, r2.NewState.State)
	assert.Nil(t, r2.NewState.CandidateStartedAt)
	assert.Empty(t, r2.Commands)
}

func TestMotionCandidate_StaysInCandidateWhenThresholdNotMet(t *testing.T) {
	uc := NewTripDetectionUseCase(testCfg())
	base := time.Now()

	// 5 s of movement, ~5 m — below both 10 s and 20 m thresholds.
	pts := movingBatch(3, 10, 20, 2.0, 10, base, 2*time.Second)
	r := uc.ProcessBatch(idleState(), pts, base.Add(time.Minute))

	assert.Equal(t, domain.StateMotionCandidate, r.NewState.State)
	assert.Empty(t, r.Commands)
}

func TestMotionCandidate_TransitionsToTripActiveWhenThresholdMet(t *testing.T) {
	uc := NewTripDetectionUseCase(testCfg())
	base := time.Now()

	// 11 points × 1 s = 10 s duration, ~110 m — exactly meets 10 s / 20 m thresholds.
	// The 11th point (index 10) triggers the transition; no points remain for TRIP_ACTIVE,
	// so exactly one command is emitted.
	pts := movingBatch(11, 10, 20, 11.0, 10, base, time.Second)
	r := uc.ProcessBatch(idleState(), pts, base.Add(time.Minute))

	assert.Equal(t, domain.StateTripActive, r.NewState.State)
	require.Len(t, r.Commands, 1)
	assert.Equal(t, CmdCreateTrip, r.Commands[0].Kind)
	require.NotNil(t, r.Commands[0].Trip)
	assert.Equal(t, domain.TripStatusActive, r.Commands[0].Trip.Status)
	assert.NotEmpty(t, r.Commands[0].Points)
	assert.NotNil(t, r.Commands[0].BackfillSince)
}

func TestTripActive_EmitsCmdUpdateOnMovingPoints(t *testing.T) {
	uc := NewTripDetectionUseCase(testCfg())
	base := time.Now()

	// Start a trip.
	pts1 := movingBatch(12, 10, 20, 11.0, 10, base, time.Second)
	r1 := uc.ProcessBatch(idleState(), pts1, base.Add(time.Minute))
	require.Equal(t, domain.StateTripActive, r1.NewState.State)

	// Continue trip with more moving points.
	pts2 := movingBatch(5, 10.0012, 20, 11.0, 10, base.Add(20*time.Second), time.Second)
	r2 := uc.ProcessBatch(r1.NewState, pts2, base.Add(2*time.Minute))

	assert.Equal(t, domain.StateTripActive, r2.NewState.State)
	// Should have exactly one CmdUpdateTrip for this batch (create was in r1).
	require.Len(t, r2.Commands, 1)
	assert.Equal(t, CmdUpdateTrip, r2.Commands[0].Kind)
	assert.Equal(t, *r1.NewState.ActiveTripID, r2.Commands[0].TripID)
	assert.Len(t, r2.Commands[0].Points, 5)
}

func TestTripActive_TransitionsToStopCandidateOnStop(t *testing.T) {
	uc := NewTripDetectionUseCase(testCfg())
	base := time.Now()

	// Start trip.
	pts1 := movingBatch(12, 10, 20, 11.0, 10, base, time.Second)
	r1 := uc.ProcessBatch(idleState(), pts1, base.Add(time.Minute))
	require.Equal(t, domain.StateTripActive, r1.NewState.State)

	// One stopped point.
	stopPt := makePoint(10.0012, 20, 10, ptr(0.0), base.Add(15*time.Second))
	r2 := uc.ProcessBatch(r1.NewState, []domain.RawLocationPoint{stopPt}, base.Add(time.Minute))

	assert.Equal(t, domain.StateStopCandidate, r2.NewState.State)
	assert.NotNil(t, r2.NewState.StopStartedAt)
}

func TestStopCandidate_ResumesToTripActiveOnMovement(t *testing.T) {
	uc := NewTripDetectionUseCase(testCfg())
	base := time.Now()

	// Start trip.
	pts1 := movingBatch(12, 10, 20, 11.0, 10, base, time.Second)
	r1 := uc.ProcessBatch(idleState(), pts1, base.Add(time.Minute))

	// Enter STOP_CANDIDATE.
	stopPt := makePoint(10.0012, 20, 10, ptr(0.0), base.Add(15*time.Second))
	r2 := uc.ProcessBatch(r1.NewState, []domain.RawLocationPoint{stopPt}, base.Add(time.Minute))
	require.Equal(t, domain.StateStopCandidate, r2.NewState.State)

	// Move far outside stop radius (>40 m).
	resumePt := makePoint(10.003, 20, 10, ptr(11.0), base.Add(18*time.Second))
	r3 := uc.ProcessBatch(r2.NewState, []domain.RawLocationPoint{resumePt}, base.Add(2*time.Minute))

	assert.Equal(t, domain.StateTripActive, r3.NewState.State)
	assert.Nil(t, r3.NewState.StopStartedAt)
}

func TestStopCandidate_StaysInStopCandidateDuringShortStop(t *testing.T) {
	uc := NewTripDetectionUseCase(testCfg())
	base := time.Now()

	// Start trip.
	r1 := uc.ProcessBatch(idleState(), movingBatch(12, 10, 20, 11.0, 10, base, time.Second), base.Add(time.Minute))

	// Enter stop.
	r2 := uc.ProcessBatch(r1.NewState, stoppedBatch(1, 10.0012, 20, 10, base.Add(15*time.Second), time.Second), base.Add(time.Minute))
	require.Equal(t, domain.StateStopCandidate, r2.NewState.State)

	// 10 s of stopped points — below StopMinDurationSec=20.
	r3 := uc.ProcessBatch(r2.NewState, stoppedBatch(5, 10.0012, 20, 10, base.Add(16*time.Second), 2*time.Second), base.Add(2*time.Minute))

	assert.Equal(t, domain.StateStopCandidate, r3.NewState.State)
	assert.Empty(t, r3.Commands)
}

func TestStopCandidate_CompletesTripAfterLongStop(t *testing.T) {
	uc := NewTripDetectionUseCase(testCfg())
	base := time.Now()

	// Start trip.
	r1 := uc.ProcessBatch(idleState(), movingBatch(12, 10, 20, 11.0, 10, base, time.Second), base.Add(time.Minute))
	tripID := *r1.NewState.ActiveTripID

	// Enter stop.
	r2 := uc.ProcessBatch(r1.NewState, stoppedBatch(1, 10.0012, 20, 10, base.Add(15*time.Second), time.Second), base.Add(time.Minute))
	require.Equal(t, domain.StateStopCandidate, r2.NewState.State)

	// 25 s of stopped points — exceeds StopMinDurationSec=20.
	r3 := uc.ProcessBatch(r2.NewState, stoppedBatch(6, 10.0012, 20, 10, base.Add(16*time.Second), 5*time.Second), base.Add(2*time.Minute))

	assert.Equal(t, domain.StateIdle, r3.NewState.State)
	assert.Nil(t, r3.NewState.ActiveTripID)

	require.Len(t, r3.Commands, 1)
	assert.Equal(t, CmdCompleteTrip, r3.Commands[0].Kind)
	assert.Equal(t, tripID, r3.Commands[0].TripID)
	assert.NotNil(t, r3.Commands[0].EndedAt)
	assert.NotNil(t, r3.Commands[0].EndLat)
	assert.NotNil(t, r3.Commands[0].EndLon)
}

func TestBadAccuracyPointSkipped(t *testing.T) {
	uc := NewTripDetectionUseCase(testCfg())
	base := time.Now()
	// Accuracy 50 m > MaxAccuracyM=30 → should be skipped.
	pts := []domain.RawLocationPoint{makePoint(10, 20, 50, ptr(5.0), base)}

	r := uc.ProcessBatch(idleState(), pts, base.Add(time.Minute))

	assert.Equal(t, domain.StateIdle, r.NewState.State)
	assert.Nil(t, r.NewState.LastPointLat) // skipped point must not update last-point
}

func TestGPSJumpIgnored(t *testing.T) {
	uc := NewTripDetectionUseCase(testCfg())
	base := time.Now()

	// Establish a previous point at (0, 0).
	state := stateWithLastPoint(0, 0, base)

	// Jump point 500 km north in 1 second ≈ 500 000 m/s >> 83.3 m/s cap.
	jumpPt := makePoint(4.5, 0, 10, nil, base.Add(time.Second))
	r := uc.ProcessBatch(state, []domain.RawLocationPoint{jumpPt}, base.Add(time.Minute))

	assert.Equal(t, domain.StateIdle, r.NewState.State)
	assert.Equal(t, ptr(float64(0)), r.NewState.LastPointLat) // unchanged
}

func TestDuplicatePointIgnored(t *testing.T) {
	uc := NewTripDetectionUseCase(testCfg())
	base := time.Now()

	state := stateWithLastPoint(10, 20, base)

	// Point at the same time as LastProcessedAt.
	dupPt := makePoint(10, 20, 10, ptr(2.0), base)
	r := uc.ProcessBatch(state, []domain.RawLocationPoint{dupPt}, base.Add(time.Minute))

	assert.Equal(t, domain.StateIdle, r.NewState.State)
	assert.Empty(t, r.Commands)
}

func TestIdempotentReprocessing(t *testing.T) {
	uc := NewTripDetectionUseCase(testCfg())
	base := time.Now()

	// Full trip: start, active, complete.
	pts1 := movingBatch(12, 10, 20, 11.0, 10, base, time.Second)
	r1 := uc.ProcessBatch(idleState(), pts1, base.Add(time.Minute))
	require.Equal(t, domain.StateTripActive, r1.NewState.State)

	stopPts := stoppedBatch(6, 10.0012, 20, 10, base.Add(15*time.Second), 5*time.Second)
	r2 := uc.ProcessBatch(r1.NewState, stopPts, base.Add(2*time.Minute))
	require.Equal(t, domain.StateIdle, r2.NewState.State)

	// Re-process the same stop batch: all points have recorded_at <= LastProcessedAt → skipped.
	r3 := uc.ProcessBatch(r2.NewState, stopPts, base.Add(3*time.Minute))

	assert.Equal(t, domain.StateIdle, r3.NewState.State)
	assert.Empty(t, r3.Commands) // no duplicate commands
}

func TestLatePointHandledSafely(t *testing.T) {
	uc := NewTripDetectionUseCase(testCfg())
	base := time.Now()

	// Establish processed state at base+10s.
	state := stateWithLastPoint(10, 20, base.Add(10*time.Second))

	// Late point at base+5s (before LastProcessedAt) → must be skipped.
	latePt := makePoint(10.001, 20, 10, ptr(5.0), base.Add(5*time.Second))
	r := uc.ProcessBatch(state, []domain.RawLocationPoint{latePt}, base.Add(time.Minute))

	assert.Equal(t, domain.StateIdle, r.NewState.State)
	// LastProcessedAt must not regress.
	require.NotNil(t, r.NewState.LastProcessedAt)
	assert.Equal(t, base.Add(10*time.Second).Unix(), r.NewState.LastProcessedAt.Unix())
}

func TestWatermarkAdvancedAfterBatch(t *testing.T) {
	uc := NewTripDetectionUseCase(testCfg())
	base := time.Now()
	now := base.Add(10 * time.Minute)

	r := uc.ProcessBatch(idleState(), nil, now)

	require.NotNil(t, r.NewState.LastWatermarkAt)
	expected := now.Add(-300 * time.Second)
	assert.WithinDuration(t, expected, *r.NewState.LastWatermarkAt, time.Second)
}
