package usecase

import (
	"math"

	"github.com/wrany/tracking-worker/internal/domain"
)

// RouteMatchingUseCase contains the pure matching algorithm, no I/O.
type RouteMatchingUseCase struct {
	cfg domain.RouteMatchConfig
}

func NewRouteMatchingUseCase(cfg domain.RouteMatchConfig) *RouteMatchingUseCase {
	return &RouteMatchingUseCase{cfg: cfg}
}

// MatchResult holds the outcome of attempting to match a trip to a route.
type MatchResult struct {
	RouteID   string // empty → no match found, create new route
	Score     float64
}

// FindBestMatch returns the best matching route for the given trip.
// Candidates are pre-filtered by start-point proximity (done in storage layer).
// Further filtering by end-point distance, distance tolerance, and path
// similarity is done here in pure Go.
func (uc *RouteMatchingUseCase) FindBestMatch(
	trip domain.Trip,
	tripPoints []domain.GeoPoint,
	candidates []domain.Route,
	templateByRouteID map[string][]domain.GeoPoint,
) MatchResult {
	if len(tripPoints) < uc.cfg.MinTripPoints {
		return MatchResult{}
	}

	tripNorm := normalizePolyline(tripPoints, uc.cfg.NormalizePointsN)

	endLat := 0.0
	endLon := 0.0
	if trip.EndLat != nil {
		endLat = *trip.EndLat
	}
	if trip.EndLon != nil {
		endLon = *trip.EndLon
	}

	bestScore := -1.0
	bestID := ""

	for _, r := range candidates {
		// Filter by end-point distance.
		if haversine(endLat, endLon, r.EndLat, r.EndLon) > uc.cfg.EndRadiusM {
			continue
		}
		// Filter by distance tolerance.
		if r.DistanceM > 0 {
			ratio := math.Abs(trip.DistanceM-r.DistanceM) / r.DistanceM
			if ratio > uc.cfg.DistanceToleranceRatio {
				continue
			}
		}
		// Path similarity.
		template, ok := templateByRouteID[r.ID.String()]
		if !ok || len(template) == 0 {
			continue
		}
		routeNorm := normalizePolyline(template, uc.cfg.NormalizePointsN)
		avgDist := avgPointDistance(tripNorm, routeNorm)
		if avgDist > uc.cfg.PathSimilarityThresholdM {
			continue
		}
		score := math.Max(0, 1.0-avgDist/uc.cfg.PathSimilarityThresholdM)
		if score > bestScore {
			bestScore = score
			bestID = r.ID.String()
		}
	}

	return MatchResult{RouteID: bestID, Score: bestScore}
}

// normalizePolyline resamples pts to exactly n evenly-spaced points.
func normalizePolyline(pts []domain.GeoPoint, n int) []domain.GeoPoint {
	if len(pts) == 0 || n <= 0 {
		return nil
	}
	if len(pts) == 1 || n == 1 {
		result := make([]domain.GeoPoint, n)
		for i := range result {
			result[i] = pts[0]
		}
		return result
	}

	cum := make([]float64, len(pts))
	for i := 1; i < len(pts); i++ {
		cum[i] = cum[i-1] + haversine(pts[i-1].Lat, pts[i-1].Lon, pts[i].Lat, pts[i].Lon)
	}
	total := cum[len(cum)-1]
	if total == 0 {
		result := make([]domain.GeoPoint, n)
		for i := range result {
			result[i] = pts[0]
		}
		return result
	}

	result := make([]domain.GeoPoint, n)
	j := 0
	for i := range n {
		target := total * float64(i) / float64(n-1)
		for j+1 < len(pts)-1 && cum[j+1] < target {
			j++
		}
		if j+1 >= len(pts) {
			result[i] = pts[len(pts)-1]
			continue
		}
		segLen := cum[j+1] - cum[j]
		if segLen == 0 {
			result[i] = pts[j]
			continue
		}
		t := (target - cum[j]) / segLen
		result[i] = domain.GeoPoint{
			Lat: pts[j].Lat + t*(pts[j+1].Lat-pts[j].Lat),
			Lon: pts[j].Lon + t*(pts[j+1].Lon-pts[j].Lon),
		}
	}
	return result
}

// avgPointDistance returns the average Haversine distance (metres) between
// two equal-length polylines sampled at the same positions.
func avgPointDistance(a, b []domain.GeoPoint) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return math.MaxFloat64
	}
	sum := 0.0
	for i := range a {
		sum += haversine(a[i].Lat, a[i].Lon, b[i].Lat, b[i].Lon)
	}
	return sum / float64(len(a))
}

// haversine returns the great-circle distance in metres between two WGS-84 points.
func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6_371_000.0
	φ1 := lat1 * math.Pi / 180
	φ2 := lat2 * math.Pi / 180
	Δφ := (lat2 - lat1) * math.Pi / 180
	Δλ := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(Δφ/2)*math.Sin(Δφ/2) +
		math.Cos(φ1)*math.Cos(φ2)*math.Sin(Δλ/2)*math.Sin(Δλ/2)
	return R * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
