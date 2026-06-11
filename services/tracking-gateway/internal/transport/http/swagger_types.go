package http

// swagger request / response types.
// Request types are used directly by handlers; response envelope types
// are referenced only in godoc annotations and declared below.

// blank declarations prevent "unused type" linter warnings for doc-only types.
var (
	_ = swErr{}
	_ = swTokenPairEnv{}
	_ = swDeviceEnv{}
	_ = swDevicesEnv{}
	_ = swMeEnv{}
	_ = swHealthzEnv{}
	_ = swTrackingPointsEnv{}
	_ = swTrackingSummaryEnv{}
)

// swRegisterReq is the body for POST /v1/auth/register.
type swRegisterReq struct {
	Email    string `json:"email"    example:"rider@example.com"`
	Password string `json:"password" example:"hunter2"`
}

// swLoginReq is the body for POST /v1/auth/login.
type swLoginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// swRefreshReq is the body for POST /v1/auth/refresh.
type swRefreshReq struct {
	RefreshToken string `json:"refresh_token"`
}

// swRegisterDeviceReq is the body for POST /v1/devices/register.
type swRegisterDeviceReq struct {
	DeviceID string  `json:"device_id" example:"b0d34c19-ef5e-4e35-bd30-1d6680245c10"`
	Name     *string `json:"name"      example:"Pixel 8"`
	Platform *string `json:"platform"  example:"android"`
}

// swTokenPair is the data field in auth responses.
type swTokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// swDevice is the data field in device responses.
type swDevice struct {
	ID         string  `json:"id"`
	DeviceID   string  `json:"device_id"`
	Name       *string `json:"name"`
	Platform   *string `json:"platform"`
	LastSeenAt string  `json:"last_seen_at"`
	CreatedAt  string  `json:"created_at"`
}

// swMe is the data field in GET /v1/me response.
type swMe struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
}

// swHealthz is the response for GET /healthz.
type swHealthz struct {
	Status string `json:"status" example:"ok"`
}

// swErr is the error envelope response.
type swErr struct {
	Data  *string `json:"data"`
	Error string  `json:"error" example:"unauthorized"`
}

// Typed envelope wrappers for swagger response docs.

type swTokenPairEnv struct {
	Data  swTokenPair `json:"data"`
	Error *string     `json:"error"`
}

type swDeviceEnv struct {
	Data  swDevice `json:"data"`
	Error *string  `json:"error"`
}

type swDevicesEnv struct {
	Data  []swDevice `json:"data"`
	Error *string    `json:"error"`
}

type swMeEnv struct {
	Data  swMe    `json:"data"`
	Error *string `json:"error"`
}

type swHealthzEnv struct {
	Data  swHealthz `json:"data"`
	Error *string   `json:"error"`
}

// swTrackingPoint is a single raw GPS point in the points list response.
type swTrackingPoint struct {
	EventID      string   `json:"event_id"`
	DeviceID     string   `json:"device_id"`
	RecordedAt   string   `json:"recorded_at"   example:"2026-06-10T12:00:01Z"`
	Lat          float64  `json:"lat"           example:"55.751244"`
	Lon          float64  `json:"lon"           example:"37.618423"`
	AccuracyM    float64  `json:"accuracy_m"    example:"8.5"`
	SpeedMps     *float64 `json:"speed_mps"`
	BearingDeg   *float64 `json:"bearing_deg"`
	ActivityType string   `json:"activity_type" example:"walking"`
}

// swTrackingPointsResponse is the data payload for GET /v1/tracking/points.
type swTrackingPointsResponse struct {
	Items      []swTrackingPoint `json:"items"`
	NextCursor *string           `json:"next_cursor"`
}

// swTrackingSummary is the data payload for GET /v1/tracking/summary.
type swTrackingSummary struct {
	PointsCount     int      `json:"points_count"      example:"123"`
	FirstRecordedAt *string  `json:"first_recorded_at" example:"2026-06-10T12:00:01Z"`
	LastRecordedAt  *string  `json:"last_recorded_at"  example:"2026-06-10T12:30:01Z"`
	DurationSec     int64    `json:"duration_sec"      example:"1800"`
	AvgSpeedMps     *float64 `json:"avg_speed_mps"`
	MaxSpeedMps     *float64 `json:"max_speed_mps"`
}

type swTrackingPointsEnv struct {
	Data  swTrackingPointsResponse `json:"data"`
	Error *string                  `json:"error"`
}

type swTrackingSummaryEnv struct {
	Data  swTrackingSummary `json:"data"`
	Error *string           `json:"error"`
}
