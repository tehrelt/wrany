package domain

import "errors"

// ErrTransient marks temporary failures (DB unavailable, context timeout).
// The consumer should NACK the NATS message for redelivery.
var ErrTransient = errors.New("transient error")

// ErrInvalidMessage marks messages that cannot be processed (bad JSON, unknown
// event_type, failed payload validation). The consumer should publish a
// dead-letter.v1 event and then ACK the original NATS message.
var ErrInvalidMessage = errors.New("invalid message")
