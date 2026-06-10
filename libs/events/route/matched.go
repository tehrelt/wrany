// Package route defines the route.*.v1 payload contracts.
package route

import (
	"time"

	"github.com/wrany/libs/events"
)

// MatchedPayload is the body of a route.matched.v1 event.
type MatchedPayload struct {
	TripID     string    `json:"trip_id"`
	RouteID    string    `json:"route_id"`
	UserID     string    `json:"user_id"`
	MatchedAt  time.Time `json:"matched_at"`
	MatchScore float64   `json:"match_score"`
}

// Validate checks required fields and value ranges.
func (p MatchedPayload) Validate() error {
	var issues []string
	issues = events.RequireNonEmpty(issues, "trip_id", p.TripID)
	issues = events.RequireNonEmpty(issues, "route_id", p.RouteID)
	issues = events.RequireNonEmpty(issues, "user_id", p.UserID)
	issues = events.RequireNonZeroTime(issues, "matched_at", p.MatchedAt)
	issues = events.RequireRange(issues, "match_score", p.MatchScore, 0, 1)
	return events.NewValidationError(issues)
}

// NewMatchedEvent validates the payload and wraps it in a route.matched.v1 envelope.
func NewMatchedEvent(eventID string, producedAt time.Time, producer, correlationID string, p MatchedPayload) (events.Envelope, error) {
	if err := p.Validate(); err != nil {
		return events.Envelope{}, err
	}
	return events.NewEnvelope(events.EnvelopeParams{
		EventID:       eventID,
		EventType:     events.SubjectRouteMatched,
		EventVersion:  1,
		OccurredAt:    p.MatchedAt,
		ProducedAt:    producedAt,
		Producer:      producer,
		CorrelationID: correlationID,
	}, p)
}
