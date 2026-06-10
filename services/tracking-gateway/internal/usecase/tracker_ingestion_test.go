package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wrany/libs/eventbus"
	libevents "github.com/wrany/libs/events"
	"github.com/wrany/tracking-gateway/internal/domain"
	"github.com/wrany/tracking-gateway/internal/usecase"
)

// --- mock DeviceLookup ---

type mockDeviceLookup struct {
	devices map[string]*domain.Device
}

func newMockDeviceLookup() *mockDeviceLookup {
	return &mockDeviceLookup{devices: make(map[string]*domain.Device)}
}

func (m *mockDeviceLookup) add(userID, deviceID uuid.UUID) {
	key := userID.String() + ":" + deviceID.String()
	m.devices[key] = &domain.Device{UserID: userID, DeviceID: deviceID}
}

func (m *mockDeviceLookup) FindByUserAndDeviceID(_ context.Context, userID, deviceID uuid.UUID) (*domain.Device, error) {
	key := userID.String() + ":" + deviceID.String()
	if d, ok := m.devices[key]; ok {
		return d, nil
	}
	return nil, domain.ErrDeviceNotFound
}

// --- mock IngestionDedupRepo ---

type mockDedupRepo struct {
	published map[string]struct{}
}

func newMockDedupRepo() *mockDedupRepo {
	return &mockDedupRepo{published: make(map[string]struct{})}
}

func (m *mockDedupRepo) key(userID, deviceID uuid.UUID, eventID string) string {
	return userID.String() + ":" + deviceID.String() + ":" + eventID
}

func (m *mockDedupRepo) IsDuplicate(_ context.Context, userID, deviceID uuid.UUID, eventID string) (bool, error) {
	_, ok := m.published[m.key(userID, deviceID, eventID)]
	return ok, nil
}

func (m *mockDedupRepo) MarkPublished(_ context.Context, userID, deviceID uuid.UUID, eventID string) error {
	m.published[m.key(userID, deviceID, eventID)] = struct{}{}
	return nil
}

// --- mock Publisher ---

type mockPublisher struct {
	published []libevents.Envelope
	failWith  error
}

func (p *mockPublisher) Publish(_ context.Context, _ string, env libevents.Envelope) error {
	if p.failWith != nil {
		return p.failWith
	}
	p.published = append(p.published, env)
	return nil
}

// --- helpers ---

func ptr[T any](v T) *T { return &v }

func makeUC(devices *mockDeviceLookup, dedup usecase.IngestionDedupRepo, pub *mockPublisher) *usecase.TrackerIngestionUseCase {
	return usecase.NewTrackerIngestionUseCase(devices, dedup, pub, "test-gateway", 100)
}

func validEvent(id string) domain.LocationEvent {
	return domain.LocationEvent{
		EventID:    id,
		RecordedAt: time.Now().UTC(),
		Lat:        55.75,
		Lon:        37.62,
		AccuracyM:  10.0,
	}
}

// --- StartSession tests ---

func TestStartSession_DeviceNotRegistered(t *testing.T) {
	uc := makeUC(newMockDeviceLookup(), newMockDedupRepo(), &mockPublisher{})
	_, err := uc.StartSession(context.Background(), uuid.New(), uuid.New())
	assert.ErrorIs(t, err, domain.ErrDeviceNotFound)
}

func TestStartSession_OK(t *testing.T) {
	devices := newMockDeviceLookup()
	userID := uuid.New()
	deviceID := uuid.New()
	devices.add(userID, deviceID)

	uc := makeUC(devices, newMockDedupRepo(), &mockPublisher{})
	session, err := uc.StartSession(context.Background(), userID, deviceID)
	require.NoError(t, err)
	assert.NotEmpty(t, session.ID)
	assert.Equal(t, userID, session.UserID)
	assert.Equal(t, deviceID, session.DeviceID)
}

// --- IngestBatch validation tests ---

func newSession(userID, deviceID uuid.UUID) *domain.TrackerSession {
	return &domain.TrackerSession{
		ID:        uuid.New().String(),
		UserID:    userID,
		DeviceID:  deviceID,
		StartedAt: time.Now().UTC(),
	}
}

func TestIngestBatch_BatchTooLarge(t *testing.T) {
	uc := makeUC(newMockDeviceLookup(), newMockDedupRepo(), &mockPublisher{})
	session := newSession(uuid.New(), uuid.New())

	events := make([]domain.LocationEvent, 101)
	for i := range events {
		events[i] = validEvent(uuid.New().String())
	}
	_, err := uc.IngestBatch(context.Background(), session, events)
	require.Error(t, err)
	code, ok := usecase.IngestionErrCode(err)
	assert.True(t, ok)
	assert.Equal(t, domain.ErrCodeBatchTooLarge, code)
}

