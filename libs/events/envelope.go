// Package events defines the shared event contracts for the WR any% event bus.
//
// Every message published to the bus is wrapped in an Envelope. Payload types
// live in subpackages (location, trip, route, deadletter). The package has no
// external dependencies and is safe to import from any layer.
package events

import (
	"encoding/json"
	"fmt"
	"time"
)

// Envelope is the common wrapper for all events on the bus.
type Envelope struct {
	EventID       string          `json:"event_id"`
	EventType     string          `json:"event_type"`
	EventVersion  int             `json:"event_version"`
	OccurredAt    time.Time       `json:"occurred_at"`
	ProducedAt    time.Time       `json:"produced_at"`
	Producer      string          `json:"producer"`
	CorrelationID string          `json:"correlation_id,omitempty"`
	Payload       json.RawMessage `json:"payload"`
}

// EnvelopeParams carries the metadata required to build an Envelope.
type EnvelopeParams struct {
	EventID       string
	EventType     string
	EventVersion  int
	OccurredAt    time.Time
	ProducedAt    time.Time
	Producer      string
	CorrelationID string
}

// NewEnvelope builds a validated Envelope, serializing payload to JSON.
// Returns an error if payload serialization fails or the result is invalid.
func NewEnvelope(params EnvelopeParams, payload any) (Envelope, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, fmt.Errorf("events: marshal payload for %q: %w", params.EventType, err)
	}

	env := Envelope{
		EventID:       params.EventID,
		EventType:     params.EventType,
		EventVersion:  params.EventVersion,
		OccurredAt:    params.OccurredAt,
		ProducedAt:    params.ProducedAt,
		Producer:      params.Producer,
		CorrelationID: params.CorrelationID,
		Payload:       raw,
	}
	if err := env.Validate(); err != nil {
		return Envelope{}, err
	}
	return env, nil
}

// DecodePayload unmarshals the envelope payload into dst.
func (e Envelope) DecodePayload(dst any) error {
	if err := json.Unmarshal(e.Payload, dst); err != nil {
		return fmt.Errorf("events: decode payload of %q: %w", e.EventType, err)
	}
	return nil
}
