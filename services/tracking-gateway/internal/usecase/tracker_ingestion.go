package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/wrany/libs/eventbus"
	liblocation "github.com/wrany/libs/events/location"
	obslogger "github.com/wrany/libs/observability/logger"
	"github.com/wrany/tracking-gateway/internal/domain"
)


// DeviceLookup is the narrow interface the ingestion usecase requires.
// The postgres DeviceRepo satisfies this interface.
type DeviceLookup interface {
	FindByUserAndDeviceID(ctx context.Context, userID, deviceID uuid.UUID) (*domain.Device, error)
}

// IngestionDedupRepo is the dedup ledger abstraction for the ingestion usecase.
type IngestionDedupRepo interface {
	IsDuplicate(ctx context.Context, userID, deviceID uuid.UUID, eventID string) (bool, error)
	MarkPublished(ctx context.Context, userID, deviceID uuid.UUID, eventID string) error
}

// BatchResult holds the outcome of processing a location.batch.
type BatchResult struct {
	Accepted   []string
	Duplicated []string
	Rejected   []domain.RejectedEvent
}

// TrackerIngestionUseCase handles session management and location batch ingestion.
type TrackerIngestionUseCase struct {
	devices  DeviceLookup
	dedup    IngestionDedupRepo
	pub      eventbus.Publisher
	producer string
	maxBatch int
	log      *slog.Logger
}

// NewTrackerIngestionUseCase constructs the usecase with its dependencies.
func NewTrackerIngestionUseCase(
	devices DeviceLookup,
	dedup IngestionDedupRepo,
	pub eventbus.Publisher,
	producer string,
	maxBatch int,
	log *slog.Logger,
) *TrackerIngestionUseCase {
	return &TrackerIngestionUseCase{
		devices:  devices,
		dedup:    dedup,
		pub:      pub,
		producer: producer,
		maxBatch: maxBatch,
		log:      log,
	}
}

// StartSession validates device ownership and creates a new TrackerSession.
// Returns domain.ErrDeviceNotFound if the device is unknown or does not belong
// to userID (callers map this to DEVICE_NOT_REGISTERED).
func (uc *TrackerIngestionUseCase) StartSession(ctx context.Context, userID, deviceID uuid.UUID) (*domain.TrackerSession, error) {
	_, err := uc.devices.FindByUserAndDeviceID(ctx, userID, deviceID)
	if err != nil {
		if errors.Is(err, domain.ErrDeviceNotFound) {
			return nil, domain.ErrDeviceNotFound
		}
		return nil, fmt.Errorf("tracker ingestion: find device: %w", err)
	}
	session := &domain.TrackerSession{
		ID:        uuid.New().String(),
		UserID:    userID,
		DeviceID:  deviceID,
		StartedAt: time.Now().UTC(),
	}
	obslogger.FromContext(ctx, uc.log).InfoContext(ctx, "session started",
		"session_id", session.ID,
		"user_id", userID,
		"device_id", deviceID,
	)
	return session, nil
}

