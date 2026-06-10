// Package eventbus defines the broker-agnostic publishing abstraction for the
// WR any% event bus. Usecase code depends on this package only — never on
// concrete broker types (NATS today; Kafka/Redpanda are possible future adapters).
package eventbus

import (
	"context"
	"errors"
	"fmt"

	"github.com/wrany/libs/events"
)

// ErrPublish marks broker-side publish failures so callers can distinguish
// them from validation errors.
var ErrPublish = errors.New("eventbus: publish failed")

// Publisher publishes validated events to the internal bus.
//
// EPIC 3 implements only the publish side. A full EventBus abstraction
// (Publisher + Consumer with durable names, ack/nack, redelivery and graceful
// shutdown) is planned for the epic that introduces the first real consumer.
type Publisher interface {
	Publish(ctx context.Context, subject string, event events.Envelope) error
}

// NopPublisher is a Publisher that always returns ErrPublish.
// Used when no broker is configured (e.g. local dev without NATS_URL).
type NopPublisher struct{}

func (NopPublisher) Publish(_ context.Context, _ string, _ events.Envelope) error {
	return fmt.Errorf("%w: no broker configured", ErrPublish)
}
