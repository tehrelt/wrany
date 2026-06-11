package nats

import (
	"context"

	"github.com/wrany/libs/eventbus"
)

// NewLocationConsumerForTest creates a LocationConsumer without an eventbus.Consumer,
// suitable for unit-testing the handle method in isolation.
func NewLocationConsumerForTest(processor Processor) *LocationConsumer {
	return &LocationConsumer{consumer: nil, processor: processor}
}

// HandleForTest exposes the internal handle method for unit tests.
func (lc *LocationConsumer) HandleForTest(ctx context.Context, msg eventbus.Message) error {
	return lc.handle(ctx, msg)
}
