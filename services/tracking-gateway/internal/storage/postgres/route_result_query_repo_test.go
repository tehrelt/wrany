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
)

// createRoutesSchema creates routes and route_trips tables.
// These are owned by tracking-worker and absent from gateway migrations.
const createRoutesSchema = `
CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE IF NOT EXISTS routes (
    id            UUID             PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID             NOT NULL,
    device_id     UUID,
    name          TEXT,
    status        TEXT             NOT NULL DEFAULT 'active',
    start_lat     DOUBLE PRECISION NOT NULL,
    start_lon     DOUBLE PRECISION NOT NULL,
    end_lat       DOUBLE PRECISION NOT NULL,
    end_lon       DOUBLE PRECISION NOT NULL,
    distance_m    DOUBLE PRECISION NOT NULL DEFAULT 0,
    trips_count   INTEGER          NOT NULL DEFAULT 0,
    template_geom geometry(LineString, 4326) NOT NULL,
    start_geom    geometry(Point, 4326) GENERATED ALWAYS AS (ST_SetSRID(ST_MakePoint(start_lon, start_lat), 4326)) STORED,
    end_geom      geometry(Point, 4326) GENERATED ALWAYS AS (ST_SetSRID(ST_MakePoint(end_lon, end_lat), 4326)) STORED,
    first_trip_id UUID             NOT NULL REFERENCES trips(id),
    last_trip_id  UUID             NOT NULL REFERENCES trips(id),
    created_at    TIMESTAMPTZ      NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ      NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS route_trips (
    route_id     UUID             NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
    trip_id      UUID             NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    user_id      UUID             NOT NULL,
    device_id    UUID             NOT NULL,
    match_score  DOUBLE PRECISION NOT NULL,
    matched_at   TIMESTAMPTZ      NOT NULL DEFAULT now(),
    duration_sec BIGINT           NOT NULL,
    distance_m   DOUBLE PRECISION NOT NULL,
    PRIMARY KEY (route_id, trip_id),
    UNIQUE (trip_id)
);`

// insertCompletedTrip inserts a TRIP_COMPLETED row with explicit duration and distance.
func insertCompletedTrip(t *testing.T, db *pgxpool.Pool, userID, deviceID uuid.UUID, startedAt time.Time, durationSec int64, distanceM float64) uuid.UUID {
	t.Helper()
	tripID := uuid.New()
	endedAt := startedAt.Add(time.Duration(durationSec) * time.Second)
	_, err := db.Exec(context.Background(), `
		INSERT INTO trips (id, user_id, device_id, status, started_at, ended_at, start_lat, start_lon,
		                   duration_sec, distance_m, created_at, updated_at)
		VALUES ($1, $2, $3, 'TRIP_COMPLETED', $4, $5, 55.75, 37.61, $6, $7, now(), now())`,
		tripID, userID, deviceID, startedAt, endedAt, durationSec, distanceM,
	)
	require.NoError(t, err)
	return tripID
}

// insertRoute inserts a route row and returns its UUID.
func insertRoute(t *testing.T, db *pgxpool.Pool, userID, deviceID, firstTripID uuid.UUID) uuid.UUID {
	t.Helper()
	routeID := uuid.New()
	_, err := db.Exec(context.Background(), `
		INSERT INTO routes (id, user_id, device_id, start_lat, start_lon, end_lat, end_lon,
		                    template_geom, first_trip_id, last_trip_id, created_at, updated_at)
		VALUES ($1, $2, $3, 55.75, 37.61, 55.76, 37.62,
		        ST_GeomFromText('LINESTRING(37.61 55.75, 37.62 55.76)', 4326),
		        $4, $4, now(), now())`,
		routeID, userID, deviceID, firstTripID,
	)
	require.NoError(t, err)
	return routeID
}

