package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wrany/tracking-worker/internal/domain"
)

// RawLocationRepo implements usecase.RawLocationRepository using Postgres/PostGIS.
type RawLocationRepo struct {
	db *pgxpool.Pool
}

// NewRawLocationRepo creates a new RawLocationRepo.
func NewRawLocationRepo(db *pgxpool.Pool) *RawLocationRepo {
	return &RawLocationRepo{db: db}
}

// Insert writes a RawLocationPoint to raw_location_points idempotently.
// Duplicate (user_id, device_id, event_id) is silently ignored via ON CONFLICT
// DO NOTHING — both a new insert and a confirmed duplicate return nil.
//
// geom is computed as ST_SetSRID(ST_MakePoint(lon, lat), 4326):
//   - ST_X(geom) = lon (X = longitude)
//   - ST_Y(geom) = lat (Y = latitude)
//   - ST_SRID(geom) = 4326
func (r *RawLocationRepo) Insert(ctx context.Context, p domain.RawLocationPoint) error {
	const q = `
		INSERT INTO raw_location_points (
			user_id, device_id, event_id,
			recorded_at, received_at,
			lat, lon, geom,
			accuracy_m, speed_mps, bearing_deg,
			activity_type, activity_confidence, battery_level,
			source
		) VALUES (
			$1,  $2,  $3,
			$4,  $5,
			$6,  $7,  ST_SetSRID(ST_MakePoint($7, $6), 4326),
			$8,  $9,  $10,
			$11, $12, $13,
			$14
		) ON CONFLICT (user_id, device_id, event_id) DO NOTHING`

	_, err := r.db.Exec(ctx, q,
		p.UserID, p.DeviceID, p.EventID,
		p.RecordedAt, p.ReceivedAt,
		p.Lat, p.Lon,
		p.AccuracyM, p.SpeedMps, p.BearingDeg,
		p.ActivityType, p.ActivityConfidence, p.BatteryLevel,
		p.Source,
	)
	if err != nil {
		return fmt.Errorf("raw_location_repo: insert %q: %w", p.EventID, err)
	}
	return nil
}
