package postgres

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wrany/tracking-gateway/internal/domain"
	"github.com/wrany/tracking-gateway/internal/usecase"
)

// RouteResultQueryRepo implements usecase.RouteResultRepo.
type RouteResultQueryRepo struct {
	db *pgxpool.Pool
}

func NewRouteResultQueryRepo(db *pgxpool.Pool) *RouteResultQueryRepo {
	return &RouteResultQueryRepo{db: db}
}

var _ usecase.RouteResultRepo = (*RouteResultQueryRepo)(nil)

// GetRouteResult computes personal records for a route in a single CTE query.
// Returns attempts_count=0, Best=nil, Latest=nil when the route has no completed attempts.
func (r *RouteResultQueryRepo) GetRouteResult(ctx context.Context, routeID string) (domain.RouteResult, error) {
	const q = `
		WITH ranked AS (
			SELECT
				rt.trip_id,
				t.started_at,
				rt.duration_sec,
				rt.distance_m,
				COUNT(*) OVER ()                                                                  AS attempts_count,
				ROW_NUMBER() OVER (ORDER BY rt.duration_sec ASC, t.started_at ASC, rt.trip_id ASC) AS best_rank,
				ROW_NUMBER() OVER (ORDER BY t.started_at DESC, rt.trip_id ASC)                     AS latest_rank
			FROM route_trips rt
			JOIN trips t ON t.id = rt.trip_id
			WHERE rt.route_id = $1
			  AND t.status    = 'TRIP_COMPLETED'
			  AND rt.duration_sec > 0
		)
		SELECT trip_id, started_at, duration_sec, distance_m, attempts_count, best_rank, latest_rank
		FROM ranked
		WHERE best_rank = 1 OR latest_rank = 1`

	rows, err := r.db.Query(ctx, q, routeID)
	if err != nil {
		return domain.RouteResult{}, fmt.Errorf("route_result_repo: get route result: %w", err)
	}
	defer rows.Close()

	result := domain.RouteResult{RouteID: routeID}

	for rows.Next() {
		var (
			tripID       string
			startedAt    time.Time
			durationSec  int64
			distanceM    float64
			attemptsCount int
			bestRank     int64
			latestRank   int64
		)
		if err := rows.Scan(&tripID, &startedAt, &durationSec, &distanceM, &attemptsCount, &bestRank, &latestRank); err != nil {
			return domain.RouteResult{}, fmt.Errorf("route_result_repo: scan: %w", err)
		}

		result.AttemptsCount = attemptsCount

		tr := &domain.TripResult{
			TripID:      tripID,
			StartedAt:   startedAt,
			DurationSec: durationSec,
			DistanceM:   distanceM,
			AvgSpeedMps: distanceM / float64(durationSec),
		}

		if bestRank == 1 {
			result.Best = tr
		}
		if latestRank == 1 {
			result.Latest = tr
		}
	}
	if err := rows.Err(); err != nil {
		return domain.RouteResult{}, fmt.Errorf("route_result_repo: rows: %w", err)
	}

	return result, nil
}

// ListRouteAttempts returns a keyset-paginated list of completed attempts for a route.
// Cursor encodes (matched_at, trip_id) to prevent skips/duplicates on equal timestamps.
func (r *RouteResultQueryRepo) ListRouteAttempts(ctx context.Context, f domain.TripAttemptFilter) ([]domain.TripAttempt, string, error) {
	bestTripID, err := r.getBestTripID(ctx, f.RouteID)
	if err != nil {
		return nil, "", err
	}

	var cursorTime time.Time
	var cursorID string
	if f.Cursor != "" {
		if dec, err2 := base64.StdEncoding.DecodeString(f.Cursor); err2 == nil {
			parts := strings.SplitN(string(dec), "|", 2)
			if len(parts) == 2 {
				cursorTime, _ = time.Parse(time.RFC3339Nano, parts[0])
				cursorID = parts[1]
			}
		}
	}

	args := []any{f.RouteID, f.UserID}
	where := "rt.route_id = $1 AND rt.user_id = $2 AND t.status = 'TRIP_COMPLETED' AND rt.duration_sec > 0"
	idx := 3

	if !cursorTime.IsZero() {
		where += fmt.Sprintf(
			" AND (rt.matched_at < $%d OR (rt.matched_at = $%d AND rt.trip_id::text < $%d))",
			idx, idx, idx+1,
		)
		args = append(args, cursorTime, cursorID)
		idx += 2
	}

	args = append(args, f.Limit+1)
	q := `
		SELECT rt.trip_id, t.started_at, t.ended_at, rt.duration_sec, rt.distance_m,
		       rt.match_score, rt.matched_at
		FROM route_trips rt
		JOIN trips t ON t.id = rt.trip_id
		WHERE ` + where + `
		ORDER BY rt.matched_at DESC, rt.trip_id DESC
		LIMIT $` + fmt.Sprintf("%d", idx)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, "", fmt.Errorf("route_result_repo: list attempts: %w", err)
	}
	defer rows.Close()

	var items []domain.TripAttempt
	for rows.Next() {
		var a domain.TripAttempt
		if err := rows.Scan(
			&a.TripID, &a.StartedAt, &a.EndedAt,
			&a.DurationSec, &a.DistanceM,
			&a.MatchScore, &a.MatchedAt,
		); err != nil {
			return nil, "", fmt.Errorf("route_result_repo: scan attempt: %w", err)
		}
		if a.DurationSec > 0 {
			a.AvgSpeedMps = a.DistanceM / float64(a.DurationSec)
		}
		a.IsBest = (a.TripID == bestTripID)
		items = append(items, a)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("route_result_repo: list attempts rows: %w", err)
	}

	var nextCursor string
	if len(items) > f.Limit {
		items = items[:f.Limit]
		last := items[len(items)-1]
		raw := last.MatchedAt.UTC().Format(time.RFC3339Nano) + "|" + last.TripID
		nextCursor = base64.StdEncoding.EncodeToString([]byte(raw))
	}

	return items, nextCursor, nil
}

// getBestTripID returns the trip_id with the minimum duration_sec for a route.
// Returns an empty string when there are no completed attempts.
func (r *RouteResultQueryRepo) getBestTripID(ctx context.Context, routeID string) (string, error) {
	const q = `
		SELECT rt.trip_id
		FROM route_trips rt
		JOIN trips t ON t.id = rt.trip_id
		WHERE rt.route_id = $1
		  AND t.status    = 'TRIP_COMPLETED'
		  AND rt.duration_sec > 0
		ORDER BY rt.duration_sec ASC, t.started_at ASC, rt.trip_id ASC
		LIMIT 1`

	var tripID string
	err := r.db.QueryRow(ctx, q, routeID).Scan(&tripID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("route_result_repo: get best trip id: %w", err)
	}
	return tripID, nil
}