// insertRouteTrip inserts a route_trips row.
func insertRouteTrip(t *testing.T, db *pgxpool.Pool, routeID, tripID, userID, deviceID uuid.UUID, matchScore float64, matchedAt time.Time, durationSec int64, distanceM float64) {
	t.Helper()
	_, err := db.Exec(context.Background(), `
		INSERT INTO route_trips (route_id, trip_id, user_id, device_id, match_score, matched_at, duration_sec, distance_m)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		routeID, tripID, userID, deviceID, matchScore, matchedAt, durationSec, distanceM,
	)
	require.NoError(t, err)
}

// setupRoutesDB creates a test DB and executes both trips and routes schemas.
func setupRoutesDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	db, cleanup := newTestDB(t)
	ctx := context.Background()
	_, err := db.Exec(ctx, createTripsSchema)
	require.NoError(t, err)
	_, err = db.Exec(ctx, createRoutesSchema)
	require.NoError(t, err)
	return db, cleanup
}

// ---- GetRouteResult tests ----

func TestRouteResultQueryRepo_GetRouteResult_ZeroAttempts(t *testing.T) {
	db, cleanup := setupRoutesDB(t)
	defer cleanup()

	repo := postgres.NewRouteResultQueryRepo(db)
	userID := insertTestUser(t, db)
	deviceID := uuid.New()
	t0 := time.Now().UTC().Add(-30 * time.Minute)

	tripID := insertCompletedTrip(t, db, userID, deviceID, t0, 600, 900)
	routeID := insertRoute(t, db, userID, deviceID, tripID)

	// No route_trips inserted → zero attempts.
	res, err := repo.GetRouteResult(context.Background(), routeID.String())
	require.NoError(t, err)
	assert.Equal(t, 0, res.AttemptsCount)
	assert.Nil(t, res.Best)
	assert.Nil(t, res.Latest)
}

func TestRouteResultQueryRepo_GetRouteResult_BestIsMinDuration(t *testing.T) {
	db, cleanup := setupRoutesDB(t)
	defer cleanup()

	repo := postgres.NewRouteResultQueryRepo(db)
	userID := insertTestUser(t, db)
	deviceID := uuid.New()
	t0 := time.Now().UTC().Add(-60 * time.Minute)

	trip1 := insertCompletedTrip(t, db, userID, deviceID, t0, 800, 900)
	trip2 := insertCompletedTrip(t, db, userID, deviceID, t0.Add(20*time.Minute), 600, 900) // faster
	trip3 := insertCompletedTrip(t, db, userID, deviceID, t0.Add(40*time.Minute), 700, 900)

	routeID := insertRoute(t, db, userID, deviceID, trip1)
	matchedAt := time.Now().UTC()
	insertRouteTrip(t, db, routeID, trip1, userID, deviceID, 0.9, matchedAt, 800, 900)
	insertRouteTrip(t, db, routeID, trip2, userID, deviceID, 0.9, matchedAt.Add(time.Second), 600, 900)
	insertRouteTrip(t, db, routeID, trip3, userID, deviceID, 0.9, matchedAt.Add(2*time.Second), 700, 900)

	res, err := repo.GetRouteResult(context.Background(), routeID.String())
	require.NoError(t, err)
	assert.Equal(t, 3, res.AttemptsCount)
	require.NotNil(t, res.Best)
	assert.Equal(t, trip2.String(), res.Best.TripID)
	assert.Equal(t, int64(600), res.Best.DurationSec)
}

func TestRouteResultQueryRepo_GetRouteResult_LatestIsMaxStartedAt(t *testing.T) {
	db, cleanup := setupRoutesDB(t)
	defer cleanup()

	repo := postgres.NewRouteResultQueryRepo(db)
	userID := insertTestUser(t, db)
	deviceID := uuid.New()
	t0 := time.Now().UTC().Add(-60 * time.Minute)

	trip1 := insertCompletedTrip(t, db, userID, deviceID, t0, 700, 900)
	trip2 := insertCompletedTrip(t, db, userID, deviceID, t0.Add(10*time.Minute), 750, 900)
	trip3 := insertCompletedTrip(t, db, userID, deviceID, t0.Add(40*time.Minute), 680, 900) // latest

	routeID := insertRoute(t, db, userID, deviceID, trip1)
	matchedAt := time.Now().UTC()
	insertRouteTrip(t, db, routeID, trip1, userID, deviceID, 0.9, matchedAt, 700, 900)
	insertRouteTrip(t, db, routeID, trip2, userID, deviceID, 0.9, matchedAt.Add(time.Second), 750, 900)
	insertRouteTrip(t, db, routeID, trip3, userID, deviceID, 0.9, matchedAt.Add(2*time.Second), 680, 900)

	res, err := repo.GetRouteResult(context.Background(), routeID.String())
	require.NoError(t, err)
	require.NotNil(t, res.Latest)
	assert.Equal(t, trip3.String(), res.Latest.TripID)
}

func TestRouteResultQueryRepo_GetRouteResult_TieBreaker_EarlierStartedAtWins(t *testing.T) {
	db, cleanup := setupRoutesDB(t)
	defer cleanup()

	repo := postgres.NewRouteResultQueryRepo(db)
	userID := insertTestUser(t, db)
	deviceID := uuid.New()
	t0 := time.Now().UTC().Add(-60 * time.Minute)

	// Two trips with equal duration_sec; earlier started_at should win as best.
	tripEarlier := insertCompletedTrip(t, db, userID, deviceID, t0, 600, 900)
	tripLater := insertCompletedTrip(t, db, userID, deviceID, t0.Add(20*time.Minute), 600, 900)

	routeID := insertRoute(t, db, userID, deviceID, tripEarlier)
	matchedAt := time.Now().UTC()
	insertRouteTrip(t, db, routeID, tripEarlier, userID, deviceID, 0.9, matchedAt, 600, 900)
	insertRouteTrip(t, db, routeID, tripLater, userID, deviceID, 0.9, matchedAt.Add(time.Second), 600, 900)

	res, err := repo.GetRouteResult(context.Background(), routeID.String())
	require.NoError(t, err)
	require.NotNil(t, res.Best)
	assert.Equal(t, tripEarlier.String(), res.Best.TripID, "earlier started_at should win tie-breaker")
}

func TestRouteResultQueryRepo_GetRouteResult_SingleAttempt_BestEqualsLatest(t *testing.T) {
	db, cleanup := setupRoutesDB(t)
	defer cleanup()

	repo := postgres.NewRouteResultQueryRepo(db)
	userID := insertTestUser(t, db)
	deviceID := uuid.New()
	t0 := time.Now().UTC().Add(-30 * time.Minute)

	tripID := insertCompletedTrip(t, db, userID, deviceID, t0, 500, 800)
	routeID := insertRoute(t, db, userID, deviceID, tripID)
	insertRouteTrip(t, db, routeID, tripID, userID, deviceID, 0.95, time.Now().UTC(), 500, 800)

	res, err := repo.GetRouteResult(context.Background(), routeID.String())
	require.NoError(t, err)
	assert.Equal(t, 1, res.AttemptsCount)
	require.NotNil(t, res.Best)
	require.NotNil(t, res.Latest)
	assert.Equal(t, res.Best.TripID, res.Latest.TripID)
}

func TestRouteResultQueryRepo_GetRouteResult_UnknownRouteID_ReturnsEmpty(t *testing.T) {
	db, cleanup := setupRoutesDB(t)
	defer cleanup()

	repo := postgres.NewRouteResultQueryRepo(db)

	res, err := repo.GetRouteResult(context.Background(), uuid.New().String())
	require.NoError(t, err)
	assert.Equal(t, 0, res.AttemptsCount)
	assert.Nil(t, res.Best)
	assert.Nil(t, res.Latest)
}

// ---- ListRouteAttempts tests ----

func TestRouteResultQueryRepo_ListRouteAttempts_IsBestFlag(t *testing.T) {
	db, cleanup := setupRoutesDB(t)
	defer cleanup()

	repo := postgres.NewRouteResultQueryRepo(db)
	userID := insertTestUser(t, db)
	deviceID := uuid.New()
	t0 := time.Now().UTC().Add(-60 * time.Minute)

	trip1 := insertCompletedTrip(t, db, userID, deviceID, t0, 800, 900)
	trip2 := insertCompletedTrip(t, db, userID, deviceID, t0.Add(20*time.Minute), 550, 900) // best
	trip3 := insertCompletedTrip(t, db, userID, deviceID, t0.Add(40*time.Minute), 700, 900)

	routeID := insertRoute(t, db, userID, deviceID, trip1)
	matchedAt := time.Now().UTC()
	insertRouteTrip(t, db, routeID, trip1, userID, deviceID, 0.9, matchedAt, 800, 900)
	insertRouteTrip(t, db, routeID, trip2, userID, deviceID, 0.9, matchedAt.Add(time.Second), 550, 900)
	insertRouteTrip(t, db, routeID, trip3, userID, deviceID, 0.9, matchedAt.Add(2*time.Second), 700, 900)

	attempts, cursor, err := repo.ListRouteAttempts(context.Background(), domain.TripAttemptFilter{
		RouteID: routeID.String(),
		UserID:  userID.String(),
		Limit:   10,
	})
	require.NoError(t, err)
	assert.Empty(t, cursor)
	assert.Len(t, attempts, 3)

	bestCount := 0
	for _, a := range attempts {
		if a.IsBest {
			bestCount++
			assert.Equal(t, trip2.String(), a.TripID, "is_best must be on the trip with min duration")
		}
	}
	assert.Equal(t, 1, bestCount, "exactly one attempt must have is_best=true")
}

func TestRouteResultQueryRepo_ListRouteAttempts_Pagination(t *testing.T) {
	db, cleanup := setupRoutesDB(t)
	defer cleanup()

	repo := postgres.NewRouteResultQueryRepo(db)
	userID := insertTestUser(t, db)
	deviceID := uuid.New()
	t0 := time.Now().UTC().Add(-60 * time.Minute)

	var tripIDs []uuid.UUID
	firstTripID := insertCompletedTrip(t, db, userID, deviceID, t0, 600, 900)
	tripIDs = append(tripIDs, firstTripID)
	routeID := insertRoute(t, db, userID, deviceID, firstTripID)

	for i := 0; i < 5; i++ {
		var tripID uuid.UUID
		if i == 0 {
			tripID = firstTripID
		} else {
			tripID = insertCompletedTrip(t, db, userID, deviceID, t0.Add(time.Duration(i)*10*time.Minute), int64(600+i*10), 900)
			tripIDs = append(tripIDs, tripID)
		}
		matchedAt := time.Now().UTC().Add(-time.Duration(5-i) * time.Second)
		insertRouteTrip(t, db, routeID, tripID, userID, deviceID, 0.9, matchedAt, int64(600+i*10), 900)
	}

	page1, cur1, err := repo.ListRouteAttempts(context.Background(), domain.TripAttemptFilter{
		RouteID: routeID.String(), UserID: userID.String(), Limit: 3,
	})
	require.NoError(t, err)
	assert.Len(t, page1, 3)
	assert.NotEmpty(t, cur1)

	page2, cur2, err := repo.ListRouteAttempts(context.Background(), domain.TripAttemptFilter{
		RouteID: routeID.String(), UserID: userID.String(), Limit: 3, Cursor: cur1,
	})
	require.NoError(t, err)
	assert.Len(t, page2, 2)
	assert.Empty(t, cur2)

	seen := map[string]bool{}
	for _, a := range append(page1, page2...) {
		assert.False(t, seen[a.TripID], "duplicate trip_id %s", a.TripID)
		seen[a.TripID] = true
	}
	assert.Len(t, seen, 5)
}

func TestRouteResultQueryRepo_ListRouteAttempts_UserIsolation(t *testing.T) {
	db, cleanup := setupRoutesDB(t)
	defer cleanup()

	repo := postgres.NewRouteResultQueryRepo(db)
	userA := insertTestUser(t, db)
	userB := insertTestUser(t, db)
	deviceID := uuid.New()
	t0 := time.Now().UTC().Add(-30 * time.Minute)

	tripA := insertCompletedTrip(t, db, userA, deviceID, t0, 600, 900)
	routeA := insertRoute(t, db, userA, deviceID, tripA)
	insertRouteTrip(t, db, routeA, tripA, userA, deviceID, 0.9, time.Now().UTC(), 600, 900)

	// userB queries the route owned by userA — should get no results.
	attempts, _, err := repo.ListRouteAttempts(context.Background(), domain.TripAttemptFilter{
		RouteID: routeA.String(), UserID: userB.String(), Limit: 10,
	})
	require.NoError(t, err)
	assert.Empty(t, attempts, "userB must not see userA's attempts")
}

func TestRouteResultQueryRepo_ListRouteAttempts_AvgSpeedComputed(t *testing.T) {
	db, cleanup := setupRoutesDB(t)
	defer cleanup()

	repo := postgres.NewRouteResultQueryRepo(db)
	userID := insertTestUser(t, db)
	deviceID := uuid.New()
	t0 := time.Now().UTC().Add(-30 * time.Minute)

	tripID := insertCompletedTrip(t, db, userID, deviceID, t0, 900, 1800) // 1800m / 900s = 2.0 m/s
	routeID := insertRoute(t, db, userID, deviceID, tripID)
	insertRouteTrip(t, db, routeID, tripID, userID, deviceID, 0.9, time.Now().UTC(), 900, 1800)

	attempts, _, err := repo.ListRouteAttempts(context.Background(), domain.TripAttemptFilter{
		RouteID: routeID.String(), UserID: userID.String(), Limit: 10,
	})
	require.NoError(t, err)
	require.Len(t, attempts, 1)
	assert.InDelta(t, 2.0, attempts[0].AvgSpeedMps, 0.001)
}
