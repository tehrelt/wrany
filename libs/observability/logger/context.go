package logger

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

type ctxKey int

const (
	requestIDKey ctxKey = iota
	userIDKey
	deviceIDKey
)

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey).(string)
	return v
}

func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

func UserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(userIDKey).(string)
	return v
}

func WithDeviceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, deviceIDKey, id)
}

func DeviceIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(deviceIDKey).(string)
	return v
}

// WithTraceID enriches base with trace_id and span_id from the active OTel span in ctx.
// When tracing is disabled (noop provider) the span context is invalid and nothing is added.
func WithTraceID(ctx context.Context, base *slog.Logger) *slog.Logger {
	sc := trace.SpanFromContext(ctx).SpanContext()
	if sc.IsValid() {
		return base.With("trace_id", sc.TraceID().String(), "span_id", sc.SpanID().String())
	}
	return base
}

// FromContext returns a logger enriched with request_id, user_id, device_id, and trace_id from ctx.
func FromContext(ctx context.Context, base *slog.Logger) *slog.Logger {
	l := base
	if id := RequestIDFromContext(ctx); id != "" {
		l = l.With("request_id", id)
	}
	if id := UserIDFromContext(ctx); id != "" {
		l = l.With("user_id", id)
	}
	if id := DeviceIDFromContext(ctx); id != "" {
		l = l.With("device_id", id)
	}
	return WithTraceID(ctx, l)
}
