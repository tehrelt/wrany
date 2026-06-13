package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wrany/tracking-worker/internal/domain"
	"github.com/wrany/tracking-worker/internal/storage/postgres"
)

// ---- helpers ----

func newPair() (uuid.UUID, uuid.UUID) {
	return uuid.New(), uuid.New()
}

func now() time.Time { return time.Now().UTC().Truncate(time.Microsecond) }

func makeRawPoint(userID, deviceID uuid.UUID, recordedAt time.Time, speedMps float64) domain.RawLocationPoint {
	sp := speedMps
	return domain.RawLocationPoint{
		UserID:     userID,
		DeviceID:   deviceID,
		EventID:    uuid.New().String(),
		RecordedAt: recordedAt,
		ReceivedAt: recordedAt,
		Lat:        55.7558,
		Lon:        37.6173,
		AccuracyM:  10,
		SpeedMps:   &sp,
		Source:     "test",
	}
}

// seedPoints inserts n raw points via RawLocationRepo at 1-second intervals.
func seedPoints(t *testing.T, repo *postgres.RawLocationRepo, userID, deviceID uuid.UUID, base time.Time, n int, speedMps float64) []domain.RawLocationPoint {
	t.Helper()
	ctx := context.Background()
	pts := make([]domain.RawLocationPoint, n)
	for i := 0; i < n; i++ {
		p := makeRawPoint(userID, deviceID, base.Add(time.Duration(i)*time.Second), speedMps)
		require.NoError(t, repo.Insert(ctx, p))
		pts[i] = p
	}
	return pts
}

// ---- tests ----

func TestLoadDistinctUserDevicePairs_NoState(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	rawRepo := postgres.NewRawLocationRepo(db)
	tripRepo := postgres.NewTripRepo(db)
	ctx := context.Background()

	userID, deviceID := newPair()
	base := now().Add(-10 * time.Minute)
	boundary := now().Add(-5 * time.Minute)

	seedPoints(t, rawRepo, userID, deviceID, base, 3, 5.0)

	pairs, err := tripRepo.LoadDistinctUserDevicePairs(ctx, boundary)
	require.NoError(t, err)

	found := false
	for _, p := range pairs {
		if p.UserID == userID && p.DeviceID == deviceID {
			found = true
		}
	}
	assert.True(t, found, "pair with unprocessed points must be returned")
}

func TestLoadDistinctUserDevicePairs_PointsAfterBoundary_NotReturned(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	rawRepo := postgres.NewRawLocationRepo(db)
	tripRepo := postgres.NewTripRepo(db)
	ctx := context.Background()

	userID, deviceID := newPair()
	// Points in the future relative to boundary.
	base := now().Add(10 * time.Minute)
	boundary := now()

	seedPoints(t, rawRepo, userID, deviceID, base, 3, 5.0)

	pairs, err := tripRepo.LoadDistinctUserDevicePairs(ctx, boundary)
	require.NoError(t, err)

	for _, p := range pairs {
		assert.False(t, p.UserID == userID && p.DeviceID == deviceID,
			"pair with points after boundary must not be returned")
	}
}

func TestLoadState_DefaultWhenMissing(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	tripRepo := postgres.NewTripRepo(db)
	ctx := context.Background()

	userID, deviceID := newPair()
	state, err := tripRepo.LoadState(ctx, userID, deviceID)
	require.NoError(t, err)

	assert.Equal(t, domain.StateIdle, state.State)
	assert.Equal(t, 45, state.LateArrivalWindowSec)
	assert.Equal(t, userID, state.UserID)
	assert.Equal(t, deviceID, state.DeviceID)
	assert.Nil(t, state.ActiveTripID)
}

