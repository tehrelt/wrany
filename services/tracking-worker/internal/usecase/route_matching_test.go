package usecase

import (
	"math"
	"testing"

	"github.com/google/uuid"
	"github.com/wrany/tracking-worker/internal/domain"
)

func defaultCfg() domain.RouteMatchConfig {
	return domain.RouteMatchConfig{
		StartRadiusM:             75,
		EndRadiusM:               75,
		DistanceToleranceRatio:   0.25,
		PathSimilarityThresholdM: 50,
		MinTripPoints:            5,
		NormalizePointsN:         10,
	}
}

func makeLine(lat, lon float64, n int, dLat, dLon float64) []domain.GeoPoint {
	pts := make([]domain.GeoPoint, n)
	for i := range pts {
		pts[i] = domain.GeoPoint{Lat: lat + float64(i)*dLat, Lon: lon + float64(i)*dLon}
	}
	return pts
}

func endOf(pts []domain.GeoPoint) (float64, float64) {
	last := pts[len(pts)-1]
	return last.Lat, last.Lon
}

func makeRoute(id uuid.UUID, pts []domain.GeoPoint, distM float64) domain.Route {
	endLat, endLon := endOf(pts)
	return domain.Route{
		ID:        id,
		UserID:    uuid.New(),
		Status:    "active",
		StartLat:  pts[0].Lat,
		StartLon:  pts[0].Lon,
		EndLat:    endLat,
		EndLon:    endLon,
		DistanceM: distM,
	}
}

func makeTrip(pts []domain.GeoPoint, distM float64) domain.Trip {
	endLat, endLon := endOf(pts)
	return domain.Trip{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		DeviceID:  uuid.New(),
		StartLat:  pts[0].Lat,
		StartLon:  pts[0].Lon,
		EndLat:    &endLat,
		EndLon:    &endLon,
		DistanceM: distM,
	}
}

// TestMatchSameRoute: identical start/end/distance/path → match.
func TestMatchSameRoute(t *testing.T) {
	uc := NewRouteMatchingUseCase(defaultCfg())

	pts := makeLine(55.75, 37.62, 10, 0.001, 0.001)
	routeID := uuid.New()
	route := makeRoute(routeID, pts, 1000)
	trip := makeTrip(pts, 1000)
	trip.UserID = route.UserID

	result := uc.FindBestMatch(trip, pts, []domain.Route{route}, map[string][]domain.GeoPoint{
		routeID.String(): pts,
	})

	if result.RouteID != routeID.String() {
		t.Fatalf("expected match to routeID %s, got %q", routeID, result.RouteID)
	}
	if result.Score <= 0 || result.Score > 1 {
		t.Fatalf("score out of range: %f", result.Score)
	}
}

// TestNoMatchDifferentStart: start points far apart → no match.
func TestNoMatchDifferentStart(t *testing.T) {
	uc := NewRouteMatchingUseCase(defaultCfg())

	routePts := makeLine(55.75, 37.62, 10, 0.001, 0.001)
	routeID := uuid.New()
	route := makeRoute(routeID, routePts, 1000)

	// Trip starts ~500m away.
	tripPts := makeLine(55.755, 37.62, 10, 0.001, 0.001)
	endLat, endLon := endOf(tripPts)
	trip := domain.Trip{
		ID: uuid.New(), UserID: route.UserID, DeviceID: uuid.New(),
		StartLat: tripPts[0].Lat, StartLon: tripPts[0].Lon,
		EndLat: &endLat, EndLon: &endLon,
		DistanceM: 1000,
	}

	result := uc.FindBestMatch(trip, tripPts, []domain.Route{route}, map[string][]domain.GeoPoint{
		routeID.String(): routePts,
	})
	if result.RouteID != "" {
		t.Fatalf("expected no match, got %s", result.RouteID)
	}
}

// TestNoMatchDifferentEnd: end points far apart → no match.
func TestNoMatchDifferentEnd(t *testing.T) {
	uc := NewRouteMatchingUseCase(defaultCfg())

	routePts := makeLine(55.75, 37.62, 10, 0.001, 0.001)
	routeID := uuid.New()
	route := makeRoute(routeID, routePts, 1000)

	// Trip ends ~500m away.
	tripPts := makeLine(55.75, 37.62, 10, 0.001, 0.0015)
	endLat, endLon := endOf(tripPts)
	trip := domain.Trip{
		ID: uuid.New(), UserID: route.UserID, DeviceID: uuid.New(),
		StartLat: tripPts[0].Lat, StartLon: tripPts[0].Lon,
		EndLat: &endLat, EndLon: &endLon,
		DistanceM: 1000,
	}

	result := uc.FindBestMatch(trip, tripPts, []domain.Route{route}, map[string][]domain.GeoPoint{
		routeID.String(): routePts,
	})
	if result.RouteID != "" {
		t.Fatalf("expected no match, got %s", result.RouteID)
	}
}

// TestNoMatchDistanceTooLarge: distance ratio > threshold → no match.
func TestNoMatchDistanceTooLarge(t *testing.T) {
	uc := NewRouteMatchingUseCase(defaultCfg())

	pts := makeLine(55.75, 37.62, 10, 0.001, 0.001)
	routeID := uuid.New()
	route := makeRoute(routeID, pts, 1000)

	endLat, endLon := endOf(pts)
	trip := domain.Trip{
		ID: uuid.New(), UserID: route.UserID, DeviceID: uuid.New(),
		StartLat: pts[0].Lat, StartLon: pts[0].Lon,
		EndLat: &endLat, EndLon: &endLon,
		DistanceM: 2000, // 100% difference → exceeds 25%
	}

	result := uc.FindBestMatch(trip, pts, []domain.Route{route}, map[string][]domain.GeoPoint{
		routeID.String(): pts,
	})
	if result.RouteID != "" {
		t.Fatalf("expected no match, got %s", result.RouteID)
	}
}

