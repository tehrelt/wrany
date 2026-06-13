package postgres

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wrany/tracking-gateway/internal/domain"
	"github.com/wrany/tracking-gateway/internal/usecase"
)

// TripQueryRepo queries the trips and trip_points tables written by tracking-worker.
// All queries filter by user_id to enforce data isolation.
type TripQueryRepo struct {
	db *pgxpool.Pool
}

func NewTripQueryRepo(db *pgxpool.Pool) *TripQueryRepo {
	return &TripQueryRepo{db: db}
}

// ListTrips returns a page of trips for the user, ordered by started_at DESC.
// Cursor is base64(started_at_rfc3339nano|trip_id).
func (r *TripQueryRepo) ListTrips(ctx context.Context, f domain.TripFilter) ([]domain.Trip, string, error) {
	var cursorTime time.Time
	var cursorID string
	if f.Cursor != "" {
		decoded, err := base64.StdEncoding.DecodeString(f.Cursor)
		if err == nil {
			parts := strings.SplitN(string(decoded), "|", 2)
			if len(parts) == 2 {
				cursorTime, _ = time.Parse(time.RFC3339Nano, parts[0])
				cursorID = parts[1]
			}
		}
	}

	args := []any{f.UserID}
	conds := []string{"user_id = $1"}
	idx := 2

	if f.DeviceID != "" {
		conds = append(conds, fmt.Sprintf("device_id = $%d", idx))
		args = append(args, f.DeviceID)
		idx++
	}
	if f.Status != "" {
		conds = append(conds, fmt.Sprintf("status = $%d", idx))
		args = append(args, string(f.Status))
		idx++
	}
	if !cursorTime.IsZero() {
		conds = append(conds, fmt.Sprintf(
			"(started_at < $%d OR (started_at = $%d AND id::text > $%d))",
			idx, idx, idx+1,
		))
		args = append(args, cursorTime, cursorID)
		idx += 2
	}

	args = append(args, f.Limit+1)
	limitIdx := idx

	q := `SELECT id, user_id, device_id, status,
	             started_at, ended_at,
	             start_lat, start_lon, end_lat, end_lon,
	             distance_m, duration_sec, points_count,
	             created_at, updated_at
	      FROM trips
	      WHERE ` + strings.Join(conds, " AND ") + `
	      ORDER BY started_at DESC, id ASC
	      LIMIT $` + fmt.Sprintf("%d", limitIdx)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, "", fmt.Errorf("trip_query_repo: list trips: %w", err)
	}
	defer rows.Close()

	var trips []domain.Trip
	for rows.Next() {
		var t domain.Trip
		var statusStr string
		if err := rows.Scan(
			&t.ID, &t.UserID, &t.DeviceID, &statusStr,
			&t.StartedAt, &t.EndedAt,
			&t.StartLat, &t.StartLon, &t.EndLat, &t.EndLon,
			&t.DistanceM, &t.DurationSec, &t.PointsCount,
			&t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, "", fmt.Errorf("trip_query_repo: scan trip: %w", err)
		}
		t.Status = domain.TripStatus(statusStr)
		trips = append(trips, t)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("trip_query_repo: list trips rows: %w", err)
	}

	var nextCursor string
	if len(trips) > f.Limit {
		trips = trips[:f.Limit]
		last := trips[len(trips)-1]
		raw := last.StartedAt.UTC().Format(time.RFC3339Nano) + "|" + last.ID
		nextCursor = base64.StdEncoding.EncodeToString([]byte(raw))
	}

	return trips, nextCursor, nil
}

// GetTrip returns a single trip by ID, verifying user ownership.
// Returns usecase.ErrTripNotFound if not found or not owned by userID.
func (r *TripQueryRepo) GetTrip(ctx context.Context, userID, tripID string) (domain.Trip, error) {
	const q = `
		SELECT id, user_id, device_id, status,
		       started_at, ended_at,
		       start_lat, start_lon, end_lat, end_lon,
		       distance_m, duration_sec, points_count,
		       created_at, updated_at
		FROM trips
		WHERE id = $1 AND user_id = $2`

	var t domain.Trip
	var statusStr string
	err := r.db.QueryRow(ctx, q, tripID, userID).Scan(
		&t.ID, &t.UserID, &t.DeviceID, &statusStr,
		&t.StartedAt, &t.EndedAt,
		&t.StartLat, &t.StartLon, &t.EndLat, &t.EndLon,
		&t.DistanceM, &t.DurationSec, &t.PointsCount,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.Trip{}, usecase.ErrTripNotFound
		}
		return domain.Trip{}, fmt.Errorf("trip_query_repo: get trip: %w", err)
	}
	t.Status = domain.TripStatus(statusStr)
	return t, nil
}

