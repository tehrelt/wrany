package trip_test

import (
	"strings"
	"testing"
	"time"

	"github.com/wrany/libs/events"
	"github.com/wrany/libs/events/trip"
)

var (
	startedAt   = time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	updatedAt   = time.Date(2026, 6, 10, 12, 5, 0, 0, time.UTC)
	completedAt = time.Date(2026, 6, 10, 12, 30, 0, 0, time.UTC)
)

func TestStartedPayloadValidate(t *testing.T) {
	valid := trip.StartedPayload{TripID: "trip_1", UserID: "user_1", DeviceID: "device_1", StartedAt: startedAt}

	if err := valid.Validate(); err != nil {
		t.Fatalf("valid payload rejected: %v", err)
	}

	tests := []struct {
		name      string
		mutate    func(p trip.StartedPayload) trip.StartedPayload
		wantIssue string
	}{
		{"missing trip_id", func(p trip.StartedPayload) trip.StartedPayload { p.TripID = ""; return p }, "trip_id"},
		{"missing user_id", func(p trip.StartedPayload) trip.StartedPayload { p.UserID = ""; return p }, "user_id"},
		{"missing device_id", func(p trip.StartedPayload) trip.StartedPayload { p.DeviceID = ""; return p }, "device_id"},
		{"missing started_at", func(p trip.StartedPayload) trip.StartedPayload { p.StartedAt = time.Time{}; return p }, "started_at"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.mutate(valid).Validate()
			if !events.IsValidationError(err) || !strings.Contains(err.Error(), tt.wantIssue) {
				t.Fatalf("expected %q validation error, got %v", tt.wantIssue, err)
			}
		})
	}
}

func TestUpdatedPayloadValidate(t *testing.T) {
	valid := trip.UpdatedPayload{
		TripID: "trip_1", UserID: "user_1", DeviceID: "device_1",
		UpdatedAt: updatedAt, DistanceM: 1200, DurationS: 300, PointCount: 60,
	}

	if err := valid.Validate(); err != nil {
		t.Fatalf("valid payload rejected: %v", err)
	}

	tests := []struct {
		name      string
		mutate    func(p trip.UpdatedPayload) trip.UpdatedPayload
		wantIssue string
	}{
		{"missing trip_id", func(p trip.UpdatedPayload) trip.UpdatedPayload { p.TripID = ""; return p }, "trip_id"},
		{"missing updated_at", func(p trip.UpdatedPayload) trip.UpdatedPayload { p.UpdatedAt = time.Time{}; return p }, "updated_at"},
		{"negative distance", func(p trip.UpdatedPayload) trip.UpdatedPayload { p.DistanceM = -1; return p }, "distance_m"},
		{"negative duration", func(p trip.UpdatedPayload) trip.UpdatedPayload { p.DurationS = -1; return p }, "duration_s"},
		{"negative point_count", func(p trip.UpdatedPayload) trip.UpdatedPayload { p.PointCount = -1; return p }, "point_count"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.mutate(valid).Validate()
			if !events.IsValidationError(err) || !strings.Contains(err.Error(), tt.wantIssue) {
				t.Fatalf("expected %q validation error, got %v", tt.wantIssue, err)
			}
		})
	}
}

func TestCompletedPayloadValidate(t *testing.T) {
	valid := trip.CompletedPayload{
		TripID: "trip_1", UserID: "user_1", DeviceID: "device_1",
		StartedAt: startedAt, CompletedAt: completedAt,
		DistanceM: 5000, DurationS: 1800, PointCount: 360,
	}

	if err := valid.Validate(); err != nil {
		t.Fatalf("valid payload rejected: %v", err)
	}

	t.Run("completed before started", func(t *testing.T) {
		p := valid
		p.CompletedAt = startedAt.Add(-time.Minute)
		err := p.Validate()
		if !events.IsValidationError(err) || !strings.Contains(err.Error(), "completed_at") {
			t.Fatalf("expected completed_at ordering error, got %v", err)
		}
	})
}

func TestTripEventConstructors(t *testing.T) {
	producedAt := completedAt.Add(2 * time.Second)

	started, err := trip.NewStartedEvent("evt_1", producedAt, "tracking-worker", "req_1",
		trip.StartedPayload{TripID: "trip_1", UserID: "user_1", DeviceID: "device_1", StartedAt: startedAt})
	if err != nil {
		t.Fatalf("NewStartedEvent: %v", err)
	}
	if started.EventType != events.SubjectTripStarted || !started.OccurredAt.Equal(startedAt) {
		t.Errorf("unexpected started envelope: %+v", started)
	}

	updated, err := trip.NewUpdatedEvent("evt_2", producedAt, "tracking-worker", "",
		trip.UpdatedPayload{TripID: "trip_1", UserID: "user_1", DeviceID: "device_1", UpdatedAt: updatedAt, DistanceM: 1, DurationS: 1, PointCount: 1})
	if err != nil {
		t.Fatalf("NewUpdatedEvent: %v", err)
	}
	if updated.EventType != events.SubjectTripUpdated {
		t.Errorf("event_type = %q", updated.EventType)
	}

	completed, err := trip.NewCompletedEvent("evt_3", producedAt, "tracking-worker", "",
		trip.CompletedPayload{TripID: "trip_1", UserID: "user_1", DeviceID: "device_1", StartedAt: startedAt, CompletedAt: completedAt})
	if err != nil {
		t.Fatalf("NewCompletedEvent: %v", err)
	}
	if completed.EventType != events.SubjectTripCompleted || !completed.OccurredAt.Equal(completedAt) {
		t.Errorf("unexpected completed envelope: %+v", completed)
	}

	if _, err := trip.NewStartedEvent("evt_4", producedAt, "tracking-worker", "", trip.StartedPayload{}); !events.IsValidationError(err) {
		t.Errorf("expected validation error for empty payload, got %v", err)
	}
}
