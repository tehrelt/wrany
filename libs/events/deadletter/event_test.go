package deadletter_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/wrany/libs/events"
	"github.com/wrany/libs/events/deadletter"
)

func validPayload() deadletter.Payload {
	return deadletter.Payload{
		OriginalSubject: events.SubjectLocationEvents,
		OriginalEvent:   json.RawMessage(`{"event_id":"evt_orig"}`),
		Error:           "handler failed after max deliveries",
		FailedAt:        time.Date(2026, 6, 10, 12, 0, 10, 0, time.UTC),
		Consumer:        events.ConsumerName("tracking-worker", "location"),
	}
}

func TestPayloadValidate(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(p deadletter.Payload) deadletter.Payload
		wantIssue string
	}{
		{"valid", func(p deadletter.Payload) deadletter.Payload { return p }, ""},
		{"missing original_subject", func(p deadletter.Payload) deadletter.Payload { p.OriginalSubject = ""; return p }, "original_subject"},
		{"missing original_event", func(p deadletter.Payload) deadletter.Payload { p.OriginalEvent = nil; return p }, "original_event"},
		{"missing error", func(p deadletter.Payload) deadletter.Payload { p.Error = ""; return p }, "error"},
		{"missing failed_at", func(p deadletter.Payload) deadletter.Payload { p.FailedAt = time.Time{}; return p }, "failed_at"},
		{"missing consumer", func(p deadletter.Payload) deadletter.Payload { p.Consumer = ""; return p }, "consumer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.mutate(validPayload()).Validate()
			if tt.wantIssue == "" {
				if err != nil {
					t.Fatalf("expected valid payload, got %v", err)
				}
				return
			}
			if !events.IsValidationError(err) || !strings.Contains(err.Error(), tt.wantIssue) {
				t.Fatalf("expected %q validation error, got %v", tt.wantIssue, err)
			}
		})
	}
}

func TestNewEvent(t *testing.T) {
	p := validPayload()
	env, err := deadletter.NewEvent("evt_dl_1", p.FailedAt.Add(time.Second), "tracking-worker", "req_1", p)
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	if env.EventType != events.SubjectDeadLetter || !env.OccurredAt.Equal(p.FailedAt) {
		t.Errorf("unexpected envelope: %+v", env)
	}
}
