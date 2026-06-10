// Package deadletter defines the dead-letter.v1 payload contract.
//
// Consumers publish a dead-letter event when a message exhausts redelivery
// attempts (MaxDeliver) and cannot be processed.
package deadletter

import (
	"encoding/json"
	"time"

	"github.com/wrany/libs/events"
)

// Payload is the body of a dead-letter.v1 event.
type Payload struct {
	OriginalSubject string          `json:"original_subject"`
	OriginalEvent   json.RawMessage `json:"original_event"`
	Error           string          `json:"error"`
	FailedAt        time.Time       `json:"failed_at"`
	Consumer        string          `json:"consumer"`
}

// Validate checks required fields.
func (p Payload) Validate() error {
	var issues []string
	issues = events.RequireNonEmpty(issues, "original_subject", p.OriginalSubject)
	if len(p.OriginalEvent) == 0 {
		issues = append(issues, "original_event is required")
	}
	issues = events.RequireNonEmpty(issues, "error", p.Error)
	issues = events.RequireNonZeroTime(issues, "failed_at", p.FailedAt)
	issues = events.RequireNonEmpty(issues, "consumer", p.Consumer)
	return events.NewValidationError(issues)
}

// NewEvent validates the payload and wraps it in a dead-letter.v1 envelope.
func NewEvent(eventID string, producedAt time.Time, producer, correlationID string, p Payload) (events.Envelope, error) {
	if err := p.Validate(); err != nil {
		return events.Envelope{}, err
	}
	return events.NewEnvelope(events.EnvelopeParams{
		EventID:       eventID,
		EventType:     events.SubjectDeadLetter,
		EventVersion:  1,
		OccurredAt:    p.FailedAt,
		ProducedAt:    producedAt,
		Producer:      producer,
		CorrelationID: correlationID,
	}, p)
}