func TestLoadState_ReturnsPersistedState(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	tripRepo := postgres.NewTripRepo(db)
	ctx := context.Background()

	userID, deviceID := newPair()
	tripID := uuid.New()
	t0 := now()

	// Insert a trip so FK constraint on active_trip_id is satisfied.
	trip := &domain.Trip{
		ID:        tripID,
		UserID:    userID,
		DeviceID:  deviceID,
		Status:    domain.TripStatusActive,
		StartedAt: t0.Add(-2 * time.Minute),
		StartLat:  55.7558,
		StartLon:  37.6173,
		CreatedAt: t0,
		UpdatedAt: t0,
	}

	state := domain.TripDetectionState{
		UserID:               userID,
		DeviceID:             deviceID,
		State:                domain.StateTripActive,
		ActiveTripID:         &tripID,
		LateArrivalWindowSec: 600,
		UpdatedAt:            t0,
	}
	watermark := t0.Add(-10 * time.Minute)
	state.LastWatermarkAt = &watermark

	batch := domain.TripDetectionBatch{
		NewState: state,
		NewTrips: []*domain.Trip{trip},
	}
	require.NoError(t, tripRepo.ApplyBatch(ctx, batch))

	loaded, err := tripRepo.LoadState(ctx, userID, deviceID)
	require.NoError(t, err)

	assert.Equal(t, domain.StateTripActive, loaded.State)
	require.NotNil(t, loaded.ActiveTripID)
	assert.Equal(t, tripID, *loaded.ActiveTripID)
	assert.Equal(t, 600, loaded.LateArrivalWindowSec)
}

func TestFetchPoints_ReturnsWindowPoints(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	rawRepo := postgres.NewRawLocationRepo(db)
	tripRepo := postgres.NewTripRepo(db)
	ctx := context.Background()

	userID, deviceID := newPair()
	base := now().Add(-20 * time.Minute)

	all := seedPoints(t, rawRepo, userID, deviceID, base, 10, 5.0)

	from := base.Add(2 * time.Second)
	to := base.Add(7 * time.Second)

	pts, err := tripRepo.FetchPoints(ctx, userID, deviceID, from, to)
	require.NoError(t, err)

	// Expect points at +2s, +3s, +4s, +5s, +6s → 5 points.
	assert.Len(t, pts, 5)
	for _, p := range pts {
		assert.Equal(t, userID, p.UserID)
		assert.Equal(t, deviceID, p.DeviceID)
		assert.True(t, !p.RecordedAt.Before(from), "recorded_at must be >= from")
		assert.True(t, p.RecordedAt.Before(to), "recorded_at must be < to")
	}
	_ = all
}

func TestFetchPoints_OrderedAsc(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	rawRepo := postgres.NewRawLocationRepo(db)
	tripRepo := postgres.NewTripRepo(db)
	ctx := context.Background()

	userID, deviceID := newPair()
	base := now().Add(-10 * time.Minute)
	seedPoints(t, rawRepo, userID, deviceID, base, 5, 5.0)

	pts, err := tripRepo.FetchPoints(ctx, userID, deviceID, base, base.Add(10*time.Second))
	require.NoError(t, err)
	require.Len(t, pts, 5)

	for i := 1; i < len(pts); i++ {
		assert.True(t, !pts[i].RecordedAt.Before(pts[i-1].RecordedAt),
			"points must be ordered ASC by recorded_at")
	}
}

func TestApplyBatch_InsertTrip(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	tripRepo := postgres.NewTripRepo(db)
	ctx := context.Background()

	userID, deviceID := newPair()
	tripID := uuid.New()
	t0 := now()

	trip := &domain.Trip{
		ID:        tripID,
		UserID:    userID,
		DeviceID:  deviceID,
		Status:    domain.TripStatusActive,
		StartedAt: t0.Add(-5 * time.Minute),
		StartLat:  55.7558,
		StartLon:  37.6173,
		CreatedAt: t0,
		UpdatedAt: t0,
	}
	state := domain.TripDetectionState{
		UserID:               userID,
		DeviceID:             deviceID,
		State:                domain.StateTripActive,
		ActiveTripID:         &tripID,
		LateArrivalWindowSec: 300,
		UpdatedAt:            t0,
	}

	batch := domain.TripDetectionBatch{
		NewState: state,
		NewTrips: []*domain.Trip{trip},
	}
	require.NoError(t, tripRepo.ApplyBatch(ctx, batch))

	var status string
	err := db.QueryRow(ctx, "SELECT status FROM trips WHERE id = $1", tripID).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "TRIP_ACTIVE", status)
}

