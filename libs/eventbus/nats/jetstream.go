package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel"

	"github.com/wrany/libs/events"
	"github.com/wrany/libs/eventbus"
)

// otelCarrier adapts NATS headers to the W3C TextMapCarrier interface.
type otelCarrier map[string][]string

func (c otelCarrier) Get(key string) string {
	if v := c[key]; len(v) > 0 {
		return v[0]
	}
	return ""
}

func (c otelCarrier) Set(key, value string) { c[key] = []string{value} }

func (c otelCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// dedupWindow is the JetStream duplicate-tracking window. Within this window
// a repeated Nats-Msg-Id (= envelope event_id) is dropped by the server.
// This is best-effort publisher retry protection, not business idempotency.
const dedupWindow = 2 * time.Minute

// Bus is a NATS JetStream implementation of eventbus.Publisher.
type Bus struct {
	conn   *nats.Conn
	js     jetstream.JetStream
	stream string
}

var _ eventbus.Publisher = (*Bus)(nil)

// Connect establishes a NATS connection and a JetStream context.
func Connect(cfg Config) (*Bus, error) {
	conn, err := nats.Connect(cfg.URL, nats.Name("wrany-eventbus"))
	if err != nil {
		return nil, fmt.Errorf("nats: connect to %q: %w", cfg.URL, err)
	}
	js, err := jetstream.New(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("nats: create jetstream context: %w", err)
	}
	return &Bus{conn: conn, js: js, stream: cfg.Stream}, nil
}

// EnsureStream idempotently creates or updates the event stream with the
// project subject filters, file storage and the dedup window.
// Dev/MVP mechanism: services call it on startup; production stream
// provisioning is hardened in a later epic.
func (b *Bus) EnsureStream(ctx context.Context) error {
	_, err := b.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:       b.stream,
		Subjects:   events.StreamSubjects(),
		Storage:    jetstream.FileStorage,
		Retention:  jetstream.LimitsPolicy,
		Duplicates: dedupWindow,
	})
	if err != nil {
		return fmt.Errorf("nats: ensure stream %q: %w", b.stream, err)
	}
	return nil
}

// Publish validates the envelope and publishes it to the subject, waiting for
// the JetStream ack. Headers carry the dedup id, event type and correlation id.
func (b *Bus) Publish(ctx context.Context, subject string, event events.Envelope) error {
	if err := event.Validate(); err != nil {
		return err
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("nats: marshal envelope %q: %w", event.EventID, err)
	}

	msg := &nats.Msg{Subject: subject, Data: data, Header: nats.Header{}}
	msg.Header.Set(events.HeaderMsgID, event.EventID)
	msg.Header.Set(events.HeaderEventType, event.EventType)
	if event.CorrelationID != "" {
		msg.Header.Set(events.HeaderCorrelationID, event.CorrelationID)
	}
	otel.GetTextMapPropagator().Inject(ctx, otelCarrier(msg.Header))

	if _, err := b.js.PublishMsg(ctx, msg); err != nil {
		return fmt.Errorf("%w: subject %q event %q: %w", eventbus.ErrPublish, subject, event.EventID, err)
	}
	return nil
}

// Ping checks NATS connectivity by verifying the connection state and querying
// JetStream account info. Used by /readyz health checks.
func (b *Bus) Ping(ctx context.Context) error {
	if !b.conn.IsConnected() {
		return fmt.Errorf("nats: not connected")
	}
	if _, err := b.js.AccountInfo(ctx); err != nil {
		return fmt.Errorf("nats: account info: %w", err)
	}
	return nil
}

// Close drains the connection, flushing pending publishes before closing.
func (b *Bus) Close() error {
	if err := b.conn.Drain(); err != nil {
		return fmt.Errorf("nats: drain connection: %w", err)
	}
	return nil
}
