package domain

import "time"

// TrackingPoint is a single raw GPS point read from raw_location_points.
type TrackingPoint struct {
	EventID      string
	DeviceID     string
	RecordedAt   time.Time
	Lat          float64
	Lon          float64
	AccuracyM    float64
	SpeedMps     *float64
	BearingDeg   *float64
	ActivityType string
}

// TrackingPointFilter holds query parameters for reading points.
// UserID must always be set from the JWT — never from a query param.
type TrackingPointFilter struct {
	UserID   string
	DeviceID string // optional; empty means all devices
	From     time.Time
	To       time.Time
	Limit    int    // default 1000, max 5000; enforced in usecase
	Cursor   string // opaque base64 cursor; empty means first page
}

// TrackingSummary holds aggregated stats for a time range.
type TrackingSummary struct {
	PointsCount     int
	FirstRecordedAt *time.Time
	LastRecordedAt  *time.Time
	DurationSec     int64
	AvgSpeedMps     *float64
	MaxSpeedMps     *float64
}

// TrackFilter holds query parameters for the simplified track endpoint.
type TrackFilter struct {
	UserID            string
	DeviceID          string
	From              time.Time
	To                time.Time
	SpeedThresholdMps float64 // points below this speed are "stationary" (default 2.0)
	MinStaySec        int     // stay segments shorter than this are dropped (default 60)
	MinMoveSec        int     // move segments shorter than this are reclassified as stationary (default 30)
}

type TrackSegmentKind string

const (
	TrackSegmentMove TrackSegmentKind = "move"
	TrackSegmentStay TrackSegmentKind = "stay"
)

// TrackSegment is one element of a simplified track:
// a single GPS point for move segments, or a centroid for stationary clusters.
type TrackSegment struct {
	Kind            TrackSegmentKind
	SegmentID       int
	EventID         string // empty for stay segments
	RecordedAt      time.Time
	PeriodEnd       time.Time
	Lat             float64
	Lon             float64
	SpeedMps        *float64
	AccuracyM       *float64
	StayDurationSec int    // seconds; 0 for move segments
	MergedCount     int    // raw points merged; 1 for move segments
	ActivityType    string // move: the point's activity; stay: modal activity
}

type FastSegmentPreset string

const (
	FastSegmentPresetSoft   FastSegmentPreset = "soft"
	FastSegmentPresetNormal FastSegmentPreset = "normal"
	FastSegmentPresetStrict FastSegmentPreset = "strict"
)

// FastSegmentSourcePoint is an accepted processed point used for ranking.
type FastSegmentSourcePoint struct {
	DeviceID   string
	EventID    string
	RecordedAt time.Time
	Lat        float64
	Lon        float64
	SegmentID  int
}

// FastSegmentFilter selects processed points for fastest-section analysis.
type FastSegmentFilter struct {
	UserID   string
	DeviceID string
	From     time.Time
	To       time.Time
}

type FastSegmentPoint struct {
	Lat        float64
	Lon        float64
	RecordedAt time.Time
}

type FastSegment struct {
	Rank             int
	DeviceID         string
	StartedAt        time.Time
	EndedAt          time.Time
	DurationSec      int64
	DistanceM        float64
	AvgSpeedMps      float64
	BaselineSpeedMps float64
	UpliftPercent    float64
	Points           []FastSegmentPoint
}
