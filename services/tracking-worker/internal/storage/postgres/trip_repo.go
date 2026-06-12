package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/google/uuid"
	"github.com/wrany/tracking-worker/internal/domain"
)

// TripRepo implements usecase.TripDetectionRepository.
type TripRepo struct {
	db *pgxpool.Pool
}

// NewTripRepo creates a TripRepo backed by the given pool.
func NewTripRepo(db *pgxpool.Pool) *TripRepo {
	return &TripRepo{db: db}
}

// LoadDistinctUserDevicePairs returns all (user_id, device_id) pairs that have
// raw_location_points recorded before the given boundary time.
func (r *TripRepo) LoadDistinctUserDevicePairs(ctx context.Context, before time.Time) ([]domain.UserDevicePair, error) {
	const q = `
		SELECT DISTINCT rlp.user_id, rlp.device_id
		FROM raw_location_points rlp
		LEFT JOIN trip_detection_state tds
			ON tds.user_id = rlp.user_id AND tds.device_id = rlp.device_id
		WHERE rlp.recorded_at < $1
		  AND rlp.recorded_at >= COALESCE(tds.last_watermark_at, '-infinity'::timestamptz)`

	rows, err := r.db.Query(ctx, q, before)
	if err != nil {
		return nil, fmt.Errorf("trip_repo: load pairs: %w", err)
	}
	defer rows.Close()

	var pairs []domain.UserDevicePair
	for rows.Next() {
		var p domain.UserDevicePair
		if err := rows.Scan(&p.UserID, &p.DeviceID); err != nil {
			return nil, fmt.Errorf("trip_repo: scan pair: %w", err)
		}
		pairs = append(pairs, p)
	}
	return pairs, rows.Err()
}

// LoadState returns the persisted detection state for a pair.
// Returns a default IDLE state (with LateArrivalWindowSec=300) when none is found.
func (r *TripRepo) LoadState(ctx context.Context, userID, deviceID uuid.UUID) (domain.TripDetectionState, error) {
	const q = `
		SELECT
			state,
			active_trip_id,
			candidate_started_at,
			candidate_start_point_id,
			candidate_distance_m,
			candidate_start_lat,
			candidate_start_lon,
			stop_started_at,
			stop_center_lat,
			stop_center_lon,
			last_point_lat,
			last_point_lon,
			last_processed_recorded_at,
			last_watermark_at,
			late_arrival_window_sec,
			updated_at
		FROM trip_detection_state
		WHERE user_id = $1 AND device_id = $2`

	row := r.db.QueryRow(ctx, q, userID, deviceID)

	var s domain.TripDetectionState
	s.UserID = userID
	s.DeviceID = deviceID

	var stateStr string
	err := row.Scan(
		&stateStr,
		&s.ActiveTripID,
		&s.CandidateStartedAt,
		&s.CandidateStartPointID,
		&s.CandidateDistanceM,
		&s.CandidateStartLat,
		&s.CandidateStartLon,
		&s.StopStartedAt,
		&s.StopCenterLat,
		&s.StopCenterLon,
		&s.LastPointLat,
		&s.LastPointLon,
		&s.LastProcessedAt,
		&s.LastWatermarkAt,
		&s.LateArrivalWindowSec,
		&s.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.TripDetectionState{
				UserID:               userID,
				DeviceID:             deviceID,
				State:                domain.StateIdle,
				LateArrivalWindowSec: 300,
			}, nil
		}
		return domain.TripDetectionState{}, fmt.Errorf("trip_repo: load state: %w", err)
	}
	s.State = domain.TripState(stateStr)
	return s, nil
}

// FetchPoints returns raw_location_points in [from, to) for a pair, ordered by recorded_at ASC.
func (r *TripRepo) FetchPoints(ctx context.Context, userID, deviceID uuid.UUID, from, to time.Time) ([]domain.RawLocationPoint, error) {
	const q = `
		SELECT
			user_id, device_id, event_id,
			recorded_at, received_at,
			lat, lon,
			accuracy_m, speed_mps, bearing_deg,
			activity_type, activity_confidence, battery_level,
			source
		FROM raw_location_points
		WHERE user_id = $1
		  AND device_id = $2
		  AND recorded_at >= $3
		  AND recorded_at < $4
		ORDER BY recorded_at ASC`

	rows, err := r.db.Query(ctx, q, userID, deviceID, from, to)
	if err != nil {
		return nil, fmt.Errorf("trip_repo: fetch points: %w", err)
	}
	defer rows.Close()

	var pts []domain.RawLocationPoint
	for rows.Next() {
		var p domain.RawLocationPoint
		if err := rows.Scan(
			&p.UserID, &p.DeviceID, &p.EventID,
			&p.RecordedAt, &p.ReceivedAt,
			&p.Lat, &p.Lon,
			&p.AccuracyM, &p.SpeedMps, &p.BearingDeg,
			&p.ActivityType, &p.ActivityConfidence, &p.BatteryLevel,
			&p.Source,
		); err != nil {
			return nil, fmt.Errorf("trip_repo: scan point: %w", err)
		}
		pts = append(pts, p)
	}
	return pts, rows.Err()
}

