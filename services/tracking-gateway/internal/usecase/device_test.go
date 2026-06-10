package usecase_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wrany/tracking-gateway/internal/domain"
	"github.com/wrany/tracking-gateway/internal/usecase"
)

// --- mock DeviceRepository ---

type mockDeviceRepo struct {
	// keyed by "userID:deviceID"
	devices map[string]*domain.Device
}

func newMockDeviceRepo() *mockDeviceRepo {
	return &mockDeviceRepo{devices: make(map[string]*domain.Device)}
}

func (m *mockDeviceRepo) key(userID, deviceID uuid.UUID) string {
	return userID.String() + ":" + deviceID.String()
}

func (m *mockDeviceRepo) Upsert(_ context.Context, d *domain.Device) error {
	k := m.key(d.UserID, d.DeviceID)
	if existing, ok := m.devices[k]; ok {
		existing.LastSeenAt = d.LastSeenAt
		existing.UpdatedAt = d.UpdatedAt
		if d.Name != nil {
			existing.Name = d.Name
		}
		if d.Platform != nil {
			existing.Platform = d.Platform
		}
		return nil
	}
	m.devices[k] = d
	return nil
}

func (m *mockDeviceRepo) ListByUserID(_ context.Context, userID uuid.UUID) ([]*domain.Device, error) {
	var result []*domain.Device
	for _, d := range m.devices {
		if d.UserID == userID {
			result = append(result, d)
		}
	}
	return result, nil
}

// --- tests ---

func TestDeviceUsecase_RegisterDevice_Success(t *testing.T) {
	repo := newMockDeviceRepo()
	uc := usecase.NewDeviceUsecase(repo)

	userID := uuid.New()
	deviceID := uuid.New()
	name := "Pixel 7"
	platform := "android"

	device, err := uc.RegisterDevice(context.Background(), usecase.RegisterDeviceInput{
		UserID:   userID,
		DeviceID: deviceID,
		Name:     &name,
		Platform: &platform,
	})

	require.NoError(t, err)
	assert.Equal(t, userID, device.UserID)
	assert.Equal(t, deviceID, device.DeviceID)
	assert.Equal(t, &name, device.Name)
}

func TestDeviceUsecase_RegisterDevice_Upsert(t *testing.T) {
	repo := newMockDeviceRepo()
	uc := usecase.NewDeviceUsecase(repo)

	userID := uuid.New()
	deviceID := uuid.New()

	d1, err := uc.RegisterDevice(context.Background(), usecase.RegisterDeviceInput{
		UserID: userID, DeviceID: deviceID,
	})
	require.NoError(t, err)
	firstSeen := d1.LastSeenAt

	newName := "updated-name"
	d2, err := uc.RegisterDevice(context.Background(), usecase.RegisterDeviceInput{
		UserID: userID, DeviceID: deviceID, Name: &newName,
	})
	require.NoError(t, err)

	// Only one record must exist
	devices, _ := uc.ListDevices(context.Background(), userID)
	assert.Len(t, devices, 1, "upsert should not create duplicate")

	assert.True(t, d2.LastSeenAt.Equal(firstSeen) || d2.LastSeenAt.After(firstSeen))
}

func TestDeviceUsecase_ListDevices_OnlyCurrentUser(t *testing.T) {
	repo := newMockDeviceRepo()
	uc := usecase.NewDeviceUsecase(repo)

	userA := uuid.New()
	userB := uuid.New()

	_, _ = uc.RegisterDevice(context.Background(), usecase.RegisterDeviceInput{
		UserID: userA, DeviceID: uuid.New(),
	})
	_, _ = uc.RegisterDevice(context.Background(), usecase.RegisterDeviceInput{
		UserID: userB, DeviceID: uuid.New(),
	})
	_, _ = uc.RegisterDevice(context.Background(), usecase.RegisterDeviceInput{
		UserID: userA, DeviceID: uuid.New(),
	})

	devices, err := uc.ListDevices(context.Background(), userA)
	require.NoError(t, err)
	assert.Len(t, devices, 2)
	for _, d := range devices {
		assert.Equal(t, userA, d.UserID)
	}
}

func TestDeviceUsecase_ListDevices_EmptyList(t *testing.T) {
	uc := usecase.NewDeviceUsecase(newMockDeviceRepo())

	devices, err := uc.ListDevices(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.NotNil(t, devices)
	assert.Empty(t, devices)
}
