// Package nats wires the eventbus.Consumer to the location event usecase.
// This is the only package in tracking-worker that calls msg.Ack() / msg.Nak().
// The usecase returns a ProcessingResult; this package executes the decision.
package nats

import (
	"context"
	"log"

	"github.com/wrany/libs/eventbus"
	"github.com/wrany/tracking-worker/internal/usecase"
)

// Processor is the subset of usecase.LocationEventProcessor used by this transport.
type Processor interface {
	Process(ctx context.Context, input usecase.ProcessingInput) usecase.ProcessingResult
}

// LocationConsumer bridges eventbus.Consumer and the location event processor.
// It owns the ACK/NAK boundary: the processor never calls Ack or Nak directly.
type LocationConsumer struct {
	consumer  eventbus.Consumer
	processor Processor
}

// NewLocationConsumer creates a LocationConsumer.
func NewLocationConsumer(consumer eventbus.Consumer, processor Processor) *LocationConsumer {
	return &LocationConsumer{consumer: consumer, processor: processor}
}

// Run starts the consume loop and blocks until ctx is cancelled.
// Returns nil on graceful shutdown.
func (lc *LocationConsumer) Run(ctx context.Context) error {
	return lc.consumer.Consume(ctx, lc.handle)
}

// Close releases consumer resources. Call only after Run returns.
func (lc *LocationConsumer) Close() error {
	return lc.consumer.Close()
}

// handle is the MessageHandler passed to the consumer.
// It is the sole caller of msg.Ack() and msg.Nak() in tracking-worker.
func (lc *LocationConsumer) handle(ctx context.Context, msg eventbus.Message) error {
	input := usecase.ProcessingInput{Data: msg.Data()}
	result := lc.processor.Process(ctx, input)

	switch result.Action {
	case usecase.ActionAck:
		if err := msg.Ack(); err != nil {
			log.Printf("location_consumer: ack failed on %q: %v", msg.Subject(), err)
			return err
		}
	case usecase.ActionNak:
		if err := msg.Nak(); err != nil {
			log.Printf("location_consumer: nak failed on %q: %v", msg.Subject(), err)
			return err
		}
	default:
		log.Printf("location_consumer: unknown action %d on %q, naking", result.Action, msg.Subject())
		if err := msg.Nak(); err != nil {
			return err
		}
	}
	return nil
}
