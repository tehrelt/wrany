package nats

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/wrany/libs/eventbus"
)

// ConsumerConfig holds durable consumer settings for a single subject.
type ConsumerConfig struct {
	// Stream is the JetStream stream name, e.g. "WRANY_EVENTS".
	Stream string
	// FilterSubject is the subject to consume, e.g. "location.events.v1".
	FilterSubject string
	// DurableName is the consumer durable name (survives restarts). Required.
	DurableName string
	// AckWait is the time the server waits for an ACK before redelivering. Default 30s.
	AckWait time.Duration
	// MaxDeliver limits redelivery attempts before NATS stops delivering.
	// -1 means unlimited. Default 5.
	// NOTE: MaxDeliver limits NATS-side redelivery only. It is unrelated to the
	// dead-letter.v1 subject, which is published explicitly by consumer code for
	// invalid/unprocessable messages.
	MaxDeliver int
	// FetchBatchSize is the maximum messages to fetch per call. Default 100.
	FetchBatchSize int
	// FetchTimeout is the maximum wait per Fetch call before returning an empty
	// batch. Default 5s. Keeps the loop responsive to context cancellation.
	FetchTimeout time.Duration
}

func (c ConsumerConfig) withDefaults() ConsumerConfig {
	if c.AckWait == 0 {
		c.AckWait = 30 * time.Second
	}
	if c.MaxDeliver == 0 {
		c.MaxDeliver = 5
	}
	if c.FetchBatchSize == 0 {
		c.FetchBatchSize = 100
	}
	if c.FetchTimeout == 0 {
		c.FetchTimeout = 5 * time.Second
	}
	return c
}

// JetStreamConsumer implements eventbus.Consumer using a pull-based durable consumer.
// It calls CreateOrUpdateConsumer on construction, so the consumer survives service
// restarts and resumes from the last acknowledged message.
type JetStreamConsumer struct {
	consumer jetstream.Consumer
	cfg      ConsumerConfig
}

var _ eventbus.Consumer = (*JetStreamConsumer)(nil)

// NewJetStreamConsumer creates or updates the durable consumer on the stream.
// Safe to call on every service start: existing consumers with the same config
// are returned as-is; config changes are applied atomically by JetStream.
func NewJetStreamConsumer(ctx context.Context, b *Bus, cfg ConsumerConfig) (*JetStreamConsumer, error) {
	cfg = cfg.withDefaults()
	consumer, err := b.js.CreateOrUpdateConsumer(ctx, cfg.Stream, jetstream.ConsumerConfig{
		Durable:       cfg.DurableName,
		FilterSubject: cfg.FilterSubject,
		DeliverPolicy: jetstream.DeliverAllPolicy,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       cfg.AckWait,
		MaxDeliver:    cfg.MaxDeliver,
	})
	if err != nil {
		return nil, fmt.Errorf("nats: create consumer %q: %w", cfg.DurableName, err)
	}
	return &JetStreamConsumer{consumer: consumer, cfg: cfg}, nil
}

// Consume runs the pull-fetch loop, delivering messages to handler until ctx
// is cancelled. Returns nil on graceful shutdown.
// No busy loop: each Fetch call blocks up to FetchTimeout before returning an
// empty batch, giving the loop a chance to check ctx.Done().
func (c *JetStreamConsumer) Consume(ctx context.Context, handler eventbus.MessageHandler) error {
	for {
		if ctx.Err() != nil {
			return nil
		}

		batch, err := c.consumer.Fetch(c.cfg.FetchBatchSize, jetstream.FetchMaxWait(c.cfg.FetchTimeout))
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			log.Printf("eventbus/nats consumer %q: fetch error: %v", c.cfg.DurableName, err)
			continue
		}

		for msg := range batch.Messages() {
			if ctx.Err() != nil {
				return nil
			}
			if herr := handler(ctx, &natsMessage{msg: msg}); herr != nil {
				log.Printf("eventbus/nats consumer %q: handler error on %q: %v",
					c.cfg.DurableName, msg.Subject(), herr)
			}
		}

		if batchErr := batch.Error(); batchErr != nil && ctx.Err() == nil {
			log.Printf("eventbus/nats consumer %q: batch error: %v", c.cfg.DurableName, batchErr)
		}
	}
}

// Close is a no-op for pull consumers. The underlying NATS connection is owned
// by Bus and must be closed separately via Bus.Close().
func (c *JetStreamConsumer) Close() error { return nil }

// natsMessage wraps jetstream.Msg and implements eventbus.Message.
// NATS types are contained here and never leak into usecase or domain.
type natsMessage struct {
	msg jetstream.Msg
}

func (m *natsMessage) Subject() string { return m.msg.Subject() }
func (m *natsMessage) Data() []byte    { return m.msg.Data() }
func (m *natsMessage) Headers() map[string][]string {
	h := m.msg.Headers()
	if h == nil {
		return nil
	}
	return map[string][]string(h)
}
func (m *natsMessage) Ack() error { return m.msg.Ack() }
func (m *natsMessage) Nak() error { return m.msg.Nak() }
