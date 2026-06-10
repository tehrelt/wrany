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

func makePoint() domain.RawLocationPoint {
	speedMps := 5.5
	bearingDeg := 90.0
	actConf := 0.9
	battLevel := 0.75

	return domain.RawLocationPoint{
		UserID:             uuid.New(),
		DeviceID:           uuid.New(),
		EventID:            uuid.New().String(),
		RecordedAt:         time.Now().UTC().Truncate(time.Microsecond),
		ReceivedAt:         time.Now().UTC().Truncate(time.Microsecond),
		Lat:                55.7558,
		Lon:                37.6173,
		AccuracyM:          10,
		SpeedMps:           &speedMps,
		BearingDeg:         &bearingDeg,
		ActivityType:       "walking",
		ActivityConfidence: &actConf,
		BatteryLevel:       &battLevel,
		Source:             "android_tracker",
	}
}

func TestInsert_ValidPoint(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	repo := postgres.NewRawLocationRepo(db)
	point := makePoint()

	err := repo.Insert(context.Background(), point)
	require.NoError(t, err)

	var count int
	err = db.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM raw_location_points WHERE event_id = $1", point.EventID,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestInsert_Duplicate_DoesNotCreateRow(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	repo := postgres.NewRawLocationRepo(db)
	point := makePoint()

	require.NoError(t, repo.Insert(context.Background(), point))
	require.NoError(t, repo.Insert(context.Background(), point), "duplicate must not return error")

	var count int
	require.NoError(t, db.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM raw_location_points WHERE event_id = $1", point.EventID,
	).Scan(&count))
	assert.Equal(t, 1, count, "duplicate insert must not add a second row")
}

func TestInsert_GeomLonLatOrder(t *testing.T) {
	// Verifies ST_MakePoint(lon, lat): ST_X = lon (X), ST_Y = lat (Y), SRID = 4326.
	db, cleanup := newTestDB(t)
	defer cleanup()

	repo := postgres.NewRawLocationRepo(db)
	point := makePoint()
	point.Lat = 55.7558
	point.Lon = 37.6173

	require.NoError(t, repo.Insert(context.Background(), point))

	var stX, stY float64
	var srid int
	err := db.QueryRow(context.Background(), `
		SELECT ST_X(geom), ST_Y(geom), ST_SRID(geom)
		FROM raw_location_points
		WHERE event_id = $1`,
		point.EventID,
	).Scan(&stX, &stY, &srid)
	require.NoError(t, err)

	assert.InDelta(t, point.Lon, stX, 1e-6, "ST_X(geom) must equal Lon")
	assert.InDelta(t, point.Lat, stY, 1e-6, "ST_Y(geom) must equal Lat")
	assert.Equal(t, 4326, srid, "SRID must be 4326")
}

func TestInsert_NullableFields_Stored(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	repo := postgres.NewRawLocationRepo(db)
	point := makePoint()
	// Set nullable fields to nil.
	point.SpeedMps = nil
	point.BearingDeg = nil
	point.ActivityConfidence = nil
	point.BatteryLevel = nil

	require.NoError(t, repo.Insert(context.Background(), point))

	var speedNull, bearingNull, actConfNull, battNull bool
	err := db.QueryRow(context.Background(), `
		SELECT
			speed_mps IS NULL,
			bearing_deg IS NULL,
			activity_confidence IS NULL,
			battery_level IS NULL
		FROM raw_location_points WHERE event_id = $1`, point.EventID,
	).Scan(&speedNull, &bearingNull, &actConfNull, &battNull)
	require.NoError(t, err)
	assert.True(t, speedNull)
	assert.True(t, bearingNull)
	assert.True(t, actConfNull)
	assert.True(t, battNull)
}
