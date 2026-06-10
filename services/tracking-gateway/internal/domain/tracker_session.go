package domain

import (
	"time"

	"github.com/google/uuid"
)

// TrackerSession represents an active WebSocket tracking session.
type TrackerSession struct {
	ID        string
	UserID    uuid.UUID
	DeviceID  uuid.UUID
	StartedAt time.Time
}

// SessionConfig is sent to the client on session.accepted.
type SessionConfig struct {
	MaxBatchSize                int
	RecommendedFlushIntervalSec int
}
