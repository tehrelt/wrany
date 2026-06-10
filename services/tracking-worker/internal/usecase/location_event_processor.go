package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/wrany/libs/eventbus"
	"github.com/wrany/libs/events"
	"github.com/wrany/libs/events/deadletter"
	"github.com/wrany/libs/events/location"
	"github.com/wrany/tracking-worker/internal/domain"
)

// ProcessingInput is the raw bytes of a NATS message payload.
// Transport/nats constructs this from eventbus.Message.Data() and passes it
// to the processor — no NATS types cross this boundary.
type ProcessingInput struct {
	Data []byte
}

// Action describes what the transport layer should do with the original NATS message.
type Action int

const (
	// ActionAck tells transport to call msg.Ack(): processing succeeded or
	// the message was sent to dead-letter successfully.
	ActionAck Action = iota
	// ActionNak tells transport to call msg.Nak(): processing failed transiently
	// and the message should be redelivered.
	ActionNak
)

// ProcessingResult carries the ACK/NAK decision back to transport/nats.
type ProcessingResult struct {
	Action Action
}

// RawLocationRepository persists raw location points.
// Defined here so usecase does not depend on storage packages.
type RawLocationRepository interface {
	Insert(ctx context.Context, p domain.RawLocationPoint) error
}

// LocationEventProcessor processes location.events.v1 messages from NATS.
//
// Processing flow:
//  1. Unmarshal envelope JSON.
//  2. Validate envelope + check event_type.
//  3. Unmarshal and validate location payload.
//  4. Map to domain.RawLocationPoint.
//  5. Insert idempotently (ON CONFLICT DO NOTHING) — duplicate = ACK.
//
// Invalid/unprocessable messages are published to dead-letter.v1 and then ACKed.
// Transient DB errors produce NAK for redelivery.
// Dead-letter publish failures also produce NAK (to avoid silent message loss).
type LocationEventProcessor struct {
	repo         RawLocationRepository
	publisher    eventbus.Publisher
	producer     string
	consumerName string
}

// NewLocationEventProcessor constructs a processor.
// producer is this service's name (e.g. "tracking-worker") used in events.
// consumerName is the NATS durable consumer name used in dead-letter payloads.
func NewLocationEventProcessor(
	repo RawLocationRepository,
	publisher eventbus.Publisher,
	producer, consumerName string,
) *LocationEventProcessor {
	return &LocationEventProcessor{
		repo:         repo,
		publisher:    publisher,
		producer:     producer,
		consumerName: consumerName,
	}
}

// Process runs the full processing pipeline for a single message.
// It never panics; all errors are handled and expressed as ActionAck/ActionNak.
func (p *LocationEventProcessor) Process(ctx context.Context, input ProcessingInput) ProcessingResult {
	// Step 1: unmarshal envelope.
	var env events.Envelope
	if err := json.Unmarshal(input.Data, &env); err != nil {
		return p.deadLetter(ctx, input.Data, "", fmt.Sprintf("invalid JSON: %v", err))
	}

	// Step 2: validate envelope.
	if err := env.Validate(); err != nil {
		return p.deadLetter(ctx, input.Data, env.CorrelationID, fmt.Sprintf("invalid envelope: %v", err))
	}

	// Step 3: check event type.
	if env.EventType != events.SubjectLocationEvents {
		return p.deadLetter(ctx, input.Data, env.CorrelationID,
			fmt.Sprintf("unsupported event_type: %q", env.EventType))
	}

	// Step 4: decode and validate location payload.
	var payload location.Payload
	if err := env.DecodePayload(&payload); err != nil {
		return p.deadLetter(ctx, input.Data, env.CorrelationID,
			fmt.Sprintf("cannot decode location payload: %v", err))
	}
	if err := payload.Validate(); err != nil {
		return p.deadLetter(ctx, input.Data, env.CorrelationID,
			fmt.Sprintf("invalid location payload: %v", err))
	}

	// Step 5: map to domain type.
	point, err := mapPayloadToPoint(env, payload)
	if err != nil {
		return p.deadLetter(ctx, input.Data, env.CorrelationID,
			fmt.Sprintf("map payload: %v", err))
	}

	// Step 6: insert idempotently.
	if err := p.repo.Insert(ctx, point); err != nil {
		return ProcessingResult{Action: ActionNak}
	}
	return ProcessingResult{Action: ActionAck}
}

// deadLetter publishes a dead-letter.v1 event for an unprocessable message.
// Returns ActionAck if publish succeeds, ActionNak if it fails.
func (p *LocationEventProcessor) deadLetter(
	ctx context.Context,
	originalData []byte,
	correlationID string,
	reason string,
) ProcessingResult {
	now := time.Now().UTC()

	// json.RawMessage must contain valid JSON. If originalData is not valid JSON
	// (e.g. truncated or binary), encode it as a JSON string so serialization succeeds.
	rawEvent := json.RawMessage(originalData)
	if !json.Valid(originalData) {
		escaped, _ := json.Marshal(string(originalData))
		rawEvent = json.RawMessage(escaped)
	}

	dlPayload := deadletter.Payload{
		OriginalSubject: events.SubjectLocationEvents,
		OriginalEvent:   rawEvent,
		Error:           reason,
		FailedAt:        now,
		Consumer:        p.consumerName,
	}
	dlEvent, err := deadletter.NewEvent(uuid.New().String(), now, p.producer, correlationID, dlPayload)
	if err != nil {
		return ProcessingResult{Action: ActionNak}
	}
	if err := p.publisher.Publish(ctx, events.SubjectDeadLetter, dlEvent); err != nil {
		return ProcessingResult{Action: ActionNak}
	}
	return ProcessingResult{Action: ActionAck}
}

// mapPayloadToPoint converts a validated location.Payload to domain.RawLocationPoint.
// Returns an error only if UUID parsing fails (treated as invalid message, not transient).
func mapPayloadToPoint(env events.Envelope, p location.Payload) (domain.RawLocationPoint, error) {
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		return domain.RawLocationPoint{}, fmt.Errorf("invalid user_id %q: %w", p.UserID, err)
	}
	deviceID, err := uuid.Parse(p.DeviceID)
	if err != nil {
		return domain.RawLocationPoint{}, fmt.Errorf("invalid device_id %q: %w", p.DeviceID, err)
	}

	speedMps := p.SpeedMps
	bearingDeg := p.BearingDeg
	actConf := p.ActivityConfidence
	battLevel := p.BatteryLevel

	return domain.RawLocationPoint{
		UserID:             userID,
		DeviceID:           deviceID,
		EventID:            env.EventID,
		RecordedAt:         p.RecordedAt,
		ReceivedAt:         p.ReceivedAt,
		Lat:                p.Lat,
		Lon:                p.Lon,
		AccuracyM:          p.AccuracyM,
		SpeedMps:           &speedMps,
		BearingDeg:         &bearingDeg,
		ActivityType:       p.ActivityType,
		ActivityConfidence: &actConf,
		BatteryLevel:       &battLevel,
		Source:             p.Source,
	}, nil
}
