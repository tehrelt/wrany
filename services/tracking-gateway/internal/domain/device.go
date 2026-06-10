package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrDeviceNotFound = errors.New("device not found")

type Device struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	DeviceID   uuid.UUID
	Name       *string
	Platform   *string
	LastSeenAt time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
