package eventbus

import (
	"context"
	"errors"
)

// ErrConsume marks fatal consumer-side failures so callers can distinguish them
// from handler errors.
var ErrConsume = errors.New("eventbus: consume failed")

// Message is a received event from the bus. The receiver must call either
// Ack or Nak exactly once to settle the message.
// NATS-specific types never appear here; implementations wrap them.
type Message interface {
	// Subject returns the subject this message was published to.
	Subject() string
	// Data returns the raw message bytes (serialized events.Envelope).
	Data() []byte
	// Headers returns the message headers.
	Headers() map[string][]string
	// Ack acknowledges successful processing. The broker will not redeliver.
	Ack() error
	// Nak signals the message should be redelivered.
	Nak() error
}

// MessageHandler is the callback invoked by Consumer for each received message.
// The handler is responsible for calling msg.Ack() or msg.Nak() exactly once.
// The error return value is for logging only and does not affect delivery semantics.
type MessageHandler func(ctx context.Context, msg Message) error

// Consumer reads messages from a durable subscription and delivers them to a
// MessageHandler. Consume blocks until ctx is cancelled or a fatal error occurs.
type Consumer interface {
	// Consume starts the message fetch loop. Returns nil on graceful context
	// cancellation, ErrConsume (or a wrapped error) on fatal failure.
	Consume(ctx context.Context, handler MessageHandler) error
	// Close releases consumer resources. Call only after Consume returns.
	Close() error
}
