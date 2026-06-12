package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wrany/tracking-gateway/internal/domain"
	"github.com/wrany/tracking-gateway/internal/storage/postgres"
	"github.com/wrany/tracking-gateway/internal/usecase"
)

// createTripsSchema creates trips, trip_points, and raw_location_points tables
// for tests. These are owned by tracking-worker and absent from gateway migrations.
const createTripsSchema = `
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
);

CREATE TABLE IF NOT EXISTS trips (
    id           UUID             PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID             NOT NULL,
    device_id    UUID             NOT NULL,
    status       TEXT             NOT NULL CHECK (status IN ('TRIP_ACTIVE', 'TRIP_COMPLETED')),
    started_at   TIMESTAMPTZ      NOT NULL,
    ended_at     TIMESTAMPTZ      NULL,
    start_lat    DOUBLE PRECISION NOT NULL,
    start_lon    DOUBLE PRECISION NOT NULL,
    end_lat      DOUBLE PRECISION NULL,
    end_lon      DOUBLE PRECISION NULL,
    distance_m   DOUBLE PRECISION NOT NULL DEFAULT 0,
    duration_sec BIGINT           NOT NULL DEFAULT 0,
    points_count INTEGER          NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ      NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ      NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS trip_points (
    trip_id     UUID        NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    user_id     UUID        NOT NULL,
    device_id   UUID        NOT NULL,
    event_id    TEXT        NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (trip_id, event_id),
    CONSTRAINT uq_trip_points_user_device_event UNIQUE (user_id, device_id, event_id)
);`

// insertTripRow inserts a trip and returns its UUID.
func insertTripRow(t *testing.T, db *pgxpool.Pool, userID, deviceID uuid.UUID, status domain.TripStatus, startedAt time.Time) uuid.UUID {
	t.Helper()
	tripID := uuid.New()
	_, err := db.Exec(context.Background(), `
		INSERT INTO trips (id, user_id, device_id, status, started_at, start_lat, start_lon, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 55.7558, 37.6173, now(), now())`,
		tripID, userID, deviceID, string(status), startedAt,
	)
	require.NoError(t, err)
	return tripID
}

// insertRawPointForTrip inserts a raw_location_points row (needed for GetTripPoints JOIN).
func insertRawPointForTrip(t *testing.T, db *pgxpool.Pool, userID, deviceID uuid.UUID, eventID string, recordedAt time.Time) {
	t.Helper()
	_, err := db.Exec(context.Background(), `
		INSERT INTO raw_location_points
		(user_id, device_id, event_id, recorded_at, received_at, lat, lon, geom, accuracy_m, activity_type, source)
		VALUES ($1, $2, $3, $4, $4, 55.76, 37.62, ST_MakePoint(37.62, 55.76), 5.0, 'walking', 'test')`,
		userID, deviceID, eventID, recordedAt,
	)
	require.NoError(t, err)
}

// insertTripPointRow inserts into trip_points.
func insertTripPointRow(t *testing.T, db *pgxpool.Pool, tripID, userID, deviceID uuid.UUID, eventID string, recordedAt time.Time) {
	t.Helper()
	_, err := db.Exec(context.Background(), `
		INSERT INTO trip_points (trip_id, user_id, device_id, event_id, recorded_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT DO NOTHING`,
		tripID, userID, deviceID, eventID, recordedAt,
	)
	require.NoError(t, err)
}

// ---- tests ----

func TestTripQueryRepo_ListTrips_UserIsolation(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()
	ctx := context.Background()

	_, err := db.Exec(ctx, createTripsSchema)
	require.NoError(t, err)

	repo := postgres.NewTripQueryRepo(db)
	userA := insertTestUser(t, db)
	userB := insertTestUser(t, db)
	deviceID := uuid.New()
	t0 := time.Now().UTC().Add(-10 * time.Minute)

	insertTripRow(t, db, userA, deviceID, domain.TripStatusCompleted, t0)

	trips, _, err := repo.ListTrips(ctx, domain.TripFilter{UserID: userA.String(), Limit: 10})
	require.NoError(t, err)
	assert.Len(t, trips, 1)

	trips, _, err = repo.ListTrips(ctx, domain.TripFilter{UserID: userB.String(), Limit: 10})
	require.NoError(t, err)
	assert.Empty(t, trips)
}

