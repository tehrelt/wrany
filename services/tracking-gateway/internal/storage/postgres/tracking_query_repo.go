package postgres

import (
	"context"
	"encoding/base64"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wrany/tracking-gateway/internal/domain"
)

// TrackingQueryRepo queries raw_location_points (written by tracking-worker).
// All queries always filter by user_id to enforce data isolation.
type TrackingQueryRepo struct {
	db *pgxpool.Pool
}

func NewTrackingQueryRepo(db *pgxpool.Pool) *TrackingQueryRepo {
	return &TrackingQueryRepo{db: db}
}

// GetPoints returns a page of raw location points for the given filter.
// Returns the next cursor string (empty if no more pages).
func (r *TrackingQueryRepo) GetPoints(
	ctx context.Context,
	f domain.TrackingPointFilter,
) ([]domain.TrackingPoint, string, error) {
	// Decode cursor: base64("<recorded_at_rfc3339nano>|<event_id>")
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

	args := []any{f.UserID, f.From, f.To}
	where := "user_id = $1 AND recorded_at >= $2 AND recorded_at <= $3"
	argIdx := 4

	if f.DeviceID != "" {
		where += " AND device_id = $" + strconv.Itoa(argIdx)
		args = append(args, f.DeviceID)
		argIdx++
	}

	if !cursorTime.IsZero() {
		where += " AND (recorded_at > $" + strconv.Itoa(argIdx) +
			" OR (recorded_at = $" + strconv.Itoa(argIdx) + " AND event_id > $" + strconv.Itoa(argIdx+1) + "))"
		args = append(args, cursorTime, cursorEventID)
		argIdx += 2
	}

	limitArg := argIdx
	args = append(args, f.Limit+1) // fetch one extra to detect next page

	query := `
		SELECT event_id, device_id, recorded_at, lat, lon,
		       accuracy_m, speed_mps, bearing_deg, activity_type
		FROM raw_location_points
		WHERE ` + where + `
		ORDER BY recorded_at ASC, event_id ASC
		LIMIT $` + strconv.Itoa(limitArg)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var points []domain.TrackingPoint
	for rows.Next() {
		var p domain.TrackingPoint
		if err := rows.Scan(
			&p.EventID, &p.DeviceID, &p.RecordedAt,
			&p.Lat, &p.Lon,
			&p.AccuracyM, &p.SpeedMps, &p.BearingDeg, &p.ActivityType,
		); err != nil {
			return nil, "", err
		}
		points = append(points, p)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(points) > f.Limit {
		// trim the extra item; encode cursor from the last kept item
		points = points[:f.Limit]
		last := points[len(points)-1]
		raw := last.RecordedAt.UTC().Format(time.RFC3339Nano) + "|" + last.EventID
		nextCursor = base64.StdEncoding.EncodeToString([]byte(raw))
	}

	return points, nextCursor, nil
}

// DeletePoint removes a single raw location point owned by userID.
// Returns pgx.ErrNoRows (mapped upstream to ErrNotFound) if not found or not owned.
func (r *TrackingQueryRepo) DeletePoint(ctx context.Context, userID, eventID string) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM raw_location_points WHERE event_id = $1 AND user_id = $2`,
		eventID, userID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// GetSummary returns aggregated stats for the given filter.
func (r *TrackingQueryRepo) GetSummary(
	ctx context.Context,
	f domain.TrackingPointFilter,
) (domain.TrackingSummary, error) {
	args := []any{f.UserID, f.From, f.To}
	where := "user_id = $1 AND recorded_at >= $2 AND recorded_at <= $3"

	if f.DeviceID != "" {
		where += " AND device_id = $4"
		args = append(args, f.DeviceID)
	}

	query := `
		SELECT
			COUNT(*)                                                   AS points_count,
			MIN(recorded_at)                                           AS first_recorded_at,
			MAX(recorded_at)                                           AS last_recorded_at,
			COALESCE(
				EXTRACT(EPOCH FROM (MAX(recorded_at) - MIN(recorded_at))),
				0
			)::BIGINT                                                  AS duration_sec,
			AVG(speed_mps)                                             AS avg_speed_mps,
			MAX(speed_mps)                                             AS max_speed_mps
		FROM raw_location_points
		WHERE ` + where

	var s domain.TrackingSummary
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&s.PointsCount,
		&s.FirstRecordedAt,
		&s.LastRecordedAt,
		&s.DurationSec,
		&s.AvgSpeedMps,
		&s.MaxSpeedMps,
	)
	if err != nil {
		return domain.TrackingSummary{}, err
	}
	return s, nil
}