func TestIngestBatch_InvalidLat(t *testing.T) {
	pub := &mockPublisher{}
	uc := makeUC(newMockDeviceLookup(), newMockDedupRepo(), pub)
	session := newSession(uuid.New(), uuid.New())

	ev := validEvent("evt-1")
	ev.Lat = 91.0

	result, err := uc.IngestBatch(context.Background(), session, []domain.LocationEvent{ev})
	require.NoError(t, err)
	assert.Len(t, result.Rejected, 1)
	assert.Equal(t, "evt-1", result.Rejected[0].EventID)
	assert.Equal(t, "invalid_latitude", result.Rejected[0].Reason)
	assert.Empty(t, pub.published)
}

func TestIngestBatch_InvalidLon(t *testing.T) {
	uc := makeUC(newMockDeviceLookup(), newMockDedupRepo(), &mockPublisher{})
	session := newSession(uuid.New(), uuid.New())

	ev := validEvent("evt-1")
	ev.Lon = -181.0

	result, err := uc.IngestBatch(context.Background(), session, []domain.LocationEvent{ev})
	require.NoError(t, err)
	assert.Equal(t, "invalid_longitude", result.Rejected[0].Reason)
}

func TestIngestBatch_InvalidAccuracy(t *testing.T) {
	uc := makeUC(newMockDeviceLookup(), newMockDedupRepo(), &mockPublisher{})
	session := newSession(uuid.New(), uuid.New())

	ev := validEvent("evt-1")
	ev.AccuracyM = -1.0

	result, err := uc.IngestBatch(context.Background(), session, []domain.LocationEvent{ev})
	require.NoError(t, err)
	assert.Equal(t, "invalid_accuracy", result.Rejected[0].Reason)
}

func TestIngestBatch_InvalidActivityType(t *testing.T) {
	uc := makeUC(newMockDeviceLookup(), newMockDedupRepo(), &mockPublisher{})
	session := newSession(uuid.New(), uuid.New())

	ev := validEvent("evt-1")
	at := domain.ActivityType("flying")
	ev.ActivityType = &at

	result, err := uc.IngestBatch(context.Background(), session, []domain.LocationEvent{ev})
	require.NoError(t, err)
	assert.Equal(t, "invalid_activity_type", result.Rejected[0].Reason)
}

func TestIngestBatch_OptionalFieldsValidRanges(t *testing.T) {
	pub := &mockPublisher{}
	uc := makeUC(newMockDeviceLookup(), newMockDedupRepo(), pub)
	userID := uuid.New()
	deviceID := uuid.New()
	session := newSession(userID, deviceID)

	ev := validEvent("evt-1")
	ev.SpeedMps = ptr(2.5)
	ev.BearingDeg = ptr(90.0)
	at := domain.ActivityWalking
	ev.ActivityType = &at
	ev.ActivityConfidence = ptr(0.9)
	ev.BatteryLevel = ptr(0.5)

	result, err := uc.IngestBatch(context.Background(), session, []domain.LocationEvent{ev})
	require.NoError(t, err)
	assert.Equal(t, []string{"evt-1"}, result.Accepted)
	assert.Len(t, pub.published, 1)
}

func TestIngestBatch_InvalidSpeed(t *testing.T) {
	uc := makeUC(newMockDeviceLookup(), newMockDedupRepo(), &mockPublisher{})
	session := newSession(uuid.New(), uuid.New())

	ev := validEvent("evt-1")
	ev.SpeedMps = ptr(-1.0)

	result, err := uc.IngestBatch(context.Background(), session, []domain.LocationEvent{ev})
	require.NoError(t, err)
	assert.Equal(t, "invalid_speed", result.Rejected[0].Reason)
}

func TestIngestBatch_InvalidBearing(t *testing.T) {
	uc := makeUC(newMockDeviceLookup(), newMockDedupRepo(), &mockPublisher{})
	session := newSession(uuid.New(), uuid.New())

	ev := validEvent("evt-1")
	ev.BearingDeg = ptr(361.0)

	result, err := uc.IngestBatch(context.Background(), session, []domain.LocationEvent{ev})
	require.NoError(t, err)
	assert.Equal(t, "invalid_bearing", result.Rejected[0].Reason)
}