func TestApplyBatch_UpdateTripStats(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	tripRepo := postgres.NewTripRepo(db)
	ctx := context.Background()

	userID, deviceID := newPair()
	tripID := uuid.New()
	t0 := now()

	trip := &domain.Trip{
		ID:        tripID,
		UserID:    userID,
		DeviceID:  deviceID,
		Status:    domain.TripStatusActive,
		StartedAt: t0.Add(-5 * time.Minute),
		StartLat:  55.7558,
		StartLon:  37.6173,
		CreatedAt: t0,
		UpdatedAt: t0,
	}
	state := domain.TripDetectionState{
		UserID:               userID,
		DeviceID:             deviceID,
		State:                domain.StateTripActive,
		ActiveTripID:         &tripID,
		LateArrivalWindowSec: 300,
		UpdatedAt:            t0,
	}
	// First batch: create trip.
	require.NoError(t, tripRepo.ApplyBatch(ctx, domain.TripDetectionBatch{
		NewState: state,
		NewTrips: []*domain.Trip{trip},
	}))

	// Second batch: update stats.
	lastAt := t0.Add(-2 * time.Minute)
	lastLat := 55.76
	lastLon := 37.62
	require.NoError(t, tripRepo.ApplyBatch(ctx, domain.TripDetectionBatch{
		NewState: state,
		UpdatedTrips: []domain.TripStatsDelta{{
			TripID:     tripID,
			DeltaDistM: 150.0,
			DeltaPts:   3,
			LastPtAt:   &lastAt,
			LastLat:    &lastLat,
			LastLon:    &lastLon,
		}},
	}))

	var distM float64
	var ptsCount int
	err := db.QueryRow(ctx,
		"SELECT distance_m, points_count FROM trips WHERE id = $1", tripID,
	).Scan(&distM, &ptsCount)
	require.NoError(t, err)
	assert.InDelta(t, 150.0, distM, 0.001)
	assert.Equal(t, 3, ptsCount)
}

func TestApplyBatch_CompleteTrip(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	tripRepo := postgres.NewTripRepo(db)
	ctx := context.Background()

	userID, deviceID := newPair()
	tripID := uuid.New()
	t0 := now()

	trip := &domain.Trip{
		ID:        tripID,
		UserID:    userID,
		DeviceID:  deviceID,
		Status:    domain.TripStatusActive,
		StartedAt: t0.Add(-10 * time.Minute),
		StartLat:  55.7558,
		StartLon:  37.6173,
		CreatedAt: t0,
		UpdatedAt: t0,
	}
	require.NoError(t, tripRepo.ApplyBatch(ctx, domain.TripDetectionBatch{
		NewState: domain.TripDetectionState{
			UserID: userID, DeviceID: deviceID,
			State: domain.StateTripActive, ActiveTripID: &tripID,
			LateArrivalWindowSec: 300, UpdatedAt: t0,
		},
		NewTrips: []*domain.Trip{trip},
	}))

	endedAt := t0.Add(-2 * time.Minute)
	endLat := 55.76
	endLon := 37.62
	require.NoError(t, tripRepo.ApplyBatch(ctx, domain.TripDetectionBatch{
		NewState: domain.TripDetectionState{
			UserID: userID, DeviceID: deviceID,
			State: domain.StateIdle, LateArrivalWindowSec: 300, UpdatedAt: t0,
		},
		CompletedTrips: []domain.TripCompletion{{
			TripID:  tripID,
			EndedAt: endedAt,
			EndLat:  endLat,
			EndLon:  endLon,
		}},
	}))

	var status string
	var dbEndedAt time.Time
	err := db.QueryRow(ctx,
		"SELECT status, ended_at FROM trips WHERE id = $1", tripID,
	).Scan(&status, &dbEndedAt)
	require.NoError(t, err)
	assert.Equal(t, "TRIP_COMPLETED", status)
	assert.WithinDuration(t, endedAt, dbEndedAt, time.Second)
}

