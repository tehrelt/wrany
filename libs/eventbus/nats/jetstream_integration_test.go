//go:build integration

// Integration test for the JetStream adapter.
// Requires a running NATS with JetStream (make up), reachable on 127.0.0.1:4222.
// Run manually: go test -tags integration ./nats/...
package nats

import (
	"context"
	"testing"
	"time"

	"github.com/wrany/libs/events"
	"github.com/wrany/libs/events/location"
)

const testURL = "nats://127.0.0.1:4222"

func connectOrSkip(t *testing.T) *Bus {
	t.Helper()
	bus, err := Connect(Config{URL: testURL, Stream: events.StreamName})
	if err != nil {
		t.Skipf("NATS is not reachable at %s (run `make up` first): %v", testURL, err)
	}
	t.Cleanup(func() {
		if err := bus.Close(); err != nil {
			t.Logf("close bus: %v", err)
		}
	})
	return bus
}

func locationEnvelope(t *testing.T, eventID string) events.Envelope {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	env, err := location.NewEvent(eventID, now, "integration-test", "req_integration", location.Payload{
		UserID:     "user_it",
		DeviceID:   "device_it",
		RecordedAt: now.Add(-time.Second),
		ReceivedAt: now,
		Lat:        55.751244,
		Lon:        37.618423,
		AccuracyM:  5,
		Source:     location.SourceAndroidTracker,
	})
	if err != nil {
		t.Fatalf("build location event: %v", err)
	}
	return env
}

func streamMessages(t *testing.T, bus *Bus, ctx context.Context) uint64 {
	t.Helper()
	stream, err := bus.js.Stream(ctx, bus.stream)
	if err != nil {
		t.Fatalf("get stream: %v", err)
	}
	info, err := stream.Info(ctx)
	if err != nil {
		t.Fatalf("stream info: %v", err)
	}
	return info.State.Msgs
}

func TestEnsureStream_Idempotent(t *testing.T) {
	bus := connectOrSkip(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := bus.EnsureStream(ctx); err != nil {
		t.Fatalf("first EnsureStream: %v", err)
	}
	if err := bus.EnsureStream(ctx); err != nil {
		t.Fatalf("second EnsureStream must be idempotent: %v", err)
	}
}

func TestPublish_AckAndDeduplication(t *testing.T) {
	bus := connectOrSkip(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := bus.EnsureStream(ctx); err != nil {
		t.Fatalf("EnsureStream: %v", err)
	}

	eventID := "evt_it_" + time.Now().UTC().Format("20060102150405.000000000")
	env := locationEnvelope(t, eventID)

	before := streamMessages(t, bus, ctx)

	if err := bus.Publish(ctx, events.SubjectLocationEvents, env); err != nil {
		t.Fatalf("first publish: %v", err)
	}
	afterFirst := streamMessages(t, bus, ctx)
	if afterFirst != before+1 {
		t.Fatalf("first publish: messages %d -> %d, want +1", before, afterFirst)
	}

	// Repeated publish with the same event_id within the dedup window:
	// JetStream must drop the duplicate (best-effort publisher retry protection).
	if err := bus.Publish(ctx, events.SubjectLocationEvents, env); err != nil {
		t.Fatalf("duplicate publish returned error (expected silent dedup ack): %v", err)
	}
	afterDuplicate := streamMessages(t, bus, ctx)
	if afterDuplicate != afterFirst {
		t.Fatalf("duplicate publish created a message: %d -> %d, want unchanged", afterFirst, afterDuplicate)
	}
}

func TestPublish_SetsDedupAndMetadataHeaders(t *testing.T) {
	bus := connectOrSkip(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := bus.EnsureStream(ctx); err != nil {
		t.Fatalf("EnsureStream: %v", err)
	}

	eventID := "evt_hdr_" + time.Now().UTC().Format("20060102150405.000000000")
	env := locationEnvelope(t, eventID)
	if err := bus.Publish(ctx, events.SubjectLocationEvents, env); err != nil {
		t.Fatalf("publish: %v", err)
	}

	stream, err := bus.js.Stream(ctx, bus.stream)
	if err != nil {
		t.Fatalf("get stream: %v", err)
	}
	msg, err := stream.GetLastMsgForSubject(ctx, events.SubjectLocationEvents)
	if err != nil {
		t.Fatalf("get last message: %v", err)
	}

	if got := msg.Header.Get(events.HeaderMsgID); got != eventID {
		t.Errorf("%s = %q, want %q", events.HeaderMsgID, got, eventID)
	}
	if got := msg.Header.Get(events.HeaderEventType); got != events.SubjectLocationEvents {
		t.Errorf("%s = %q, want %q", events.HeaderEventType, got, events.SubjectLocationEvents)
	}
	if got := msg.Header.Get(events.HeaderCorrelationID); got != "req_integration" {
		t.Errorf("%s = %q, want %q", events.HeaderCorrelationID, got, "req_integration")
	}
}

func TestPublish_RejectsInvalidEnvelope(t *testing.T) {
	bus := connectOrSkip(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := bus.Publish(ctx, events.SubjectLocationEvents, events.Envelope{}); !events.IsValidationError(err) {
		t.Fatalf("expected validation error, got %v", err)
	}
}
