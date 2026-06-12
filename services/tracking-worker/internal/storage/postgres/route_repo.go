package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wrany/tracking-worker/internal/domain"
	"github.com/wrany/tracking-worker/internal/usecase"
)

type RouteRepo struct {
	db *pgxpool.Pool
}

func NewRouteRepo(db *pgxpool.Pool) *RouteRepo {
	return &RouteRepo{db: db}
}

var _ usecase.RouteMatchingRepository = (*RouteRepo)(nil)

func (r *RouteRepo) FindUnmatchedTrips(ctx context.Context, minPoints, limit int) ([]domain.Trip, error) {
	const q = `
		SELECT t.id, t.user_id, t.device_id, t.status,
		       t.started_at, t.ended_at,
		       t.start_lat, t.start_lon, t.end_lat, t.end_lon,
		       t.distance_m, t.duration_sec, t.points_count,
		       t.created_at, t.updated_at
		FROM trips t
		LEFT JOIN route_trips rt ON rt.trip_id = t.id
		WHERE t.status = 'TRIP_COMPLETED'
		  AND rt.trip_id IS NULL
		  AND t.points_count >= $1
		ORDER BY t.created_at ASC
		LIMIT $2`

	rows, err := r.db.Query(ctx, q, minPoints, limit)
	if err != nil {
		return nil, fmt.Errorf("route_repo: find unmatched trips: %w", err)
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
			return nil, fmt.Errorf("route_repo: scan trip: %w", err)
		}
		t.Status = domain.TripStatus(statusStr)
		trips = append(trips, t)
	}
	return trips, rows.Err()
}

func (r *RouteRepo) FindTripPointsWithCoords(ctx context.Context, tripID uuid.UUID) ([]domain.GeoPoint, error) {
	const q = `
		SELECT rlp.lat, rlp.lon
		FROM trip_points tp
		JOIN raw_location_points rlp
		  ON rlp.event_id  = tp.event_id
		 AND rlp.user_id   = tp.user_id
		 AND rlp.device_id = tp.device_id
		WHERE tp.trip_id = $1
		ORDER BY tp.recorded_at ASC, tp.event_id ASC`

	rows, err := r.db.Query(ctx, q, tripID)
	if err != nil {
		return nil, fmt.Errorf("route_repo: find trip points: %w", err)
	}
	defer rows.Close()

	var pts []domain.GeoPoint
	for rows.Next() {
		var p domain.GeoPoint
		if err := rows.Scan(&p.Lat, &p.Lon); err != nil {
			return nil, fmt.Errorf("route_repo: scan point: %w", err)
		}
		pts = append(pts, p)
	}
	return pts, rows.Err()
}

func (r *RouteRepo) FindCandidateRoutes(ctx context.Context, userID uuid.UUID, startLat, startLon, radiusM float64) ([]domain.Route, error) {
	const q = `
		SELECT id, user_id, device_id, name, status,
		       start_lat, start_lon, end_lat, end_lon,
		       distance_m, trips_count,
		       first_trip_id, last_trip_id,
		       created_at, updated_at
		FROM routes
		WHERE user_id = $1
		  AND status = 'active'
		  AND ST_DWithin(
		          start_geom::geography,
		          ST_SetSRID(ST_MakePoint($3, $2), 4326)::geography,
		          $4
		      )`

	rows, err := r.db.Query(ctx, q, userID, startLat, startLon, radiusM)
	if err != nil {
		return nil, fmt.Errorf("route_repo: find candidates: %w", err)
	}
	defer rows.Close()

	var routes []domain.Route
	for rows.Next() {
		var rt domain.Route
		if err := rows.Scan(
			&rt.ID, &rt.UserID, &rt.DeviceID, &rt.Name, &rt.Status,
			&rt.StartLat, &rt.StartLon, &rt.EndLat, &rt.EndLon,
			&rt.DistanceM, &rt.TripsCount,
			&rt.FirstTripID, &rt.LastTripID,
			&rt.CreatedAt, &rt.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("route_repo: scan candidate: %w", err)
		}
		routes = append(routes, rt)
	}
	return routes, rows.Err()
}

func (r *RouteRepo) FindRouteTemplate(ctx context.Context, routeID uuid.UUID) ([]domain.GeoPoint, error) {
	const q = `
		SELECT ST_Y((dp).geom) AS lat, ST_X((dp).geom) AS lon
		FROM routes, ST_DumpPoints(template_geom) AS dp
		WHERE id = $1
		ORDER BY (dp).path[1]`

	rows, err := r.db.Query(ctx, q, routeID)
	if err != nil {
		return nil, fmt.Errorf("route_repo: find template: %w", err)
	}
	defer rows.Close()

	var pts []domain.GeoPoint
	for rows.Next() {
		var p domain.GeoPoint
		if err := rows.Scan(&p.Lat, &p.Lon); err != nil {
			return nil, fmt.Errorf("route_repo: scan template point: %w", err)
		}
		pts = append(pts, p)
	}
	return pts, rows.Err()
}

func (r *RouteRepo) InsertRoute(ctx context.Context, route domain.Route) error {
	wkt := buildLineStringWKT(route.Template)
	var deviceID *uuid.UUID
	if route.DeviceID != nil {
		deviceID = route.DeviceID
	}
	const q = `
		INSERT INTO routes (
			id, user_id, device_id, name, status,
			start_lat, start_lon, end_lat, end_lon,
			distance_m, trips_count,
			template_geom,
			first_trip_id, last_trip_id,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11,
			ST_GeomFromText($12, 4326),
			$13, $14,
			$15, $16
		) ON CONFLICT (id) DO NOTHING`

	_, err := r.db.Exec(ctx, q,
		route.ID, route.UserID, deviceID, route.Name, route.Status,
		route.StartLat, route.StartLon, route.EndLat, route.EndLon,
		route.DistanceM, route.TripsCount,
		wkt,
		route.FirstTripID, route.LastTripID,
		route.CreatedAt, route.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("route_repo: insert route: %w", err)
	}
	return nil
}

func (r *RouteRepo) InsertRouteTrip(ctx context.Context, rt domain.RouteTrip) error {
	const q = `
		INSERT INTO route_trips (route_id, trip_id, user_id, device_id, match_score, matched_at, duration_sec, distance_m)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (trip_id) DO NOTHING`

	_, err := r.db.Exec(ctx, q,
		rt.RouteID, rt.TripID, rt.UserID, rt.DeviceID,
		rt.MatchScore, rt.MatchedAt, rt.DurationSec, rt.DistanceM,
	)
	if err != nil {
		return fmt.Errorf("route_repo: insert route_trip: %w", err)
	}
	return nil
}

func (r *RouteRepo) IncrRouteStats(ctx context.Context, routeID, lastTripID uuid.UUID) error {
	const q = `
		UPDATE routes
		SET trips_count = trips_count + 1,
		    last_trip_id = $2,
		    updated_at = now()
		WHERE id = $1`

	_, err := r.db.Exec(ctx, q, routeID, lastTripID)
	if err != nil {
		return fmt.Errorf("route_repo: incr route stats: %w", err)
	}
	return nil
}

// buildLineStringWKT converts a slice of GeoPoints into a WKT LINESTRING.
// PostGIS uses (lon lat) order inside WKT.
func buildLineStringWKT(pts []domain.GeoPoint) string {
	coords := make([]string, len(pts))
	for i, p := range pts {
		coords[i] = fmt.Sprintf("%f %f", p.Lon, p.Lat)
	}
	return "LINESTRING(" + strings.Join(coords, ",") + ")"
}
