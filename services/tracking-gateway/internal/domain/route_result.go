package domain

import "time"

// TripResult holds key metrics for a single trip.
type TripResult struct {
	TripID      string
	StartedAt   time.Time
	DurationSec int64
	DistanceM   float64
	AvgSpeedMps float64
}

// RouteResultComparison compares the latest trip against the best.
type RouteResultComparison struct {
	LatestVsBestSec     int64
	LatestVsBestPercent float64
}

// RouteResult is the personal records summary for a route.
type RouteResult struct {
	RouteID       string
	AttemptsCount int
	Best          *TripResult
	Latest        *TripResult
	Comparison    *RouteResultComparison
}

// TripAttempt is a single entry in the paginated attempts list.
type TripAttempt struct {
	TripID      string
	StartedAt   time.Time
	EndedAt     *time.Time
	DurationSec int64
	DistanceM   float64
	AvgSpeedMps float64
	MatchScore  float64
	MatchedAt   time.Time
	IsBest      bool
}

// TripAttemptFilter controls pagination for ListRouteAttempts.
type TripAttemptFilter struct {
	RouteID string
	UserID  string
	Limit   int
	Cursor  string
}
