// Package location defines the location.events.v1 payload contract.
package location

import (
	"time"

	"github.com/wrany/libs/events"
)

// Source values for location events.
const SourceAndroidTracker = "android_tracker"

// Payload is the body of a location.events.v1 event.
type Payload struct {
	UserID             string    `json:"user_id"`
	DeviceID           string    `json:"device_id"`
	RecordedAt         time.Time `json:"recorded_at"`
	ReceivedAt         time.Time `json:"received_at"`
	Lat                float64   `json:"lat"`
	Lon                float64   `json:"lon"`
	AccuracyM          float64   `json:"accuracy_m"`
	SpeedMps           float64   `json:"speed_mps"`
	BearingDeg         float64   `json:"bearing_deg"`
	ActivityType       string    `json:"activity_type,omitempty"`
	ActivityConfidence float64   `json:"activity_confidence,omitempty"`
	BatteryLevel       float64   `json:"battery_level,omitempty"`
	Source             string    `json:"source"`
}

// Validate checks required fields and value ranges.
func (p Payload) Validate() error {
	var issues []string
	issues = events.RequireNonEmpty(issues, "user_id", p.UserID)
	issues = events.RequireNonEmpty(issues, "device_id", p.DeviceID)
	issues = events.RequireNonZeroTime(issues, "recorded_at", p.RecordedAt)
	issues = events.RequireNonZeroTime(issues, "received_at", p.ReceivedAt)
	issues = events.RequireRange(issues, "lat", p.Lat, -90, 90)
	issues = events.RequireRange(issues, "lon", p.Lon, -180, 180)
	issues = events.RequireNonNegative(issues, "accuracy_m", p.AccuracyM)
	issues = events.RequireRange(issues, "activity_confidence", p.ActivityConfidence, 0, 1)
	issues = events.RequireRange(issues, "battery_level", p.BatteryLevel, 0, 1)
	issues = events.RequireNonEmpty(issues, "source", p.Source)
	return events.NewValidationError(issues)
}

// OrderingKey returns the logical ordering key "user_id:device_id".
// This is metadata for headers and future sharding — NOT an ordering
// guarantee: NATS preserves order per publisher connection only, and
// consumers must sort by recorded_at.
func (p Payload) OrderingKey() string {
	return p.UserID + ":" + p.DeviceID
}

// NewEvent validates the payload and wraps it in a location.events.v1 envelope.
// correlation_id is required for location events.
func NewEvent(eventID string, producedAt time.Time, producer, correlationID string, p Payload) (events.Envelope, error) {
	if err := p.Validate(); err != nil {
		return events.Envelope{}, err
	}
	if correlationID == "" {
		return events.Envelope{}, events.NewValidationError([]string{"correlation_id is required for location events"})
	}
	return events.NewEnvelope(events.EnvelopeParams{
		EventID:       eventID,
		EventType:     events.SubjectLocationEvents,
		EventVersion:  1,
		OccurredAt:    p.RecordedAt,
		ProducedAt:    producedAt,
		Producer:      producer,
		CorrelationID: correlationID,
	}, p)
}