// DeleteTrip removes a trip owned by userID inside a transaction.
//
// The trips table is referenced by routes.first_trip_id / last_trip_id without
// ON DELETE CASCADE, so any route seeded by this trip is removed first (which
// cascades its route_trips links). Deleting the trip then cascades trip_points
// and any remaining route_trips links. Returns usecase.ErrTripNotFound if the
// trip does not exist or is not owned by userID.
func (r *TripQueryRepo) DeleteTrip(ctx context.Context, userID, tripID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("trip_query_repo: begin delete trip: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`DELETE FROM routes
		 WHERE user_id = $2 AND (first_trip_id = $1 OR last_trip_id = $1)`,
		tripID, userID,
	); err != nil {
		return fmt.Errorf("trip_query_repo: delete seeded routes: %w", err)
	}

	tag, err := tx.Exec(ctx,
		`DELETE FROM trips WHERE id = $1 AND user_id = $2`,
		tripID, userID,
	)
	if err != nil {
		return fmt.Errorf("trip_query_repo: delete trip: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return usecase.ErrTripNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("trip_query_repo: commit delete trip: %w", err)
	}
	return nil
}

// GetTripPoints returns paginated GPS points for a trip, ordered by recorded_at ASC.
// Cursor is base64(recorded_at_rfc3339nano|event_id).
// Points are joined from raw_location_points to obtain lat/lon.
func (r *TripQueryRepo) GetTripPoints(ctx context.Context, f domain.TripPointFilter) ([]domain.TripPoint, string, error) {
	var cursorTime time.Time
	var cursorEventID string
	if f.Cursor != "" {
		decoded, err := base64.StdEncoding.DecodeString(f.Cursor)
		if err == nil {
			parts := strings.SplitN(string(decoded), "|", 2)
			if len(parts) == 2 {
				cursorTime, _ = time.Parse(time.RFC3339Nano, parts[0])
				cursorEventID = parts[1]
			}
		}
	}

	args := []any{f.TripID, f.UserID}
	where := "tp.trip_id = $1 AND tp.user_id = $2"
	idx := 3

	if !cursorTime.IsZero() {
		where += fmt.Sprintf(
			" AND (tp.recorded_at > $%d OR (tp.recorded_at = $%d AND tp.event_id > $%d))",
			idx, idx, idx+1,
		)
		args = append(args, cursorTime, cursorEventID)
		idx += 2
	}

	args = append(args, f.Limit+1)
	q := `
		SELECT tp.event_id, tp.trip_id, tp.recorded_at, rlp.lat, rlp.lon
		FROM trip_points tp
		JOIN raw_location_points rlp
		  ON rlp.event_id = tp.event_id
		 AND rlp.user_id  = tp.user_id
		 AND rlp.device_id = tp.device_id
		WHERE ` + where + `
		ORDER BY tp.recorded_at ASC, tp.event_id ASC
		LIMIT $` + fmt.Sprintf("%d", idx)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, "", fmt.Errorf("trip_query_repo: get trip points: %w", err)
	}
	defer rows.Close()

	var pts []domain.TripPoint
	for rows.Next() {
		var p domain.TripPoint
		if err := rows.Scan(&p.EventID, &p.TripID, &p.RecordedAt, &p.Lat, &p.Lon); err != nil {
			return nil, "", fmt.Errorf("trip_query_repo: scan trip point: %w", err)
		}
		pts = append(pts, p)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("trip_query_repo: get trip points rows: %w", err)
	}

	var nextCursor string
	if len(pts) > f.Limit {
		pts = pts[:f.Limit]
		last := pts[len(pts)-1]
		raw := last.RecordedAt.UTC().Format(time.RFC3339Nano) + "|" + last.EventID
		nextCursor = base64.StdEncoding.EncodeToString([]byte(raw))
	}

	return pts, nextCursor, nil
}
