package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wrany/tracking-gateway/internal/domain"
	"github.com/wrany/tracking-gateway/internal/storage/postgres"
)

// createRawLocationPointsTable creates the table in the test DB.
// tracking-gateway migrations don't include it (it's in tracking-worker),
// so we create it here for isolation.
const createRawLocationPointsSQL = `
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE TABLE IF NOT EXISTS raw_location_points (
    user_id     UUID             NOT NULL,
    device_id   UUID             NOT NULL,
    event_id    TEXT             NOT NULL,
    recorded_at TIMESTAMPTZ      NOT NULL,
    received_at TIMESTAMPTZ      NOT NULL,
    stored_at   TIMESTAMPTZ      NOT NULL DEFAULT now(),
    lat         DOUBLE PRECISION NOT NULL,
    lon         DOUBLE PRECISION NOT NULL,
    geom        geometry(Point, 4326) NOT NULL,
    accuracy_m  DOUBLE PRECISION NOT NULL,
    speed_mps   DOUBLE PRECISION NULL,
    bearing_deg DOUBLE PRECISION NULL,
    activity_type        TEXT NOT NULL,
    activity_confidence  DOUBLE PRECISION NULL,
    battery_level        DOUBLE PRECISION NULL,
    source TEXT NOT NULL,
    PRIMARY KEY (user_id, device_id, event_id)
);`

const createProcessedLocationPointsSQL = `
CREATE TABLE IF NOT EXISTS processed_location_points (
    user_id UUID NOT NULL,
    device_id UUID NOT NULL,
    event_id TEXT NOT NULL,
    filtered_lat DOUBLE PRECISION NULL,
    filtered_lon DOUBLE PRECISION NULL,
    accuracy_m DOUBLE PRECISION NOT NULL,
    speed_mps DOUBLE PRECISION NULL,
    is_accepted BOOLEAN NOT NULL,
    is_stationary BOOLEAN NOT NULL,
    noise_reason TEXT NOT NULL DEFAULT '',
    recorded_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (user_id, device_id, event_id)
);`

func TestTrackingQueryRepo_GetPoints_UserIsolation(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()
	ctx := context.Background()

	_, err := db.Exec(ctx, createRawLocationPointsSQL)
	require.NoError(t, err)

	repo := postgres.NewTrackingQueryRepo(db)

	userA := insertTestUser(t, db)
	userB := insertTestUser(t, db)

	recAt := time.Now().UTC().Add(-10 * time.Minute)
	_, err = db.Exec(ctx, `
		INSERT INTO raw_location_points
		(user_id, device_id, event_id, recorded_at, received_at, lat, lon, geom, accuracy_m, activity_type, source)
		VALUES ($1, $2, 'evt-a', $3, $3, 55.0, 37.0, ST_MakePoint(37.0, 55.0), 5.0, 'walking', 'test')`,
		userA, userA, recAt,
	)
	require.NoError(t, err)

	from := recAt.Add(-1 * time.Hour)
	to := recAt.Add(1 * time.Hour)

	// User A sees their point.
	points, _, err := repo.GetPoints(ctx, domain.TrackingPointFilter{
		UserID: userA.String(), From: from, To: to, Limit: 100,
	})
	require.NoError(t, err)
	assert.Len(t, points, 1)
	assert.Equal(t, "evt-a", points[0].EventID)

	// User B sees nothing.
	points, _, err = repo.GetPoints(ctx, domain.TrackingPointFilter{
		UserID: userB.String(), From: from, To: to, Limit: 100,
	})
	require.NoError(t, err)
	assert.Empty(t, points)
}

