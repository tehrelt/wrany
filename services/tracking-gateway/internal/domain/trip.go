package domain

import "time"

// TripStatus mirrors the CHECK constraint in tracking-worker.
type TripStatus string

const (
	TripStatusActive    TripStatus = "TRIP_ACTIVE"
	TripStatusCompleted TripStatus = "TRIP_COMPLETED"
)

// Trip is a detected trip row as exposed by the query API.
type Trip struct {
	ID       string
	UserID   string
	DeviceID string

	Status    TripStatus
	StartedAt time.Time
	EndedAt   *time.Time

	StartLat float64
	StartLon float64
	EndLat   *float64
	EndLon   *float64

	DistanceM   float64
	DurationSec int64
	PointsCount int

	CreatedAt time.Time
	UpdatedAt time.Time
}

// TripPoint is a single GPS point linked to a trip (lat/lon joined from raw_location_points).
type TripPoint struct {
	EventID    string
	TripID     string
	RecordedAt time.Time
	Lat        float64
	Lon        float64
}

// TripFilter holds query parameters for ListTrips.
// UserID must always be set from the JWT.
type TripFilter struct {
	UserID   string
	DeviceID string     // optional
	Status   TripStatus // optional; empty means all statuses
	Limit    int
	Cursor   string // opaque base64; empty means first page
}

// TripPointFilter holds query parameters for GetTripPoints.
type TripPointFilter struct {
	TripID string
	UserID string
	Limit  int
	Cursor string
}