// --- Dedup tests ---

func TestIngestBatch_DuplicateEventNotPublished(t *testing.T) {
	pub := &mockPublisher{}
	dedup := newMockDedupRepo()
	uc := makeUC(newMockDeviceLookup(), dedup, pub)
	userID := uuid.New()
	deviceID := uuid.New()
	session := newSession(userID, deviceID)

	// Mark as already published.
	_ = dedup.MarkPublished(context.Background(), userID, deviceID, "evt-1")

	result, err := uc.IngestBatch(context.Background(), session, []domain.LocationEvent{validEvent("evt-1")})
	require.NoError(t, err)
	assert.Equal(t, []string{"evt-1"}, result.Duplicated)
	assert.Empty(t, result.Accepted)
	assert.Empty(t, pub.published)
}

func TestIngestBatch_SameEventIDDifferentDeviceIsNotDuplicate(t *testing.T) {
	pub := &mockPublisher{}
	dedup := newMockDedupRepo()
	userID := uuid.New()
	device1 := uuid.New()
	device2 := uuid.New()

	// device1 has published "evt-1".
	_ = dedup.MarkPublished(context.Background(), userID, device1, "evt-1")

	// device2 sends same event_id — should be accepted.
	uc := makeUC(newMockDeviceLookup(), dedup, pub)
	session := newSession(userID, device2)

	result, err := uc.IngestBatch(context.Background(), session, []domain.LocationEvent{validEvent("evt-1")})
	require.NoError(t, err)
	assert.Equal(t, []string{"evt-1"}, result.Accepted)
	assert.Empty(t, result.Duplicated)
}

func TestIngestBatch_AcceptedRejectedDuplicatedSplit(t *testing.T) {
	pub := &mockPublisher{}
	dedup := newMockDedupRepo()
	uc := makeUC(newMockDeviceLookup(), dedup, pub)
	userID := uuid.New()
	deviceID := uuid.New()
	session := newSession(userID, deviceID)

	_ = dedup.MarkPublished(context.Background(), userID, deviceID, "dup-1")

	invalid := validEvent("bad-1")
	invalid.Lat = 999

	events := []domain.LocationEvent{
		validEvent("ok-1"),
		{EventID: "dup-1", RecordedAt: time.Now(), Lat: 55, Lon: 37, AccuracyM: 5},
		invalid,
	}

	result, err := uc.IngestBatch(context.Background(), session, events)
	require.NoError(t, err)
	assert.Equal(t, []string{"ok-1"}, result.Accepted)
	assert.Equal(t, []string{"dup-1"}, result.Duplicated)
	assert.Equal(t, "bad-1", result.Rejected[0].EventID)
}

// --- Publisher failure ---

func TestIngestBatch_PublisherFailure_EventBusUnavailable(t *testing.T) {
	pub := &mockPublisher{failWith: fmt.Errorf("%w: nats down", eventbus.ErrPublish)}
	uc := makeUC(newMockDeviceLookup(), newMockDedupRepo(), pub)
	session := newSession(uuid.New(), uuid.New())

	result, err := uc.IngestBatch(context.Background(), session, []domain.LocationEvent{validEvent("evt-1")})
	require.NoError(t, err)
	assert.Empty(t, result.Accepted)
	assert.Equal(t, "evt-1", result.Rejected[0].EventID)
	assert.Equal(t, string(domain.ErrCodeEventBusUnavailable), result.Rejected[0].Reason)
}

func TestIngestBatch_MarkPublishedConflictIsNonFatal(t *testing.T) {
	pub := &mockPublisher{}
	dedup := &conflictDedupRepo{}
	uc := makeUC(newMockDeviceLookup(), dedup, pub)
	session := newSession(uuid.New(), uuid.New())

	result, err := uc.IngestBatch(context.Background(), session, []domain.LocationEvent{validEvent("evt-1")})
	require.NoError(t, err)
	// Even though MarkPublished returned a conflict error, the event is still accepted.
	assert.Equal(t, []string{"evt-1"}, result.Accepted)
}

// conflictDedupRepo simulates a conflict on MarkPublished (e.g. concurrent insert).
type conflictDedupRepo struct{}

func (c *conflictDedupRepo) IsDuplicate(_ context.Context, _, _ uuid.UUID, _ string) (bool, error) {
	return false, nil
}
func (c *conflictDedupRepo) MarkPublished(_ context.Context, _, _ uuid.UUID, _ string) error {
	return errors.New("pq: duplicate key value violates unique constraint")
}
