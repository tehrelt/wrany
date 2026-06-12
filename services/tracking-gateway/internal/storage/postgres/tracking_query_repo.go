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

// GetTrack returns a simplified track: stationary clusters are collapsed into
// a single centroid (kind="stay"), moving points are returned individually (kind="move").
// Points with speed_mps < 0.5 m/s or activity_type = 'stationary' are treated as stationary.
func (r *TrackingQueryRepo) GetTrack(
	ctx context.Context,
	f domain.TrackFilter,
) ([]domain.TrackSegment, error) {
	args := []any{f.UserID, f.From, f.To, f.SpeedThresholdMps}
	argIdx := 5
	deviceClause := ""
	if f.DeviceID != "" {
		deviceClause = "AND device_id = $" + strconv.Itoa(argIdx)
		args = append(args, f.DeviceID)
		argIdx++
	}
	minMoveArg := strconv.Itoa(argIdx)
	args = append(args, f.MinMoveSec)
	argIdx++
	minStayArg := strconv.Itoa(argIdx)
	args = append(args, f.MinStaySec)

	// Two-pass island detection:
	// Pass 1 — classify by speed threshold, group consecutive same-state runs.
	// Pass 2 — reclassify move groups shorter than min_move_sec as stationary, re-group.
	// Final — aggregate stay groups (with HAVING min_stay_sec), keep move points as-is.
	query := `
		WITH classified AS (
			SELECT event_id, recorded_at, lat, lon, speed_mps, accuracy_m,
				(COALESCE(speed_mps, 0) < $4 OR activity_type = 'stationary') AS is_stationary
			FROM raw_location_points
			WHERE user_id = $1 AND recorded_at >= $2 AND recorded_at <= $3 ` + deviceClause + `
		),
		bordered1 AS (
			SELECT *,
				(is_stationary IS DISTINCT FROM LAG(is_stationary) OVER (ORDER BY recorded_at)) AS is_border
			FROM classified
		),
		grouped1 AS (
			SELECT *,
				SUM(is_border::int) OVER (ORDER BY recorded_at ROWS UNBOUNDED PRECEDING) AS grp
			FROM bordered1
		),
		move_durations AS (
			SELECT grp,
				EXTRACT(EPOCH FROM (MAX(recorded_at) - MIN(recorded_at)))::int AS dur_sec
			FROM grouped1
			WHERE NOT is_stationary
			GROUP BY grp
		),
		reclassified AS (
			SELECT g.*,
				CASE
					WHEN NOT g.is_stationary AND COALESCE(md.dur_sec, 0) < $` + minMoveArg + `
						THEN true
					ELSE g.is_stationary
				END AS is_stationary2
			FROM grouped1 g
			LEFT JOIN move_durations md ON md.grp = g.grp
		),
		bordered2 AS (
			SELECT *,
				(is_stationary2 IS DISTINCT FROM LAG(is_stationary2) OVER (ORDER BY recorded_at)) AS is_border2
			FROM reclassified
		),
		grouped2 AS (
			SELECT *,
				SUM(is_border2::int) OVER (ORDER BY recorded_at ROWS UNBOUNDED PRECEDING) AS grp2
			FROM bordered2
		)
		SELECT 'stay'::text AS kind,
			''::text AS event_id,
			MIN(recorded_at) AS recorded_at,
			MAX(recorded_at) AS period_end,
			AVG(lat) AS lat, AVG(lon) AS lon,
			NULL::float8 AS speed_mps, NULL::float8 AS accuracy_m,
			EXTRACT(EPOCH FROM (MAX(recorded_at) - MIN(recorded_at)))::int AS stay_duration_sec,
			COUNT(*)::int AS merged_count
		FROM grouped2
		WHERE is_stationary2
		GROUP BY grp2
		HAVING EXTRACT(EPOCH FROM (MAX(recorded_at) - MIN(recorded_at)))::int >= $` + minStayArg + `

		UNION ALL

		SELECT 'move'::text AS kind,
			event_id::text,
			recorded_at, recorded_at AS period_end,
			lat, lon,
			speed_mps, accuracy_m,
			0::int AS stay_duration_sec,
			1::int AS merged_count
		FROM grouped2
		WHERE NOT is_stationary2

		ORDER BY recorded_at
	`

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	segments := make([]domain.TrackSegment, 0)
	for rows.Next() {
		var s domain.TrackSegment
		var kindStr string
		if err := rows.Scan(
			&kindStr, &s.EventID,
			&s.RecordedAt, &s.PeriodEnd,
			&s.Lat, &s.Lon,
			&s.SpeedMps, &s.AccuracyM,
			&s.StayDurationSec, &s.MergedCount,
		); err != nil {
			return nil, err
		}
		s.Kind = domain.TrackSegmentKind(kindStr)
		segments = append(segments, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return segments, nil
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