// ApplyBatch persists all detection results inside a single serialisable transaction.
func (r *TripRepo) ApplyBatch(ctx context.Context, batch domain.TripDetectionBatch) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("trip_repo: begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	// 1. INSERT new trips.
	for _, trip := range batch.NewTrips {
		if err = insertTrip(ctx, tx, trip); err != nil {
			return err
		}
	}

	// 2. Backfill candidate points (those that arrived before the current batch window).
	for _, trip := range batch.NewTrips {
		if since, ok := batch.BackfillSince[trip.ID]; ok {
			// Find the earliest recorded_at in NewPoints for this trip to avoid overlap.
			var firstBatchPt time.Time
			for _, tp := range batch.NewPoints {
				if tp.TripID == trip.ID {
					if firstBatchPt.IsZero() || tp.RecordedAt.Before(firstBatchPt) {
						firstBatchPt = tp.RecordedAt
					}
				}
			}
			if err = backfillTripPoints(ctx, tx, trip.ID, trip.UserID, trip.DeviceID, since, firstBatchPt); err != nil {
				return err
			}
		}
	}

	// 3. INSERT current-batch trip_points (ON CONFLICT DO NOTHING for idempotency).
	if err = insertTripPoints(ctx, tx, batch.NewPoints); err != nil {
		return err
	}

	// 4. UPDATE active trips stats.
	for _, u := range batch.UpdatedTrips {
		if err = updateTripStats(ctx, tx, u); err != nil {
			return err
		}
	}

	// 5. COMPLETE trips.
	for _, c := range batch.CompletedTrips {
		if err = completeTrip(ctx, tx, c); err != nil {
			return err
		}
	}

	// 6. UPSERT detection state.
	if err = upsertDetectionState(ctx, tx, batch.NewState); err != nil {
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("trip_repo: commit: %w", err)
	}
	return nil
}

func insertTrip(ctx context.Context, tx pgx.Tx, t *domain.Trip) error {
	const q = `
		INSERT INTO trips (
			id, user_id, device_id,
			status, started_at,
			start_lat, start_lon,
			distance_m, duration_sec, points_count,
			created_at, updated_at
		) VALUES (
			$1, $2, $3,
			$4, $5,
			$6, $7,
			$8, $9, $10,
			$11, $12
		) ON CONFLICT (id) DO NOTHING`

	_, err := tx.Exec(ctx, q,
		t.ID, t.UserID, t.DeviceID,
		string(t.Status), t.StartedAt,
		t.StartLat, t.StartLon,
		t.DistanceM, t.DurationSec, t.PointsCount,
		t.CreatedAt, t.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("trip_repo: insert trip %s: %w", t.ID, err)
	}
	return nil
}

func backfillTripPoints(ctx context.Context, tx pgx.Tx, tripID, userID, deviceID uuid.UUID, since, before time.Time) error {
	const q = `
		INSERT INTO trip_points (trip_id, user_id, device_id, event_id, recorded_at)
		SELECT $1, user_id, device_id, event_id, recorded_at
		FROM raw_location_points
		WHERE user_id = $2
		  AND device_id = $3
		  AND recorded_at >= $4
		  AND recorded_at < $5
		ON CONFLICT DO NOTHING`

	_, err := tx.Exec(ctx, q, tripID, userID, deviceID, since, before)
	if err != nil {
		return fmt.Errorf("trip_repo: backfill points trip %s: %w", tripID, err)
	}
	return nil
}

func insertTripPoints(ctx context.Context, tx pgx.Tx, points []domain.TripPoint) error {
	if len(points) == 0 {
		return nil
	}
	const q = `
		INSERT INTO trip_points (trip_id, user_id, device_id, event_id, recorded_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT DO NOTHING`

	for _, tp := range points {
		if _, err := tx.Exec(ctx, q, tp.TripID, tp.UserID, tp.DeviceID, tp.EventID, tp.RecordedAt); err != nil {
			return fmt.Errorf("trip_repo: insert trip_point %s: %w", tp.EventID, err)
		}
	}
	return nil
}

