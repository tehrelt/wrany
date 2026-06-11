package domain

import "time"

// TrackingPoint is a single raw GPS point read from raw_location_points.
type TrackingPoint struct {
	EventID      string
	DeviceID     string
	RecordedAt   time.Time
	Lat          float64
	Lon          float64
	AccuracyM    float64
	SpeedMps     *float64
	BearingDeg   *float64
	ActivityType string
}

// TrackingPointFilter holds query parameters for reading points.
// UserID must always be set from the JWT — never from a query param.
type TrackingPointFilter struct {
	UserID   string
	DeviceID string // optional; empty means all devices
	From     time.Time
	To       time.Time
	Limit    int    // default 1000, max 5000; enforced in usecase
	Cursor   string // opaque base64 cursor; empty means first page
}

// TrackingSummary holds aggregated stats for a time range.
type TrackingSummary struct {
	PointsCount     int
	FirstRecordedAt *time.Time
	LastRecordedAt  *time.Time
	DurationSec     int64
	AvgSpeedMps     *float64
	MaxSpeedMps     *float64
}