func TestApplyBatch_InsertTripPoints_Idempotent(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	rawRepo := postgres.NewRawLocationRepo(db)
	tripRepo := postgres.NewTripRepo(db)
	ctx := context.Background()

	userID, deviceID := newPair()
	tripID := uuid.New()
	t0 := now()
	base := t0.Add(-10 * time.Minute)

	rawPts := seedPoints(t, rawRepo, userID, deviceID, base, 3, 5.0)

	trip := &domain.Trip{
		ID: tripID, UserID: userID, DeviceID: deviceID,
		Status: domain.TripStatusActive, StartedAt: base,
		StartLat: 55.7558, StartLon: 37.6173,
		CreatedAt: t0, UpdatedAt: t0,
	}
	state := domain.TripDetectionState{
		UserID: userID, DeviceID: deviceID,
		State: domain.StateTripActive, ActiveTripID: &tripID,
		LateArrivalWindowSec: 300, UpdatedAt: t0,
	}

	tpPoints := make([]domain.TripPoint, len(rawPts))
	for i, rp := range rawPts {
		tpPoints[i] = domain.TripPoint{
			TripID: tripID, UserID: userID, DeviceID: deviceID,
			EventID: rp.EventID, RecordedAt: rp.RecordedAt,
		}
	}

	batch := domain.TripDetectionBatch{
		NewState: state, NewTrips: []*domain.Trip{trip}, NewPoints: tpPoints,
	}
	require.NoError(t, tripRepo.ApplyBatch(ctx, batch))

	// Apply same batch again — must succeed (idempotent).
	require.NoError(t, tripRepo.ApplyBatch(ctx, batch))

	var count int
	require.NoError(t, db.QueryRow(ctx,
		"SELECT COUNT(*) FROM trip_points WHERE trip_id = $1", tripID,
	).Scan(&count))
	assert.Equal(t, 3, count, "duplicate points must not be inserted twice")
}

func TestApplyBatch_UpsertDetectionState(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	tripRepo := postgres.NewTripRepo(db)
	ctx := context.Background()

	userID, deviceID := newPair()
	t0 := now()

	state1 := domain.TripDetectionState{
		UserID: userID, DeviceID: deviceID,
		State: domain.StateIdle, LateArrivalWindowSec: 300, UpdatedAt: t0,
	}
	require.NoError(t, tripRepo.ApplyBatch(ctx, domain.TripDetectionBatch{NewState: state1}))

	wm := t0.Add(-5 * time.Minute)
	state2 := domain.TripDetectionState{
		UserID: userID, DeviceID: deviceID,
		State: domain.StateMotionCandidate, LateArrivalWindowSec: 300,
		LastWatermarkAt: &wm, UpdatedAt: t0.Add(time.Minute),
	}
	require.NoError(t, tripRepo.ApplyBatch(ctx, domain.TripDetectionBatch{NewState: state2}))

	loaded, err := tripRepo.LoadState(ctx, userID, deviceID)
	require.NoError(t, err)
	assert.Equal(t, domain.StateMotionCandidate, loaded.State)
	require.NotNil(t, loaded.LastWatermarkAt)
	assert.WithinDuration(t, wm, *loaded.LastWatermarkAt, time.Second)
}

func TestApplyBatch_StoresProcessedPoint(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	rawRepo := postgres.NewRawLocationRepo(db)
	tripRepo := postgres.NewTripRepo(db)
	ctx := context.Background()
	userID, deviceID := newPair()
	recordedAt := now().Add(-time.Minute)
	raw := makeRawPoint(userID, deviceID, recordedAt, 1.2)
	require.NoError(t, rawRepo.Insert(ctx, raw))

	filteredLat := raw.Lat + 0.00001
	filteredLon := raw.Lon + 0.00001
	processed := domain.ProcessedLocationPoint{
		UserID: userID, DeviceID: deviceID, EventID: raw.EventID,
		RawLat: raw.Lat, RawLon: raw.Lon,
		FilteredLat: &filteredLat, FilteredLon: &filteredLon,
		AccuracyM: raw.AccuracyM, SpeedMps: raw.SpeedMps,
		ImpliedSpeedMps: 1.1, DistanceDeltaM: 12,
		IsAccepted: true, RecordedAt: raw.RecordedAt,
		ReceivedAt: raw.ReceivedAt, ProcessedAt: now(),
	}
	state := domain.TripDetectionState{
		UserID: userID, DeviceID: deviceID,
		State: domain.StateIdle, LateArrivalWindowSec: 45, UpdatedAt: now(),
	}

	require.NoError(t, tripRepo.ApplyBatch(ctx, domain.TripDetectionBatch{
		NewState: state, ProcessedPoints: []domain.ProcessedLocationPoint{processed},
	}))

	var accepted bool
	var storedLat float64
	require.NoError(t, db.QueryRow(ctx, `
		SELECT is_accepted, filtered_lat
		FROM processed_location_points
		WHERE user_id = $1 AND device_id = $2 AND event_id = $3`,
		userID, deviceID, raw.EventID,
	).Scan(&accepted, &storedLat))
	assert.True(t, accepted)
	assert.InDelta(t, filteredLat, storedLat, 0.000001)
}

