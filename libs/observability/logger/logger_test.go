package logger_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/wrany/libs/observability/logger"
)

func newTestLogger(buf *bytes.Buffer, service string) *slog.Logger {
	h := slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(h).With("service", service)
}

func TestLogger_ServiceField(t *testing.T) {
	var buf bytes.Buffer
	l := newTestLogger(&buf, "test-service")
	l.Info("hello")

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("json: %v", err)
	}
	if rec["service"] != "test-service" {
		t.Errorf("service = %v, want test-service", rec["service"])
	}
}

func TestLogger_RequestIDFromContext(t *testing.T) {
	ctx := logger.WithRequestID(context.Background(), "req-abc")
	if id := logger.RequestIDFromContext(ctx); id != "req-abc" {
		t.Errorf("got %q, want req-abc", id)
	}
}

func TestLogger_FromContext_AddsRequestID(t *testing.T) {
	var buf bytes.Buffer
	base := newTestLogger(&buf, "svc")
	ctx := logger.WithRequestID(context.Background(), "req-xyz")

	logger.FromContext(ctx, base).Info("test")

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("json: %v", err)
	}
	if rec["request_id"] != "req-xyz" {
		t.Errorf("request_id = %v, want req-xyz", rec["request_id"])
	}
}

func TestLogger_EmptyContext_NoRequestID(t *testing.T) {
	var buf bytes.Buffer
	base := newTestLogger(&buf, "svc")
	logger.FromContext(context.Background(), base).Info("no id")

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("json: %v", err)
	}
	if _, ok := rec["request_id"]; ok {
		t.Error("request_id should not be present for empty context")
	}
}
