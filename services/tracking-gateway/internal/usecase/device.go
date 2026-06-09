package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/wrany/tracking-gateway/internal/domain"
)

type DeviceRepository interface {
	Upsert(ctx context.Context, device *domain.Device) error
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Device, error)
}

type DeviceUsecase struct {
	devices DeviceRepository
}

func NewDeviceUsecase(devices DeviceRepository) *DeviceUsecase {
	return &DeviceUsecase{devices: devices}
}

type RegisterDeviceInput struct {
	UserID   uuid.UUID
	DeviceID uuid.UUID
	Name     *string
	Platform *string
}

func (uc *DeviceUsecase) RegisterDevice(ctx context.Context, in RegisterDeviceInput) (*domain.Device, error) {
	now := time.Now().UTC()
	device := &domain.Device{
		ID:         uuid.New(),
		UserID:     in.UserID,
		DeviceID:   in.DeviceID,
		Name:       in.Name,
		Platform:   in.Platform,
		LastSeenAt: now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := uc.devices.Upsert(ctx, device); err != nil {
		return nil, err
	}
	return device, nil
}

func (uc *DeviceUsecase) ListDevices(ctx context.Context, userID uuid.UUID) ([]*domain.Device, error) {
	devices, err := uc.devices.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if devices == nil {
		return []*domain.Device{}, nil
	}
	return devices, nil
}
