package events_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/wrany/libs/events"
)

func validParams() events.EnvelopeParams {
	return events.EnvelopeParams{
		EventID:       "evt_123",
		EventType:     events.SubjectLocationEvents,
		EventVersion:  1,
		OccurredAt:    time.Date(2026, 6, 10, 12, 0, 1, 0, time.UTC),
		ProducedAt:    time.Date(2026, 6, 10, 12, 0, 4, 0, time.UTC),
		Producer:      "tracking-gateway",
		CorrelationID: "req_123",
	}
}

type testPayload struct {
	Value string `json:"value"`
}

func TestNewEnvelope_Valid(t *testing.T) {
	env, err := events.NewEnvelope(validParams(), testPayload{Value: "x"})
	if err != nil {
		t.Fatalf("NewEnvelope returned error: %v", err)
	}
	if env.EventID != "evt_123" || env.EventType != events.SubjectLocationEvents {
		t.Errorf("unexpected envelope metadata: %+v", env)
	}

	var p testPayload
	if err := env.DecodePayload(&p); err != nil {
		t.Fatalf("DecodePayload: %v", err)
	}
	if p.Value != "x" {
		t.Errorf("payload round-trip failed: got %q", p.Value)
	}
}

func TestNewEnvelope_InvalidParams(t *testing.T) {
	params := validParams()
	params.EventID = ""
	if _, err := events.NewEnvelope(params, testPayload{}); !events.IsValidationError(err) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestNewEnvelope_UnmarshalablePayload(t *testing.T) {
	if _, err := events.NewEnvelope(validParams(), make(chan int)); err == nil {
		t.Fatal("expected marshal error for chan payload")
	}
}

func TestEnvelope_JSONRoundTrip(t *testing.T) {
	env, err := events.NewEnvelope(validParams(), testPayload{Value: "round-trip"})
	if err != nil {
		t.Fatalf("NewEnvelope: %v", err)
	}

	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}

	var decoded events.Envelope
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if decoded.EventID != env.EventID ||
		decoded.EventType != env.EventType ||
		decoded.EventVersion != env.EventVersion ||
		!decoded.OccurredAt.Equal(env.OccurredAt) ||
		!decoded.ProducedAt.Equal(env.ProducedAt) ||
		decoded.Producer != env.Producer ||
		decoded.CorrelationID != env.CorrelationID {
		t.Errorf("round-trip mismatch:\n got %+v\nwant %+v", decoded, env)
	}

	var p testPayload
	if err := decoded.DecodePayload(&p); err != nil {
		t.Fatalf("DecodePayload after round-trip: %v", err)
	}
	if p.Value != "round-trip" {
		t.Errorf("payload after round-trip: got %q", p.Value)
	}
}