// IngestBatch validates, deduplicates and publishes a batch of location events.
//
// For each event:
//  1. Validate domain fields (business layer).
//  2. Check dedup ledger by (userID, deviceID, eventID).
//  3. If duplicate → add to Duplicated, skip publish.
//  4. If not duplicate → publish to NATS; on PubAck → MarkPublished.
//     A conflict in MarkPublished after a successful PubAck is non-fatal.
//
// ACK is sent to the client only after this function returns without a global
// EVENT_BUS_UNAVAILABLE error; per-event publish failures are in Rejected.
func (uc *TrackerIngestionUseCase) IngestBatch(ctx context.Context, session *domain.TrackerSession, events []domain.LocationEvent) (*BatchResult, error) {
	if len(events) > uc.maxBatch {
		return nil, &ingestionError{code: domain.ErrCodeBatchTooLarge, msg: fmt.Sprintf("batch size %d exceeds limit %d", len(events), uc.maxBatch)}
	}

	result := &BatchResult{
		Accepted:   []string{},
		Duplicated: []string{},
		Rejected:   []domain.RejectedEvent{},
	}

	now := time.Now().UTC()

	for _, ev := range events {
		// 1. Business validation
		if reason := validateLocationEvent(ev); reason != "" {
			result.Rejected = append(result.Rejected, domain.RejectedEvent{EventID: ev.EventID, Reason: reason})
			continue
		}

		// 2. Dedup check
		dup, err := uc.dedup.IsDuplicate(ctx, session.UserID, session.DeviceID, ev.EventID)
		if err != nil {
			return nil, fmt.Errorf("tracker ingestion: dedup check: %w", err)
		}
		if dup {
			result.Duplicated = append(result.Duplicated, ev.EventID)
			continue
		}

		// 3. Build and publish event
		actType := ""
		if ev.ActivityType != nil {
			actType = string(*ev.ActivityType)
		}
		payload := liblocation.Payload{
			UserID:     session.UserID.String(),
			DeviceID:   session.DeviceID.String(),
			RecordedAt: ev.RecordedAt,
			ReceivedAt: now,
			Lat:        ev.Lat,
			Lon:        ev.Lon,
			AccuracyM:  ev.AccuracyM,
			Source:     liblocation.SourceAndroidTracker,
		}
		if ev.SpeedMps != nil {
			payload.SpeedMps = *ev.SpeedMps
		}
		if ev.BearingDeg != nil {
			payload.BearingDeg = *ev.BearingDeg
		}
		if actType != "" {
			payload.ActivityType = actType
		}
		if ev.ActivityConfidence != nil {
			payload.ActivityConfidence = *ev.ActivityConfidence
		}
		if ev.BatteryLevel != nil {
			payload.BatteryLevel = *ev.BatteryLevel
		}

		envelope, err := liblocation.NewEvent(ev.EventID, now, uc.producer, session.ID, payload)
		if err != nil {
			result.Rejected = append(result.Rejected, domain.RejectedEvent{EventID: ev.EventID, Reason: err.Error()})
			continue
		}

		if err := uc.pub.Publish(ctx, "location.events.v1", envelope); err != nil {
			if errors.Is(err, eventbus.ErrPublish) {
				result.Rejected = append(result.Rejected, domain.RejectedEvent{
					EventID: ev.EventID,
					Reason:  string(domain.ErrCodeEventBusUnavailable),
				})
				continue
			}
			return nil, &ingestionError{code: domain.ErrCodeEventBusUnavailable, msg: err.Error()}
		}

		// 4. Mark published — conflict is non-fatal (publish already confirmed)
		if err := uc.dedup.MarkPublished(ctx, session.UserID, session.DeviceID, ev.EventID); err != nil {
			// ledger insert failure is non-fatal: JetStream dedup window absorbs the retry
			uc.log.WarnContext(ctx, "mark published failed (non-fatal)",
				"event_id", ev.EventID, "err", err)
		}

		result.Accepted = append(result.Accepted, ev.EventID)
	}

	obslogger.FromContext(ctx, uc.log).InfoContext(ctx, "batch processed",
		"user_id", session.UserID,
		"device_id", session.DeviceID,
		"session_id", session.ID,
		"batch_size", len(events),
		"accepted", len(result.Accepted),
		"duplicated", len(result.Duplicated),
		"rejected", len(result.Rejected),
	)
	return result, nil
}

// validateLocationEvent validates domain-level fields of a single event.
// Returns an empty string on success or a human-readable reason string.
func validateLocationEvent(ev domain.LocationEvent) string {
	if ev.EventID == "" {
		return "event_id is required"
	}
	if ev.RecordedAt.IsZero() {
		return "recorded_at is required"
	}
	if ev.Lat < -90 || ev.Lat > 90 {
		return "invalid_latitude"
	}
	if ev.Lon < -180 || ev.Lon > 180 {
		return "invalid_longitude"
	}
	if ev.AccuracyM < 0 {
		return "invalid_accuracy"
	}
	if ev.SpeedMps != nil && *ev.SpeedMps < 0 {
		return "invalid_speed"
	}
	if ev.BearingDeg != nil && (*ev.BearingDeg < 0 || *ev.BearingDeg > 360) {
		return "invalid_bearing"
	}
	if ev.ActivityType != nil {
		if _, ok := domain.ValidActivityTypes[*ev.ActivityType]; !ok {
			return "invalid_activity_type"
		}
	}
	if ev.ActivityConfidence != nil && (*ev.ActivityConfidence < 0 || *ev.ActivityConfidence > 1) {
		return "invalid_activity_confidence"
	}
	if ev.BatteryLevel != nil && (*ev.BatteryLevel < 0 || *ev.BatteryLevel > 1) {
		return "invalid_battery_level"
	}
	return ""
}

// ingestionError is a usecase-layer error that carries a protocol error code.
type ingestionError struct {
	code domain.IngestionErrorCode
	msg  string
}

func (e *ingestionError) Error() string {
	return fmt.Sprintf("%s: %s", e.code, e.msg)
}

// IngestionErrCode extracts the IngestionErrorCode from an error, if present.
func IngestionErrCode(err error) (domain.IngestionErrorCode, bool) {
	var e *ingestionError
	if errors.As(err, &e) {
		return e.code, true
	}
	return "", false
}
