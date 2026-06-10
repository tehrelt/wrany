package events_test

import (
	"strings"
	"testing"
	"time"

	"github.com/wrany/libs/events"
)

func validEnvelope(t *testing.T) events.Envelope {
	t.Helper()
	env, err := events.NewEnvelope(validParams(), testPayload{Value: "x"})
	if err != nil {
		t.Fatalf("building valid envelope: %v", err)
	}
	return env
}

func TestEnvelopeValidate_RequiredFields(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(e events.Envelope) events.Envelope
		wantIssue string
	}{
		{"missing event_id", func(e events.Envelope) events.Envelope { e.EventID = ""; return e }, "event_id"},
		{"missing event_type", func(e events.Envelope) events.Envelope { e.EventType = ""; return e }, "event_type"},
		{"zero event_version", func(e events.Envelope) events.Envelope { e.EventVersion = 0; return e }, "event_version"},
		{"negative event_version", func(e events.Envelope) events.Envelope { e.EventVersion = -1; return e }, "event_version"},
		{"missing occurred_at", func(e events.Envelope) events.Envelope { e.OccurredAt = time.Time{}; return e }, "occurred_at"},
		{"missing produced_at", func(e events.Envelope) events.Envelope { e.ProducedAt = time.Time{}; return e }, "produced_at"},
		{"missing producer", func(e events.Envelope) events.Envelope { e.Producer = ""; return e }, "producer"},
		{"missing payload", func(e events.Envelope) events.Envelope { e.Payload = nil; return e }, "payload"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.mutate(validEnvelope(t)).Validate()
			if !events.IsValidationError(err) {
				t.Fatalf("expected validation error, got %v", err)
			}
			if !strings.Contains(err.Error(), tt.wantIssue) {
				t.Errorf("error %q does not mention %q", err, tt.wantIssue)
			}
		})
	}
}

func TestEnvelopeValidate_OptionalCorrelationID(t *testing.T) {
	env := validEnvelope(t)
	env.CorrelationID = ""
	if err := env.Validate(); err != nil {
		t.Errorf("correlation_id must be optional at envelope level, got %v", err)
	}
}

func TestEnvelopeValidate_AggregatesIssues(t *testing.T) {
	err := events.Envelope{}.Validate()
	if !events.IsValidationError(err) {
		t.Fatalf("expected validation error, got %v", err)
	}
	var ve *events.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if len(ve.Issues) < 6 {
		t.Errorf("expected all issues aggregated, got %d: %v", len(ve.Issues), ve.Issues)
	}
}

func asValidationError(err error, target **events.ValidationError) bool {
	ve, ok := err.(*events.ValidationError)
	if ok {
		*target = ve
	}
	return ok
}

func TestStreamSubjects_ReturnsFreshSlice(t *testing.T) {
	first := events.StreamSubjects()
	first[0] = "mutated"
	second := events.StreamSubjects()
	if second[0] != "location.events.*" {
		t.Errorf("StreamSubjects must return a fresh slice, got %v", second)
	}
}

func TestConsumerName(t *testing.T) {
	got := events.ConsumerName("tracking-worker", "location")
	want := "tracking-worker-location-consumer"
	if got != want {
		t.Errorf("ConsumerName = %q, want %q", got, want)
	}
}
