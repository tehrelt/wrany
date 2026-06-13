package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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
		LEFT JOIN processed_location_points plp
			ON plp.user_id = rlp.user_id
			AND plp.device_id = rlp.device_id
			AND plp.event_id = rlp.event_id
		LEFT JOIN trip_detection_state tds
			ON tds.user_id = rlp.user_id AND tds.device_id = rlp.device_id
		WHERE rlp.recorded_at < $1
		  AND plp.event_id IS NULL
		  AND rlp.recorded_at >= COALESCE(
		      tds.last_processed_recorded_at - make_interval(secs => tds.late_arrival_window_sec),
		      '-infinity'::timestamptz
		  )`

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
// Returns a default IDLE state when none is found.
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
			candidate_good_points,
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
		&s.CandidateGoodPoints,
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
				LateArrivalWindowSec: 45,
			}, nil
		}
		return domain.TripDetectionState{}, fmt.Errorf("trip_repo: load state: %w", err)
	}
	s.State = domain.TripState(stateStr)
	return s, nil
}

// FetchPoints returns raw_location_points in [from, to) for a pair, ordered by recorded_at ASC.
func (r *TripRepo) FetchUnprocessedPoints(ctx context.Context, userID, deviceID uuid.UUID, from, to time.Time) ([]domain.RawLocationPoint, error) {
	const q = `
		SELECT
			user_id, device_id, event_id,
			recorded_at, received_at,
			lat, lon,
			accuracy_m, speed_mps, bearing_deg,
			activity_type, activity_confidence, battery_level,
			source
		FROM raw_location_points rlp
		WHERE rlp.user_id = $1
		  AND rlp.device_id = $2
		  AND rlp.recorded_at >= $3
		  AND rlp.recorded_at < $4
		  AND NOT EXISTS (
		      SELECT 1 FROM processed_location_points plp
		      WHERE plp.user_id = rlp.user_id
		        AND plp.device_id = rlp.device_id
		        AND plp.event_id = rlp.event_id
		  )
		ORDER BY rlp.recorded_at ASC, rlp.event_id ASC`

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

func (r *TripRepo) FetchPoints(ctx context.Context, userID, deviceID uuid.UUID, from, to time.Time) ([]domain.RawLocationPoint, error) {
	return r.FetchUnprocessedPoints(ctx, userID, deviceID, from, to)
}

func (r *TripRepo) FetchProcessedHistory(ctx context.Context, userID, deviceID uuid.UUID, before time.Time, limit int) ([]domain.ProcessedLocationPoint, error) {
	const q = `
		SELECT plp.user_id, plp.device_id, plp.event_id,
		       plp.raw_lat, plp.raw_lon, plp.filtered_lat, plp.filtered_lon,
		       plp.accuracy_m, plp.speed_mps, plp.implied_speed_mps, plp.distance_delta_m,
		       COALESCE(rlp.activity_type, 'unknown'), rlp.activity_confidence,
		       plp.is_accepted, plp.is_outlier, plp.is_stationary, plp.noise_reason,
		       plp.stationary_since,
		       plp.recorded_at, plp.received_at, plp.processed_at
		FROM processed_location_points plp
		JOIN raw_location_points rlp
		  ON rlp.user_id = plp.user_id
		 AND rlp.device_id = plp.device_id
		 AND rlp.event_id = plp.event_id
		WHERE plp.user_id = $1
		  AND plp.device_id = $2
		  AND plp.recorded_at < $3
		  AND plp.is_accepted
		ORDER BY plp.recorded_at DESC, plp.event_id DESC
		LIMIT $4`
	rows, err := r.db.Query(ctx, q, userID, deviceID, before, limit)
	if err != nil {
		return nil, fmt.Errorf("trip_repo: fetch history: %w", err)
	}
	defer rows.Close()
	var reversed []domain.ProcessedLocationPoint
	for rows.Next() {
		var point domain.ProcessedLocationPoint
		if err := rows.Scan(
			&point.UserID, &point.DeviceID, &point.EventID,
			&point.RawLat, &point.RawLon, &point.FilteredLat, &point.FilteredLon,
			&point.AccuracyM, &point.SpeedMps, &point.ImpliedSpeedMps, &point.DistanceDeltaM,
			&point.ActivityType, &point.ActivityConfidence,
			&point.IsAccepted, &point.IsOutlier, &point.IsStationary, &point.NoiseReason,
			&point.StationarySince,
			&point.RecordedAt, &point.ReceivedAt, &point.ProcessedAt,
		); err != nil {
			return nil, fmt.Errorf("trip_repo: scan history: %w", err)
		}
		reversed = append(reversed, point)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	return reversed, nil
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

	if err = insertProcessedPoints(ctx, tx, batch.ProcessedPoints); err != nil {
		return err
	}

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
		FROM processed_location_points
		WHERE user_id = $2
		  AND device_id = $3
		  AND recorded_at >= $4
		  AND recorded_at < $5
		  AND is_accepted
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
			candidate_good_points,
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
			$10,
			$11, $12, $13,
			$14, $15,
			$16,
			$17, $18,
			$19
		)
		ON CONFLICT (user_id, device_id) DO UPDATE SET
			state                       = EXCLUDED.state,
			active_trip_id              = EXCLUDED.active_trip_id,
			candidate_started_at        = EXCLUDED.candidate_started_at,
			candidate_start_point_id    = EXCLUDED.candidate_start_point_id,
			candidate_distance_m        = EXCLUDED.candidate_distance_m,
			candidate_start_lat         = EXCLUDED.candidate_start_lat,
			candidate_start_lon         = EXCLUDED.candidate_start_lon,
			candidate_good_points       = EXCLUDED.candidate_good_points,
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
		s.CandidateGoodPoints,
		s.StopStartedAt, s.StopCenterLat, s.StopCenterLon,
		s.LastPointLat, s.LastPointLon, s.LastProcessedAt,
		s.LastWatermarkAt, s.LateArrivalWindowSec, s.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("trip_repo: upsert state %s/%s: %w", s.UserID, s.DeviceID, err)
	}
	return nil
}

func insertProcessedPoints(ctx context.Context, tx pgx.Tx, points []domain.ProcessedLocationPoint) error {
	const q = `
		INSERT INTO processed_location_points (
			user_id, device_id, event_id,
			raw_lat, raw_lon, filtered_lat, filtered_lon, filtered_geom,
			accuracy_m, speed_mps, implied_speed_mps, distance_delta_m,
			is_accepted, is_outlier, is_stationary, noise_reason, stationary_since,
			recorded_at, received_at, processed_at, algorithm_version
		) VALUES (
			$1, $2, $3,
			$4, $5, $6, $7,
			CASE WHEN $6::double precision IS NULL OR $7::double precision IS NULL
			     THEN NULL
			     ELSE ST_SetSRID(ST_MakePoint($7, $6), 4326)
			END,
			$8, $9, $10, $11,
			$12, $13, $14, $15, $16,
			$17, $18, $19, $20
		) ON CONFLICT (user_id, device_id, event_id) DO NOTHING`
	for _, point := range points {
		if _, err := tx.Exec(ctx, q,
			point.UserID, point.DeviceID, point.EventID,
			point.RawLat, point.RawLon, point.FilteredLat, point.FilteredLon,
			point.AccuracyM, point.SpeedMps, point.ImpliedSpeedMps, point.DistanceDeltaM,
			point.IsAccepted, point.IsOutlier, point.IsStationary, string(point.NoiseReason),
			point.StationarySince,
			point.RecordedAt, point.ReceivedAt, point.ProcessedAt, point.AlgorithmVersion,
		); err != nil {
			return fmt.Errorf("trip_repo: insert processed point %s: %w", point.EventID, err)
		}
	}
	return nil
}

// FetchPointsForReprocessing returns raw points in [from, to) for a pair whose
// processing result is either missing OR was produced by an algorithm version
// older than currentVersion. Ordered by recorded_at ASC, event_id ASC.
//
// Unlike FetchUnprocessedPoints (which only ever returns never-processed points),
// this path is for re-running the noise pipeline after the algorithm is bumped.
//
// TODO(reprocess): this only recomputes processed_location_points. A full
// historical reprocess must also rebuild affected trips/trip_points — that is a
// separate task and intentionally not done here.
func (r *TripRepo) FetchPointsForReprocessing(ctx context.Context, userID, deviceID uuid.UUID, from, to time.Time, currentVersion int16) ([]domain.RawLocationPoint, error) {
	const q = `
		SELECT
			rlp.user_id, rlp.device_id, rlp.event_id,
			rlp.recorded_at, rlp.received_at,
			rlp.lat, rlp.lon,
			rlp.accuracy_m, rlp.speed_mps, rlp.bearing_deg,
			rlp.activity_type, rlp.activity_confidence, rlp.battery_level,
			rlp.source
		FROM raw_location_points rlp
		LEFT JOIN processed_location_points plp
		       ON plp.user_id = rlp.user_id
		      AND plp.device_id = rlp.device_id
		      AND plp.event_id = rlp.event_id
		WHERE rlp.user_id = $1
		  AND rlp.device_id = $2
		  AND rlp.recorded_at >= $3
		  AND rlp.recorded_at < $4
		  AND (plp.event_id IS NULL OR plp.algorithm_version < $5)
		ORDER BY rlp.recorded_at ASC, rlp.event_id ASC`

	rows, err := r.db.Query(ctx, q, userID, deviceID, from, to, currentVersion)
	if err != nil {
		return nil, fmt.Errorf("trip_repo: fetch reprocess points: %w", err)
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
			return nil, fmt.Errorf("trip_repo: scan reprocess point: %w", err)
		}
		pts = append(pts, p)
	}
	return pts, rows.Err()
}

// UpsertProcessedPoints inserts new processing results or updates existing ones
// in place, but only when the stored algorithm_version is strictly older than the
// incoming one. Derived fields, noise flags, algorithm_version and processed_at are
// refreshed; raw_lat/raw_lon are left untouched (they mirror the immutable raw row).
//
// This is the write side of the reprocess path. The normal detection batch keeps
// using insertProcessedPoints (ON CONFLICT DO NOTHING) so a re-run never clobbers a
// result that is already at the current version.
func (r *TripRepo) UpsertProcessedPoints(ctx context.Context, points []domain.ProcessedLocationPoint) error {
	const q = `
		INSERT INTO processed_location_points (
			user_id, device_id, event_id,
			raw_lat, raw_lon, filtered_lat, filtered_lon, filtered_geom,
			accuracy_m, speed_mps, implied_speed_mps, distance_delta_m,
			is_accepted, is_outlier, is_stationary, noise_reason, stationary_since,
			recorded_at, received_at, processed_at, algorithm_version
		) VALUES (
			$1, $2, $3,
			$4, $5, $6, $7,
			CASE WHEN $6::double precision IS NULL OR $7::double precision IS NULL
			     THEN NULL
			     ELSE ST_SetSRID(ST_MakePoint($7, $6), 4326)
			END,
			$8, $9, $10, $11,
			$12, $13, $14, $15, $16,
			$17, $18, $19, $20
		)
		ON CONFLICT (user_id, device_id, event_id) DO UPDATE SET
			filtered_lat      = EXCLUDED.filtered_lat,
			filtered_lon      = EXCLUDED.filtered_lon,
			filtered_geom     = EXCLUDED.filtered_geom,
			implied_speed_mps = EXCLUDED.implied_speed_mps,
			distance_delta_m  = EXCLUDED.distance_delta_m,
			is_accepted       = EXCLUDED.is_accepted,
			is_outlier        = EXCLUDED.is_outlier,
			is_stationary     = EXCLUDED.is_stationary,
			noise_reason      = EXCLUDED.noise_reason,
			stationary_since  = EXCLUDED.stationary_since,
			processed_at      = EXCLUDED.processed_at,
			algorithm_version = EXCLUDED.algorithm_version
		WHERE processed_location_points.algorithm_version < EXCLUDED.algorithm_version`
	for _, point := range points {
		if _, err := r.db.Exec(ctx, q,
			point.UserID, point.DeviceID, point.EventID,
			point.RawLat, point.RawLon, point.FilteredLat, point.FilteredLon,
			point.AccuracyM, point.SpeedMps, point.ImpliedSpeedMps, point.DistanceDeltaM,
			point.IsAccepted, point.IsOutlier, point.IsStationary, string(point.NoiseReason),
			point.StationarySince,
			point.RecordedAt, point.ReceivedAt, point.ProcessedAt, point.AlgorithmVersion,
		); err != nil {
			return fmt.Errorf("trip_repo: upsert processed point %s: %w", point.EventID, err)
		}
	}
	return nil
}