// processedFor builds a processing result for a raw point at a given algorithm version.
func processedFor(raw domain.RawLocationPoint, version int16, processedAt time.Time) domain.ProcessedLocationPoint {
	lat, lon := raw.Lat, raw.Lon
	return domain.ProcessedLocationPoint{
		UserID: raw.UserID, DeviceID: raw.DeviceID, EventID: raw.EventID,
		RawLat: raw.Lat, RawLon: raw.Lon,
		FilteredLat: &lat, FilteredLon: &lon,
		AccuracyM: raw.AccuracyM, SpeedMps: raw.SpeedMps,
		IsAccepted: true, RecordedAt: raw.RecordedAt, ReceivedAt: raw.ReceivedAt,
		ProcessedAt: processedAt, AlgorithmVersion: version,
	}
}

func TestFetchPointsForReprocessing_SelectsMissingAndStale(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	rawRepo := postgres.NewRawLocationRepo(db)
	tripRepo := postgres.NewTripRepo(db)
	ctx := context.Background()
	userID, deviceID := newPair()
	base := now().Add(-10 * time.Minute)

	pts := seedPoints(t, rawRepo, userID, deviceID, base, 3, 1.5)
	stale, current, neverProcessed := pts[0], pts[1], pts[2]

	// stale -> processed at version 1; current -> processed at version 2; last -> never processed.
	require.NoError(t, tripRepo.UpsertProcessedPoints(ctx, []domain.ProcessedLocationPoint{
		processedFor(stale, 1, now()),
		processedFor(current, 2, now()),
	}))

	from, to := base.Add(-time.Minute), base.Add(time.Minute)
	got, err := tripRepo.FetchPointsForReprocessing(ctx, userID, deviceID, from, to, 2)
	require.NoError(t, err)

	ids := map[string]bool{}
	for _, p := range got {
		ids[p.EventID] = true
	}
	assert.True(t, ids[stale.EventID], "stale (version < current) must be selected")
	assert.True(t, ids[neverProcessed.EventID], "never-processed must be selected")
	assert.False(t, ids[current.EventID], "already-current version must be skipped")
}

func TestUpsertProcessedPoints_OnlyUpdatesWhenVersionOlder(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	rawRepo := postgres.NewRawLocationRepo(db)
	tripRepo := postgres.NewTripRepo(db)
	ctx := context.Background()
	userID, deviceID := newPair()
	recordedAt := now().Add(-time.Minute)
	raw := makeRawPoint(userID, deviceID, recordedAt, 1.5)
	require.NoError(t, rawRepo.Insert(ctx, raw))

	// Seed version 1 with a known distance.
	v1 := processedFor(raw, 1, now())
	v1.DistanceDeltaM = 10
	require.NoError(t, tripRepo.UpsertProcessedPoints(ctx, []domain.ProcessedLocationPoint{v1}))

	readBack := func() (int16, float64) {
		var version int16
		var distance float64
		require.NoError(t, db.QueryRow(ctx, `
			SELECT algorithm_version, distance_delta_m
			FROM processed_location_points
			WHERE user_id = $1 AND device_id = $2 AND event_id = $3`,
			userID, deviceID, raw.EventID,
		).Scan(&version, &distance))
		return version, distance
	}

	// Newer version updates derived fields in place.
	v2 := processedFor(raw, 2, now())
	v2.DistanceDeltaM = 22
	require.NoError(t, tripRepo.UpsertProcessedPoints(ctx, []domain.ProcessedLocationPoint{v2}))
	version, distance := readBack()
	assert.Equal(t, int16(2), version)
	assert.Equal(t, 22.0, distance)

	// Same/older version must be a no-op (WHERE version < EXCLUDED.version is false).
	stale := processedFor(raw, 1, now())
	stale.DistanceDeltaM = 999
	require.NoError(t, tripRepo.UpsertProcessedPoints(ctx, []domain.ProcessedLocationPoint{stale}))
	version, distance = readBack()
	assert.Equal(t, int16(2), version, "older version must not downgrade the row")
	assert.Equal(t, 22.0, distance, "older version must not overwrite derived fields")
}