// TestNoMatchPathTooLarge: avg path distance > threshold → no match.
func TestNoMatchPathTooLarge(t *testing.T) {
	cfg := defaultCfg()
	cfg.PathSimilarityThresholdM = 10 // tight threshold
	uc := NewRouteMatchingUseCase(cfg)

	routePts := makeLine(55.75, 37.62, 10, 0.001, 0.001)
	routeID := uuid.New()
	route := makeRoute(routeID, routePts, 1000)

	// Trip follows a different path (offset by ~200m).
	tripPts := makeLine(55.75, 37.62, 10, 0.002, 0.001)
	endLat, endLon := endOf(tripPts)
	// Force end to be close to route end to pass end-point filter.
	rEnd := routePts[len(routePts)-1]
	trip := domain.Trip{
		ID: uuid.New(), UserID: route.UserID, DeviceID: uuid.New(),
		StartLat: tripPts[0].Lat, StartLon: tripPts[0].Lon,
		EndLat: &rEnd.Lat, EndLon: &rEnd.Lon,
		DistanceM: 1000,
	}
	_ = endLat
	_ = endLon

	result := uc.FindBestMatch(trip, tripPts, []domain.Route{route}, map[string][]domain.GeoPoint{
		routeID.String(): routePts,
	})
	if result.RouteID != "" {
		t.Fatalf("expected no match due to path distance, got %s", result.RouteID)
	}
}

// TestNoMatchReverseDirection: B→A does not match A→B route.
func TestNoMatchReverseDirection(t *testing.T) {
	uc := NewRouteMatchingUseCase(defaultCfg())

	routePts := makeLine(55.75, 37.62, 10, 0.001, 0.001)
	routeID := uuid.New()
	route := makeRoute(routeID, routePts, 1000)

	// Reverse: starts at route's end, ends at route's start.
	reversePts := make([]domain.GeoPoint, len(routePts))
	for i, p := range routePts {
		reversePts[len(routePts)-1-i] = p
	}
	endLat, endLon := endOf(reversePts)
	trip := domain.Trip{
		ID: uuid.New(), UserID: route.UserID, DeviceID: uuid.New(),
		StartLat: reversePts[0].Lat, StartLon: reversePts[0].Lon,
		EndLat: &endLat, EndLon: &endLon,
		DistanceM: 1000,
	}

	// The reverse trip's start is far from route's start → no candidate returned
	// (simulated by passing empty candidates or a candidate whose start is far).
	result := uc.FindBestMatch(trip, reversePts, []domain.Route{route}, map[string][]domain.GeoPoint{
		routeID.String(): routePts,
	})
	// The start of the reverse trip is the end of the route → far from route.start.
	// If the candidate happens to pass start check (close start), the end check will fail.
	if result.RouteID != "" {
		t.Logf("match score: %f", result.Score)
		t.Fatalf("reverse direction should not match the forward route")
	}
}

// TestScoreDeterministic: same input always produces same score.
func TestScoreDeterministic(t *testing.T) {
	uc := NewRouteMatchingUseCase(defaultCfg())

	pts := makeLine(55.75, 37.62, 10, 0.001, 0.001)
	routeID := uuid.New()
	route := makeRoute(routeID, pts, 1000)
	trip := makeTrip(pts, 1000)
	trip.UserID = route.UserID

	args := []domain.Route{route}
	tpls := map[string][]domain.GeoPoint{routeID.String(): pts}

	r1 := uc.FindBestMatch(trip, pts, args, tpls)
	r2 := uc.FindBestMatch(trip, pts, args, tpls)

	if math.Abs(r1.Score-r2.Score) > 1e-9 {
		t.Fatalf("non-deterministic score: %f vs %f", r1.Score, r2.Score)
	}
}

// TestHaversine: basic sanity check (Moscow area, ~100m apart).
func TestHaversine(t *testing.T) {
	d := haversine(55.7512, 37.6184, 55.7512, 37.6184)
	if d != 0 {
		t.Fatalf("same point: expected 0, got %f", d)
	}
	// ~1 degree lat ≈ 111km.
	d = haversine(55.0, 37.0, 56.0, 37.0)
	if d < 100_000 || d > 120_000 {
		t.Fatalf("1-degree lat distance out of expected range: %f", d)
	}
}

// TestNormalizePolyline: length of result always equals n.
func TestNormalizePolyline(t *testing.T) {
	pts := makeLine(55.0, 37.0, 20, 0.001, 0.001)
	for _, n := range []int{1, 5, 10, 50} {
		got := normalizePolyline(pts, n)
		if len(got) != n {
			t.Fatalf("normalize(%d): got %d points", n, len(got))
		}
	}
}

// TestTooFewPoints: trip with fewer than MinTripPoints → no match regardless.
func TestTooFewPoints(t *testing.T) {
	uc := NewRouteMatchingUseCase(defaultCfg())

	routePts := makeLine(55.75, 37.62, 10, 0.001, 0.001)
	routeID := uuid.New()
	route := makeRoute(routeID, routePts, 1000)

	shortPts := routePts[:3] // only 3 points, min is 5
	trip := makeTrip(routePts, 1000)
	trip.UserID = route.UserID

	result := uc.FindBestMatch(trip, shortPts, []domain.Route{route}, map[string][]domain.GeoPoint{
		routeID.String(): routePts,
	})
	if result.RouteID != "" {
		t.Fatalf("trip with too few points should not match, got %s", result.RouteID)
	}
}
