package tracing

import (
	"context"

	"go.opentelemetry.io/otel"
)

// natsMsgCarrier adapts NATS message headers (map[string][]string) to the
// W3C TextMapCarrier interface so OTel can inject/extract traceparent.
type natsMsgCarrier map[string][]string

func (c natsMsgCarrier) Get(key string) string {
	vals := c[key]
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

func (c natsMsgCarrier) Set(key, value string) {
	c[key] = []string{value}
}

func (c natsMsgCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// InjectToHeaders writes W3C traceparent/tracestate from ctx into NATS headers.
// Call this in the publisher before sending the message.
func InjectToHeaders(ctx context.Context, headers map[string][]string) {
	otel.GetTextMapPropagator().Inject(ctx, natsMsgCarrier(headers))
}

// ExtractFromHeaders reads W3C trace context from NATS message headers and
// returns a context carrying that trace. Call this in the consumer before
// starting the processing span so the consumer span is linked to the publisher.
func ExtractFromHeaders(headers map[string][]string) context.Context {
	return otel.GetTextMapPropagator().Extract(context.Background(), natsMsgCarrier(headers))
}
