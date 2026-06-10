// Package trip defines the trip.*.v1 payload contracts.
package trip

import (
	"time"

	"github.com/wrany/libs/events"
)

// StartedPayload is the body of a trip.started.v1 event.
type StartedPayload struct {
	TripID    string    `json:"trip_id"`
	UserID    string    `json:"user_id"`
	DeviceID  string    `json:"device_id"`
	StartedAt time.Time `json:"started_at"`
}

// Validate checks required fields.
func (p StartedPayload) Validate() error {
	var issues []string
	issues = events.RequireNonEmpty(issues, "trip_id", p.TripID)
	issues = events.RequireNonEmpty(issues, "user_id", p.UserID)
	issues = events.RequireNonEmpty(issues, "device_id", p.DeviceID)
	issues = events.RequireNonZeroTime(issues, "started_at", p.StartedAt)
	return events.NewValidationError(issues)
}

// NewStartedEvent validates the payload and wraps it in a trip.started.v1 envelope.
func NewStartedEvent(eventID string, producedAt time.Time, producer, correlationID string, p StartedPayload) (events.Envelope, error) {
	if err := p.Validate(); err != nil {
		return events.Envelope{}, err
	}
	return events.NewEnvelope(events.EnvelopeParams{
		EventID:       eventID,
		EventType:     events.SubjectTripStarted,
		EventVersion:  1,
		OccurredAt:    p.StartedAt,
		ProducedAt:    producedAt,
		Producer:      producer,
		CorrelationID: correlationID,
	}, p)
}
