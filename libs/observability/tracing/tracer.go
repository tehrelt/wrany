// Package tracing bootstraps OpenTelemetry and provides zero-boilerplate span helpers.
//
// Initialise once in main.go:
//
//	shutdown, err := tracing.Init(ctx, tracing.Config{...})
//	defer shutdown(ctx)
//
// Then instrument any function:
//
//	func (r *repo) GetPoints(ctx context.Context, ...) (pts []Point, err error) {
//	    ctx, end := tracing.Start(ctx, "repo.GetPoints", tracing.Str("user_id", userID))
//	    defer end(&err)
//	    ...
//	}
//
// Or the functional form when the called code returns its own error:
//
//	err = tracing.Do(ctx, "nats.publish", func(ctx context.Context) error {
//	    return bus.Publish(ctx, subject, event)
//	})
package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// Config configures the tracer provider.
type Config struct {
	ServiceName  string
	OTLPEndpoint string // e.g. "tempo:4317" — no scheme
	Enabled      bool   // false → noop provider, zero overhead
}

// Init sets up the global TracerProvider and TextMapPropagator.
// When Enabled is false it installs a no-op provider so all tracing calls
// are safe to call without any side effects.
// The returned shutdown function must be deferred from main.
func Init(ctx context.Context, cfg Config) (shutdown func(context.Context) error, err error) {
	if !cfg.Enabled {
		otel.SetTracerProvider(tracenoop.NewTracerProvider())
		return func(context.Context) error { return nil }, nil
	}

	exp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("tracing: create OTLP exporter: %w", err)
	}

	res, err := sdkresource.New(ctx,
		sdkresource.WithAttributes(semconv.ServiceName(cfg.ServiceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("tracing: create resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

// tracer returns the global tracer keyed by package path.
// Using a fixed name means all spans share the same instrumentation library entry in Tempo.
func tracer() trace.Tracer {
	return otel.Tracer("github.com/wrany/wrany")
}

// Start begins a new span as a child of the span in ctx.
// The returned end function MUST be deferred; pass a pointer to the function's
// named error return so the span captures errors automatically:
//
//	func foo(ctx context.Context) (err error) {
//	    ctx, end := tracing.Start(ctx, "foo")
//	    defer end(&err)
//	    ...
//	}
//
// Pass nil to end() when there is no error return or errors are irrelevant.
func Start(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, func(*error)) {
	ctx, span := tracer().Start(ctx, name, trace.WithAttributes(attrs...))
	return ctx, func(errPtr *error) {
		if errPtr != nil && *errPtr != nil {
			span.RecordError(*errPtr)
			span.SetStatus(codes.Error, (*errPtr).Error())
		} else {
			span.SetStatus(codes.Ok, "")
		}
		span.End()
	}
}

// Do runs fn inside a new span, records any returned error on the span,
// and returns the error. Useful for wrapping a single call without
// needing to juggle defer:
//
//	err = tracing.Do(ctx, "nats.publish", func(ctx context.Context) error {
//	    return bus.Publish(ctx, subject, event)
//	}, tracing.Str("subject", subject))
func Do(ctx context.Context, name string, fn func(context.Context) error, attrs ...attribute.KeyValue) error {
	ctx, span := tracer().Start(ctx, name, trace.WithAttributes(attrs...))
	defer span.End()
	err := fn(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
	return err
}

// Str is a shorthand for attribute.String — reduces import noise at call sites.
func Str(key, value string) attribute.KeyValue { return attribute.String(key, value) }

// Int is a shorthand for attribute.Int.
func Int(key string, value int) attribute.KeyValue { return attribute.Int(key, value) }
