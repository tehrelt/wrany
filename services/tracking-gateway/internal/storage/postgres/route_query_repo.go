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

type RouteQueryRepo struct {
	db *pgxpool.Pool
}

func NewRouteQueryRepo(db *pgxpool.Pool) *RouteQueryRepo {
	return &RouteQueryRepo{db: db}
}

var _ usecase.RouteQueryRepo = (*RouteQueryRepo)(nil)

func (r *RouteQueryRepo) ListRoutes(ctx context.Context, f domain.RouteFilter) ([]domain.Route, string, error) {
	var cursorTime time.Time
	var cursorID string
	if f.Cursor != "" {
		if dec, err := base64.StdEncoding.DecodeString(f.Cursor); err == nil {
			parts := strings.SplitN(string(dec), "|", 2)
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
	if !cursorTime.IsZero() {
		conds = append(conds, fmt.Sprintf(
			"(updated_at < $%d OR (updated_at = $%d AND id::text > $%d))",
			idx, idx, idx+1,
		))
		args = append(args, cursorTime, cursorID)
		idx += 2
	}

	args = append(args, f.Limit+1)
	q := `SELECT id, user_id, name, status,
	             start_lat, start_lon, end_lat, end_lon,
	             distance_m, trips_count,
	             first_trip_id, last_trip_id,
	             created_at, updated_at
	      FROM routes
	      WHERE ` + strings.Join(conds, " AND ") + `
	      ORDER BY updated_at DESC, id ASC
	      LIMIT $` + fmt.Sprintf("%d", idx)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, "", fmt.Errorf("route_query_repo: list routes: %w", err)
	}
	defer rows.Close()

	var routes []domain.Route
	for rows.Next() {
		var rt domain.Route
		if err := rows.Scan(
			&rt.ID, &rt.UserID, &rt.Name, &rt.Status,
			&rt.StartLat, &rt.StartLon, &rt.EndLat, &rt.EndLon,
			&rt.DistanceM, &rt.TripsCount,
			&rt.FirstTripID, &rt.LastTripID,
			&rt.CreatedAt, &rt.UpdatedAt,
		); err != nil {
			return nil, "", fmt.Errorf("route_query_repo: scan route: %w", err)
		}
		routes = append(routes, rt)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("route_query_repo: list routes rows: %w", err)
	}

	var nextCursor string
	if len(routes) > f.Limit {
		routes = routes[:f.Limit]
		last := routes[len(routes)-1]
		raw := last.UpdatedAt.UTC().Format(time.RFC3339Nano) + "|" + last.ID
		nextCursor = base64.StdEncoding.EncodeToString([]byte(raw))
	}

	return routes, nextCursor, nil
}

func (r *RouteQueryRepo) GetRoute(ctx context.Context, routeID, userID string) (domain.Route, error) {
	const q = `
		SELECT id, user_id, name, status,
		       start_lat, start_lon, end_lat, end_lon,
		       distance_m, trips_count,
		       first_trip_id, last_trip_id,
		       created_at, updated_at
		FROM routes
		WHERE id = $1 AND user_id = $2`

	var rt domain.Route
	err := r.db.QueryRow(ctx, q, routeID, userID).Scan(
		&rt.ID, &rt.UserID, &rt.Name, &rt.Status,
		&rt.StartLat, &rt.StartLon, &rt.EndLat, &rt.EndLon,
		&rt.DistanceM, &rt.TripsCount,
		&rt.FirstTripID, &rt.LastTripID,
		&rt.CreatedAt, &rt.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Route{}, usecase.ErrRouteNotFound
	}
	if err != nil {
		return domain.Route{}, fmt.Errorf("route_query_repo: get route: %w", err)
	}
	return rt, nil
}

func (r *RouteQueryRepo) ListRouteTrips(ctx context.Context, f domain.RouteTripFilter) ([]domain.RouteTrip, string, error) {
	var cursorTime time.Time
	var cursorID string
	if f.Cursor != "" {
		if dec, err := base64.StdEncoding.DecodeString(f.Cursor); err == nil {
			parts := strings.SplitN(string(dec), "|", 2)
			if len(parts) == 2 {
				cursorTime, _ = time.Parse(time.RFC3339Nano, parts[0])
				cursorID = parts[1]
			}
		}
	}

	args := []any{f.RouteID, f.UserID}
	where := "rt.route_id = $1 AND rt.user_id = $2"
	idx := 3

	if !cursorTime.IsZero() {
		where += fmt.Sprintf(
			" AND (rt.matched_at < $%d OR (rt.matched_at = $%d AND rt.trip_id::text > $%d))",
			idx, idx, idx+1,
		)
		args = append(args, cursorTime, cursorID)
		idx += 2
	}

	args = append(args, f.Limit+1)
	q := `
		SELECT rt.route_id, rt.trip_id, rt.match_score, rt.matched_at,
		       rt.duration_sec, rt.distance_m,
		       t.started_at, t.ended_at
		FROM route_trips rt
		JOIN trips t ON t.id = rt.trip_id
		WHERE ` + where + `
		ORDER BY rt.matched_at DESC, rt.trip_id ASC
		LIMIT $` + fmt.Sprintf("%d", idx)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, "", fmt.Errorf("route_query_repo: list route trips: %w", err)
	}
	defer rows.Close()

	var items []domain.RouteTrip
	for rows.Next() {
		var item domain.RouteTrip
		if err := rows.Scan(
			&item.RouteID, &item.TripID, &item.MatchScore, &item.MatchedAt,
			&item.DurationSec, &item.DistanceM,
			&item.StartedAt, &item.EndedAt,
		); err != nil {
			return nil, "", fmt.Errorf("route_query_repo: scan route trip: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("route_query_repo: list route trips rows: %w", err)
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

func (r *RouteQueryRepo) GetRoutePoints(ctx context.Context, routeID, userID string) ([]domain.RoutePoint, error) {
	const q = `
		SELECT ST_Y((dp).geom) AS lat, ST_X((dp).geom) AS lon
		FROM routes CROSS JOIN ST_DumpPoints(template_geom) AS dp
		WHERE id = $1 AND user_id = $2
		ORDER BY (dp).path[1]`

	rows, err := r.db.Query(ctx, q, routeID, userID)
	if err != nil {
		return nil, fmt.Errorf("route_query_repo: get route points: %w", err)
	}
	defer rows.Close()

	var pts []domain.RoutePoint
	for rows.Next() {
		var p domain.RoutePoint
		if err := rows.Scan(&p.Lat, &p.Lon); err != nil {
			return nil, fmt.Errorf("route_query_repo: scan route point: %w", err)
		}
		pts = append(pts, p)
	}
	return pts, rows.Err()
}
