package route_test

import (
	"strings"
	"testing"
	"time"

	"github.com/wrany/libs/events"
	"github.com/wrany/libs/events/route"
)

func validPayload() route.MatchedPayload {
	return route.MatchedPayload{
		TripID:     "trip_1",
		RouteID:    "route_1",
		UserID:     "user_1",
		MatchedAt:  time.Date(2026, 6, 10, 12, 30, 5, 0, time.UTC),
		MatchScore: 0.93,
	}
}

func TestMatchedPayloadValidate(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(p route.MatchedPayload) route.MatchedPayload
		wantIssue string
	}{
		{"valid", func(p route.MatchedPayload) route.MatchedPayload { return p }, ""},
		{"missing trip_id", func(p route.MatchedPayload) route.MatchedPayload { p.TripID = ""; return p }, "trip_id"},
		{"missing route_id", func(p route.MatchedPayload) route.MatchedPayload { p.RouteID = ""; return p }, "route_id"},
		{"missing user_id", func(p route.MatchedPayload) route.MatchedPayload { p.UserID = ""; return p }, "user_id"},
		{"missing matched_at", func(p route.MatchedPayload) route.MatchedPayload { p.MatchedAt = time.Time{}; return p }, "matched_at"},
		{"score above 1", func(p route.MatchedPayload) route.MatchedPayload { p.MatchScore = 1.1; return p }, "match_score"},
		{"score below 0", func(p route.MatchedPayload) route.MatchedPayload { p.MatchScore = -0.1; return p }, "match_score"},
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

func TestNewMatchedEvent(t *testing.T) {
	p := validPayload()
	env, err := route.NewMatchedEvent("evt_1", p.MatchedAt.Add(time.Second), "route-worker", "", p)
	if err != nil {
		t.Fatalf("NewMatchedEvent: %v", err)
	}
	if env.EventType != events.SubjectRouteMatched || !env.OccurredAt.Equal(p.MatchedAt) {
		t.Errorf("unexpected envelope: %+v", env)
	}
}
