package domain

import "time"

// ActivityType enumerates recognised activity types from the tracker.
type ActivityType string

const (
	ActivityWalking    ActivityType = "walking"
	ActivityRunning    ActivityType = "running"
	ActivityBicycle    ActivityType = "bicycle"
	ActivityVehicle    ActivityType = "vehicle"
	ActivityStationary ActivityType = "stationary"
	ActivityUnknown    ActivityType = "unknown"
)

// ValidActivityTypes is the closed set of accepted values.
var ValidActivityTypes = map[ActivityType]struct{}{
	ActivityWalking:    {},
	ActivityRunning:    {},
	ActivityBicycle:    {},
	ActivityVehicle:    {},
	ActivityStationary: {},
	ActivityUnknown:    {},
}

// LocationEvent is the parsed, domain-level representation of a single
// location data point received from the tracker client.
type LocationEvent struct {
	EventID            string
	RecordedAt         time.Time
	Lat                float64
	Lon                float64
	AccuracyM          float64
	SpeedMps           *float64
	BearingDeg         *float64
	ActivityType       *ActivityType
	ActivityConfidence *float64
	BatteryLevel       *float64
}

// RejectedEvent carries an event_id and the reason it was not accepted.
type RejectedEvent struct {
	EventID string
	Reason  string
}