func TestTripQueryRepo_ListTrips_StatusFilter(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()
	ctx := context.Background()

	_, err := db.Exec(ctx, createTripsSchema)
	require.NoError(t, err)

	repo := postgres.NewTripQueryRepo(db)
	userID := insertTestUser(t, db)
	deviceID := uuid.New()
	t0 := time.Now().UTC().Add(-30 * time.Minute)

	insertTripRow(t, db, userID, deviceID, domain.TripStatusCompleted, t0)
	insertTripRow(t, db, userID, deviceID, domain.TripStatusActive, t0.Add(time.Minute))

	completed, _, err := repo.ListTrips(ctx, domain.TripFilter{
		UserID: userID.String(), Status: domain.TripStatusCompleted, Limit: 10,
	})
	require.NoError(t, err)
	assert.Len(t, completed, 1)
	assert.Equal(t, domain.TripStatusCompleted, completed[0].Status)

	active, _, err := repo.ListTrips(ctx, domain.TripFilter{
		UserID: userID.String(), Status: domain.TripStatusActive, Limit: 10,
	})
	require.NoError(t, err)
	assert.Len(t, active, 1)
	assert.Equal(t, domain.TripStatusActive, active[0].Status)

	all, _, err := repo.ListTrips(ctx, domain.TripFilter{UserID: userID.String(), Limit: 10})
	require.NoError(t, err)
	assert.Len(t, all, 2)
}

func TestTripQueryRepo_ListTrips_Pagination(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()
	ctx := context.Background()

	_, err := db.Exec(ctx, createTripsSchema)
	require.NoError(t, err)

	repo := postgres.NewTripQueryRepo(db)
	userID := insertTestUser(t, db)
	deviceID := uuid.New()
	t0 := time.Now().UTC().Add(-60 * time.Minute)

	for i := 0; i < 5; i++ {
		insertTripRow(t, db, userID, deviceID, domain.TripStatusCompleted, t0.Add(time.Duration(i)*time.Minute))
	}

	page1, cursor, err := repo.ListTrips(ctx, domain.TripFilter{UserID: userID.String(), Limit: 3})
	require.NoError(t, err)
	assert.Len(t, page1, 3)
	assert.NotEmpty(t, cursor)

	page2, cursor2, err := repo.ListTrips(ctx, domain.TripFilter{UserID: userID.String(), Limit: 3, Cursor: cursor})
	require.NoError(t, err)
	assert.Len(t, page2, 2)
	assert.Empty(t, cursor2)

	seen := map[string]bool{}
	for _, tr := range append(page1, page2...) {
		assert.False(t, seen[tr.ID], "duplicate trip %s", tr.ID)
		seen[tr.ID] = true
	}
	assert.Len(t, seen, 5)
}

func TestTripQueryRepo_GetTrip_Found(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()
	ctx := context.Background()

	_, err := db.Exec(ctx, createTripsSchema)
	require.NoError(t, err)

	repo := postgres.NewTripQueryRepo(db)
	userID := insertTestUser(t, db)
	deviceID := uuid.New()
	t0 := time.Now().UTC().Add(-5 * time.Minute)

	tripID := insertTripRow(t, db, userID, deviceID, domain.TripStatusCompleted, t0)

	trip, err := repo.GetTrip(ctx, userID.String(), tripID.String())
	require.NoError(t, err)
	assert.Equal(t, tripID.String(), trip.ID)
	assert.Equal(t, domain.TripStatusCompleted, trip.Status)
}

func TestTripQueryRepo_GetTrip_NotFound(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()
	ctx := context.Background()

	_, err := db.Exec(ctx, createTripsSchema)
	require.NoError(t, err)

	repo := postgres.NewTripQueryRepo(db)
	userID := insertTestUser(t, db)

	_, err = repo.GetTrip(ctx, userID.String(), uuid.New().String())
	assert.ErrorIs(t, err, usecase.ErrTripNotFound)
}

