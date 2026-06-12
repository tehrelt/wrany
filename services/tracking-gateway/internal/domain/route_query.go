package domain

import "time"

type Route struct {
	ID     string
	UserID string

	Name   *string
	Status string

	StartLat float64
	StartLon float64
	EndLat   float64
	EndLon   float64

	DistanceM  float64
	TripsCount int

	FirstTripID string
	LastTripID  string

	CreatedAt time.Time
	UpdatedAt time.Time
}

type RoutePoint struct {
	Lat float64
	Lon float64
}

type RouteTrip struct {
	RouteID     string
	TripID      string
	MatchScore  float64
	MatchedAt   time.Time
	DurationSec int64
	DistanceM   float64

	StartedAt time.Time
	EndedAt   *time.Time
}

type RouteFilter struct {
	UserID   string
	DeviceID string
	Limit    int
	Cursor   string
}

type RouteTripFilter struct {
	RouteID string
	UserID  string
	Limit   int
	Cursor  string
}
