package location_test

import (
	"strings"
	"testing"
	"time"

	"github.com/wrany/libs/events"
	"github.com/wrany/libs/events/location"
)

func validPayload() location.Payload {
	return location.Payload{
		UserID:             "user_123",
		DeviceID:           "device_123",
		RecordedAt:         time.Date(2026, 6, 10, 12, 0, 1, 0, time.UTC),
		ReceivedAt:         time.Date(2026, 6, 10, 12, 0, 4, 0, time.UTC),
		Lat:                55.751244,
		Lon:                37.618423,
		AccuracyM:          8.5,
		SpeedMps:           1.4,
		BearingDeg:         82,
		ActivityType:       "walking",
		ActivityConfidence: 0.87,
		BatteryLevel:       0.74,
		Source:             location.SourceAndroidTracker,
	}
}

func TestPayloadValidate(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(p location.Payload) location.Payload
		wantIssue string // empty means valid
	}{
		{"valid", func(p location.Payload) location.Payload { return p }, ""},
		{"missing user_id", func(p location.Payload) location.Payload { p.UserID = ""; return p }, "user_id"},
		{"missing device_id", func(p location.Payload) location.Payload { p.DeviceID = ""; return p }, "device_id"},
		{"missing recorded_at", func(p location.Payload) location.Payload { p.RecordedAt = time.Time{}; return p }, "recorded_at"},
		{"missing received_at", func(p location.Payload) location.Payload { p.ReceivedAt = time.Time{}; return p }, "received_at"},
		{"lat above range", func(p location.Payload) location.Payload { p.Lat = 90.1; return p }, "lat"},
		{"lat below range", func(p location.Payload) location.Payload { p.Lat = -90.1; return p }, "lat"},
		{"lon above range", func(p location.Payload) location.Payload { p.Lon = 180.1; return p }, "lon"},
		{"lon below range", func(p location.Payload) location.Payload { p.Lon = -180.1; return p }, "lon"},
		{"negative accuracy", func(p location.Payload) location.Payload { p.AccuracyM = -1; return p }, "accuracy_m"},
		{"confidence above 1", func(p location.Payload) location.Payload { p.ActivityConfidence = 1.5; return p }, "activity_confidence"},
		{"battery above 1", func(p location.Payload) location.Payload { p.BatteryLevel = 1.5; return p }, "battery_level"},
		{"missing source", func(p location.Payload) location.Payload { p.Source = ""; return p }, "source"},
		{"boundary lat/lon ok", func(p location.Payload) location.Payload { p.Lat, p.Lon = -90, 180; return p }, ""},
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
			if !events.IsValidationError(err) {
				t.Fatalf("expected validation error, got %v", err)
			}
			if !strings.Contains(err.Error(), tt.wantIssue) {
				t.Errorf("error %q does not mention %q", err, tt.wantIssue)
			}
		})
	}
}

func TestNewEvent_Valid(t *testing.T) {
	p := validPayload()
	env, err := location.NewEvent("evt_123", p.ReceivedAt, "tracking-gateway", "req_123", p)
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	if env.EventType != events.SubjectLocationEvents {
		t.Errorf("event_type = %q, want %q", env.EventType, events.SubjectLocationEvents)
	}
	if !env.OccurredAt.Equal(p.RecordedAt) {
		t.Errorf("occurred_at = %v, want recorded_at %v", env.OccurredAt, p.RecordedAt)
	}

	var decoded location.Payload
	if err := env.DecodePayload(&decoded); err != nil {
		t.Fatalf("DecodePayload: %v", err)
	}
	if decoded != p {
		t.Errorf("payload round-trip mismatch:\n got %+v\nwant %+v", decoded, p)
	}
}

func TestNewEvent_RequiresCorrelationID(t *testing.T) {
	p := validPayload()
	_, err := location.NewEvent("evt_123", p.ReceivedAt, "tracking-gateway", "", p)
	if !events.IsValidationError(err) || !strings.Contains(err.Error(), "correlation_id") {
		t.Fatalf("expected correlation_id validation error, got %v", err)
	}
}

func TestNewEvent_RejectsInvalidPayload(t *testing.T) {
	p := validPayload()
	p.Lat = 200
	if _, err := location.NewEvent("evt_123", p.ReceivedAt, "tracking-gateway", "req_123", p); !events.IsValidationError(err) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestOrderingKey(t *testing.T) {
	got := validPayload().OrderingKey()
	if got != "user_123:device_123" {
		t.Errorf("OrderingKey = %q, want %q", got, "user_123:device_123")
	}
}
