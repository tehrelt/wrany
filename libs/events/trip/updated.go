package trip

import (
	"time"

	"github.com/wrany/libs/events"
)

// UpdatedPayload is the body of a trip.updated.v1 event.
type UpdatedPayload struct {
	TripID     string    `json:"trip_id"`
	UserID     string    `json:"user_id"`
	DeviceID   string    `json:"device_id"`
	UpdatedAt  time.Time `json:"updated_at"`
	DistanceM  float64   `json:"distance_m"`
	DurationS  float64   `json:"duration_s"`
	PointCount int       `json:"point_count"`
}

// Validate checks required fields and value ranges.
func (p UpdatedPayload) Validate() error {
	var issues []string
	issues = events.RequireNonEmpty(issues, "trip_id", p.TripID)
	issues = events.RequireNonEmpty(issues, "user_id", p.UserID)
	issues = events.RequireNonEmpty(issues, "device_id", p.DeviceID)
	issues = events.RequireNonZeroTime(issues, "updated_at", p.UpdatedAt)
	issues = events.RequireNonNegative(issues, "distance_m", p.DistanceM)
	issues = events.RequireNonNegative(issues, "duration_s", p.DurationS)
	issues = events.RequireNonNegative(issues, "point_count", float64(p.PointCount))
	return events.NewValidationError(issues)
}

// NewUpdatedEvent validates the payload and wraps it in a trip.updated.v1 envelope.
func NewUpdatedEvent(eventID string, producedAt time.Time, producer, correlationID string, p UpdatedPayload) (events.Envelope, error) {
	if err := p.Validate(); err != nil {
		return events.Envelope{}, err
	}
	return events.NewEnvelope(events.EnvelopeParams{
		EventID:       eventID,
		EventType:     events.SubjectTripUpdated,
		EventVersion:  1,
		OccurredAt:    p.UpdatedAt,
		ProducedAt:    producedAt,
		Producer:      producer,
		CorrelationID: correlationID,
	}, p)
}