func TestTrackingQueryRepo_GetPoints_Pagination(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()
	ctx := context.Background()

	_, err := db.Exec(ctx, createRawLocationPointsSQL)
	require.NoError(t, err)

	repo := postgres.NewTrackingQueryRepo(db)
	userID := insertTestUser(t, db)

	base := time.Now().UTC().Add(-30 * time.Minute)
	for i := range 5 {
		recAt := base.Add(time.Duration(i) * time.Second)
		_, err = db.Exec(ctx, `
			INSERT INTO raw_location_points
			(user_id, device_id, event_id, recorded_at, received_at, lat, lon, geom, accuracy_m, activity_type, source)
			VALUES ($1, $2, $3, $4, $4, 55.0, 37.0, ST_MakePoint(37.0, 55.0), 5.0, 'walking', 'test')`,
			userID, userID, "evt-"+string(rune('0'+i)), recAt,
		)
		require.NoError(t, err)
	}

	from := base.Add(-1 * time.Minute)
	to := base.Add(10 * time.Minute)

	// Page 1: limit=3.
	page1, cursor, err := repo.GetPoints(ctx, domain.TrackingPointFilter{
		UserID: userID.String(), From: from, To: to, Limit: 3,
	})
	require.NoError(t, err)
	assert.Len(t, page1, 3)
	assert.NotEmpty(t, cursor)

	// Page 2 using cursor.
	page2, cursor2, err := repo.GetPoints(ctx, domain.TrackingPointFilter{
		UserID: userID.String(), From: from, To: to, Limit: 3, Cursor: cursor,
	})
	require.NoError(t, err)
	assert.Len(t, page2, 2)
	assert.Empty(t, cursor2)

	// No overlap between pages.
	seen := map[string]bool{}
	for _, p := range append(page1, page2...) {
		assert.False(t, seen[p.EventID], "duplicate event_id %s", p.EventID)
		seen[p.EventID] = true
	}
	assert.Len(t, seen, 5)
}

func TestTrackingQueryRepo_GetPoints_DeviceFilter(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()
	ctx := context.Background()

	_, err := db.Exec(ctx, createRawLocationPointsSQL)
	require.NoError(t, err)

	repo := postgres.NewTrackingQueryRepo(db)
	userID := insertTestUser(t, db)

	recAt := time.Now().UTC().Add(-5 * time.Minute)
	deviceA := insertTestUser(t, db) // reusing uuid generator
	deviceB := insertTestUser(t, db)

	for _, pair := range []struct{ dev, evt string }{{deviceA.String(), "evt-da"}, {deviceB.String(), "evt-db"}} {
		_, err = db.Exec(ctx, `
			INSERT INTO raw_location_points
			(user_id, device_id, event_id, recorded_at, received_at, lat, lon, geom, accuracy_m, activity_type, source)
			VALUES ($1, $2, $3, $4, $4, 55.0, 37.0, ST_MakePoint(37.0, 55.0), 5.0, 'walking', 'test')`,
			userID, pair.dev, pair.evt, recAt,
		)
		require.NoError(t, err)
	}

	from := recAt.Add(-1 * time.Hour)
	to := recAt.Add(1 * time.Hour)

	points, _, err := repo.GetPoints(ctx, domain.TrackingPointFilter{
		UserID: userID.String(), DeviceID: deviceA.String(), From: from, To: to, Limit: 100,
	})
	require.NoError(t, err)
	require.Len(t, points, 1)
	assert.Equal(t, "evt-da", points[0].EventID)
}

func TestTrackingQueryRepo_GetSummary(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()
	ctx := context.Background()

	_, err := db.Exec(ctx, createRawLocationPointsSQL)
	require.NoError(t, err)

	repo := postgres.NewTrackingQueryRepo(db)
	userID := insertTestUser(t, db)

	base := time.Now().UTC().Add(-1 * time.Hour)
	speed := 2.0
	for i := range 3 {
		recAt := base.Add(time.Duration(i*10) * time.Minute)
		_, err = db.Exec(ctx, `
			INSERT INTO raw_location_points
			(user_id, device_id, event_id, recorded_at, received_at, lat, lon, geom, accuracy_m, speed_mps, activity_type, source)
			VALUES ($1, $2, $3, $4, $4, 55.0, 37.0, ST_MakePoint(37.0, 55.0), 5.0, $5, 'walking', 'test')`,
			userID, userID, "s"+string(rune('0'+i)), recAt, float64(i+1)*speed,
		)
		require.NoError(t, err)
	}

	from := base.Add(-5 * time.Minute)
	to := base.Add(30 * time.Minute)

	summary, err := repo.GetSummary(ctx, domain.TrackingPointFilter{
		UserID: userID.String(), From: from, To: to,
	})
	require.NoError(t, err)
	assert.Equal(t, 3, summary.PointsCount)
	assert.NotNil(t, summary.FirstRecordedAt)
	assert.NotNil(t, summary.LastRecordedAt)
	assert.Greater(t, summary.DurationSec, int64(0))
	assert.NotNil(t, summary.MaxSpeedMps)
	assert.InDelta(t, 6.0, *summary.MaxSpeedMps, 0.01)
}

