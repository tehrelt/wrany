package trip

import (
	"time"

	"github.com/wrany/libs/events"
)

// CompletedPayload is the body of a trip.completed.v1 event.
type CompletedPayload struct {
	TripID      string    `json:"trip_id"`
	UserID      string    `json:"user_id"`
	DeviceID    string    `json:"device_id"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	DistanceM   float64   `json:"distance_m"`
	DurationS   float64   `json:"duration_s"`
	PointCount  int       `json:"point_count"`
}

// Validate checks required fields and value ranges.
func (p CompletedPayload) Validate() error {
	var issues []string
	issues = events.RequireNonEmpty(issues, "trip_id", p.TripID)
	issues = events.RequireNonEmpty(issues, "user_id", p.UserID)
	issues = events.RequireNonEmpty(issues, "device_id", p.DeviceID)
	issues = events.RequireNonZeroTime(issues, "started_at", p.StartedAt)
	issues = events.RequireNonZeroTime(issues, "completed_at", p.CompletedAt)
	if !p.StartedAt.IsZero() && !p.CompletedAt.IsZero() && p.CompletedAt.Before(p.StartedAt) {
		issues = append(issues, "completed_at must not be before started_at")
	}
	issues = events.RequireNonNegative(issues, "distance_m", p.DistanceM)
	issues = events.RequireNonNegative(issues, "duration_s", p.DurationS)
	issues = events.RequireNonNegative(issues, "point_count", float64(p.PointCount))
	return events.NewValidationError(issues)
}

// NewCompletedEvent validates the payload and wraps it in a trip.completed.v1 envelope.
func NewCompletedEvent(eventID string, producedAt time.Time, producer, correlationID string, p CompletedPayload) (events.Envelope, error) {
	if err := p.Validate(); err != nil {
		return events.Envelope{}, err
	}
	return events.NewEnvelope(events.EnvelopeParams{
		EventID:       eventID,
		EventType:     events.SubjectTripCompleted,
		EventVersion:  1,
		OccurredAt:    p.CompletedAt,
		ProducedAt:    producedAt,
		Producer:      producer,
		CorrelationID: correlationID,
	}, p)
}