func updateTripStats(ctx context.Context, tx pgx.Tx, u domain.TripStatsDelta) error {
	const q = `
		UPDATE trips SET
			distance_m   = distance_m + $2,
			points_count = points_count + $3,
			duration_sec = CASE
				WHEN $4::timestamptz IS NOT NULL
				THEN EXTRACT(EPOCH FROM $4::timestamptz - started_at)::bigint
				ELSE duration_sec
			END,
			end_lat      = COALESCE($5, end_lat),
			end_lon      = COALESCE($6, end_lon),
			updated_at   = now()
		WHERE id = $1`

	_, err := tx.Exec(ctx, q,
		u.TripID, u.DeltaDistM, u.DeltaPts,
		u.LastPtAt, u.LastLat, u.LastLon,
	)
	if err != nil {
		return fmt.Errorf("trip_repo: update trip %s: %w", u.TripID, err)
	}
	return nil
}

func completeTrip(ctx context.Context, tx pgx.Tx, c domain.TripCompletion) error {
	const q = `
		UPDATE trips SET
			status     = 'TRIP_COMPLETED',
			ended_at   = $2,
			end_lat    = $3,
			end_lon    = $4,
			duration_sec = EXTRACT(EPOCH FROM $2 - started_at)::bigint,
			updated_at = now()
		WHERE id = $1`

	_, err := tx.Exec(ctx, q, c.TripID, c.EndedAt, c.EndLat, c.EndLon)
	if err != nil {
		return fmt.Errorf("trip_repo: complete trip %s: %w", c.TripID, err)
	}
	return nil
}

func upsertDetectionState(ctx context.Context, tx pgx.Tx, s domain.TripDetectionState) error {
	const q = `
		INSERT INTO trip_detection_state (
			user_id, device_id,
			state, active_trip_id,
			candidate_started_at, candidate_start_point_id,
			candidate_distance_m, candidate_start_lat, candidate_start_lon,
			stop_started_at, stop_center_lat, stop_center_lon,
			last_point_lat, last_point_lon,
			last_processed_recorded_at,
			last_watermark_at, late_arrival_window_sec,
			updated_at
		) VALUES (
			$1,  $2,
			$3,  $4,
			$5,  $6,
			$7,  $8,  $9,
			$10, $11, $12,
			$13, $14,
			$15,
			$16, $17,
			$18
		)
		ON CONFLICT (user_id, device_id) DO UPDATE SET
			state                       = EXCLUDED.state,
			active_trip_id              = EXCLUDED.active_trip_id,
			candidate_started_at        = EXCLUDED.candidate_started_at,
			candidate_start_point_id    = EXCLUDED.candidate_start_point_id,
			candidate_distance_m        = EXCLUDED.candidate_distance_m,
			candidate_start_lat         = EXCLUDED.candidate_start_lat,
			candidate_start_lon         = EXCLUDED.candidate_start_lon,
			stop_started_at             = EXCLUDED.stop_started_at,
			stop_center_lat             = EXCLUDED.stop_center_lat,
			stop_center_lon             = EXCLUDED.stop_center_lon,
			last_point_lat              = EXCLUDED.last_point_lat,
			last_point_lon              = EXCLUDED.last_point_lon,
			last_processed_recorded_at  = EXCLUDED.last_processed_recorded_at,
			last_watermark_at           = EXCLUDED.last_watermark_at,
			late_arrival_window_sec     = EXCLUDED.late_arrival_window_sec,
			updated_at                  = EXCLUDED.updated_at`

	_, err := tx.Exec(ctx, q,
		s.UserID, s.DeviceID,
		string(s.State), s.ActiveTripID,
		s.CandidateStartedAt, s.CandidateStartPointID,
		s.CandidateDistanceM, s.CandidateStartLat, s.CandidateStartLon,
		s.StopStartedAt, s.StopCenterLat, s.StopCenterLon,
		s.LastPointLat, s.LastPointLon,
		s.LastProcessedAt,
		s.LastWatermarkAt, s.LateArrivalWindowSec,
		s.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("trip_repo: upsert state %s/%s: %w", s.UserID, s.DeviceID, err)
	}
	return nil
}