func TestTrackingQueryRepo_GetSummary_Empty(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()
	ctx := context.Background()

	_, err := db.Exec(ctx, createRawLocationPointsSQL)
	require.NoError(t, err)

	repo := postgres.NewTrackingQueryRepo(db)
	userID := insertTestUser(t, db)

	from := time.Now().UTC().Add(-2 * time.Hour)
	to := time.Now().UTC().Add(-1 * time.Hour)

	summary, err := repo.GetSummary(ctx, domain.TrackingPointFilter{
		UserID: userID.String(), From: from, To: to,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, summary.PointsCount)
	assert.Nil(t, summary.FirstRecordedAt)
	assert.Nil(t, summary.LastRecordedAt)
}

func TestTrackingQueryRepo_GetTrack_UsesProcessedPointsAndSegmentBreaks(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()
	ctx := context.Background()

	_, err := db.Exec(ctx, createProcessedLocationPointsSQL)
	require.NoError(t, err)

	repo := postgres.NewTrackingQueryRepo(db)
	userID := insertTestUser(t, db)
	base := time.Now().UTC().Add(-30 * time.Minute)

	for i := range 4 {
		recordedAt := base.Add(time.Duration(i) * 30 * time.Second)
		_, err = db.Exec(ctx, `
			INSERT INTO processed_location_points
				(user_id, device_id, event_id, filtered_lat, filtered_lon, accuracy_m,
				 speed_mps, is_accepted, is_stationary, noise_reason, recorded_at)
			VALUES ($1, $1, $2, $3, $4, 5, 0.1, true, true, '', $5)`,
			userID, "stay-"+string(rune('0'+i)), 55.0+float64(i)*0.00001,
			37.0+float64(i)*0.00001, recordedAt,
		)
		require.NoError(t, err)
	}

	for i := range 2 {
		recordedAt := base.Add(5*time.Minute + time.Duration(i)*40*time.Second)
		reason := ""
		if i == 0 {
			reason = "segment_break"
		}
		_, err = db.Exec(ctx, `
			INSERT INTO processed_location_points
				(user_id, device_id, event_id, filtered_lat, filtered_lon, accuracy_m,
				 speed_mps, is_accepted, is_stationary, noise_reason, recorded_at)
			VALUES ($1, $1, $2, $3, $4, 5, 1.2, true, false, $5, $6)`,
			userID, "move-"+string(rune('0'+i)), 55.001+float64(i)*0.0001,
			37.001+float64(i)*0.0001, reason, recordedAt,
		)
		require.NoError(t, err)
	}

	segments, err := repo.GetTrack(ctx, domain.TrackFilter{
		UserID: userID.String(), From: base.Add(-time.Minute), To: base.Add(time.Hour),
		MinStaySec: 60, MinMoveSec: 30,
	})
	require.NoError(t, err)
	require.Len(t, segments, 3)

	assert.Equal(t, domain.TrackSegmentStay, segments[0].Kind)
	assert.Equal(t, 4, segments[0].MergedCount)
	assert.InDelta(t, 55.000015, segments[0].Lat, 0.000001)
	assert.NotEqual(t, segments[0].SegmentID, segments[1].SegmentID)
	assert.Equal(t, domain.TrackSegmentMove, segments[1].Kind)
	assert.Equal(t, segments[1].SegmentID, segments[2].SegmentID)
}