func TestTripQueryRepo_GetTrip_WrongUser(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()
	ctx := context.Background()

	_, err := db.Exec(ctx, createTripsSchema)
	require.NoError(t, err)

	repo := postgres.NewTripQueryRepo(db)
	ownerID := insertTestUser(t, db)
	otherID := insertTestUser(t, db)
	deviceID := uuid.New()
	t0 := time.Now().UTC().Add(-5 * time.Minute)

	tripID := insertTripRow(t, db, ownerID, deviceID, domain.TripStatusCompleted, t0)

	_, err = repo.GetTrip(ctx, otherID.String(), tripID.String())
	assert.ErrorIs(t, err, usecase.ErrTripNotFound)
}

func TestTripQueryRepo_GetTripPoints(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()
	ctx := context.Background()

	_, err := db.Exec(ctx, createTripsSchema)
	require.NoError(t, err)

	repo := postgres.NewTripQueryRepo(db)
	userID := insertTestUser(t, db)
	deviceID := uuid.New()
	t0 := time.Now().UTC().Add(-10 * time.Minute)

	tripID := insertTripRow(t, db, userID, deviceID, domain.TripStatusActive, t0)

	for i := 0; i < 3; i++ {
		recAt := t0.Add(time.Duration(i) * time.Second)
		evtID := uuid.New().String()
		insertRawPointForTrip(t, db, userID, deviceID, evtID, recAt)
		insertTripPointRow(t, db, tripID, userID, deviceID, evtID, recAt)
	}

	pts, nextCursor, err := repo.GetTripPoints(ctx, domain.TripPointFilter{
		TripID: tripID.String(),
		UserID: userID.String(),
		Limit:  10,
	})
	require.NoError(t, err)
	assert.Len(t, pts, 3)
	assert.Empty(t, nextCursor)

	for _, p := range pts {
		assert.Equal(t, tripID.String(), p.TripID)
		assert.InDelta(t, 55.76, p.Lat, 1e-4)
		assert.InDelta(t, 37.62, p.Lon, 1e-4)
	}

	for i := 1; i < len(pts); i++ {
		assert.True(t, !pts[i].RecordedAt.Before(pts[i-1].RecordedAt), "points must be ordered ASC")
	}
}

func TestTripQueryRepo_GetTripPoints_Pagination(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()
	ctx := context.Background()

	_, err := db.Exec(ctx, createTripsSchema)
	require.NoError(t, err)

	repo := postgres.NewTripQueryRepo(db)
	userID := insertTestUser(t, db)
	deviceID := uuid.New()
	t0 := time.Now().UTC().Add(-10 * time.Minute)

	tripID := insertTripRow(t, db, userID, deviceID, domain.TripStatusActive, t0)

	for i := 0; i < 5; i++ {
		recAt := t0.Add(time.Duration(i) * time.Second)
		evtID := uuid.New().String()
		insertRawPointForTrip(t, db, userID, deviceID, evtID, recAt)
		insertTripPointRow(t, db, tripID, userID, deviceID, evtID, recAt)
	}

	page1, cursor, err := repo.GetTripPoints(ctx, domain.TripPointFilter{
		TripID: tripID.String(), UserID: userID.String(), Limit: 3,
	})
	require.NoError(t, err)
	assert.Len(t, page1, 3)
	assert.NotEmpty(t, cursor)

	page2, cursor2, err := repo.GetTripPoints(ctx, domain.TripPointFilter{
		TripID: tripID.String(), UserID: userID.String(), Limit: 3, Cursor: cursor,
	})
	require.NoError(t, err)
	assert.Len(t, page2, 2)
	assert.Empty(t, cursor2)

	seen := map[string]bool{}
	for _, p := range append(page1, page2...) {
		assert.False(t, seen[p.EventID], "duplicate event_id %s", p.EventID)
		seen[p.EventID] = true
	}
	assert.Len(t, seen, 5)
}
