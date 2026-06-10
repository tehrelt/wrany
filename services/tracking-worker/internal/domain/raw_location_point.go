package domain

import (
	"time"

	"github.com/google/uuid"
)

// RawLocationPoint is a single GPS point as received from the tracker client
// and stored verbatim in raw_location_points.
//
// Geom encoding: the Postgres column uses ST_SetSRID(ST_MakePoint(Lon, Lat), 4326)
// where Lon is X and Lat is Y. Always preserve this order; swapping produces a
// mirrored point.
//
// TripDetectionEngine (future) must query these rows ordered by
// (UserID, DeviceID, RecordedAt) and handle late/out-of-order delivery.
type RawLocationPoint struct {
	UserID   uuid.UUID
	DeviceID uuid.UUID
	EventID  string

	RecordedAt time.Time
	ReceivedAt time.Time

	Lat float64
	Lon float64

	AccuracyM  float64
	SpeedMps   *float64 // nil when not provided by the source
	BearingDeg *float64 // nil when not provided by the source

	ActivityType       string
	ActivityConfidence *float64 // nil when not provided
	BatteryLevel       *float64 // nil when not provided

	Source string
}
