package http

// Swagger request / response types — all exported so swag generates clean schema names.

// RegisterReq is the body for POST /v1/auth/register.
type RegisterReq struct {
	Email    string `json:"email"    example:"rider@example.com"`
	Password string `json:"password" example:"hunter2"`
}

// LoginReq is the body for POST /v1/auth/login.
type LoginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RefreshReq is the body for POST /v1/auth/refresh.
type RefreshReq struct {
	RefreshToken string `json:"refresh_token"`
}

// RegisterDeviceReq is the body for POST /v1/devices/register.
type RegisterDeviceReq struct {
	DeviceID string  `json:"device_id" example:"b0d34c19-ef5e-4e35-bd30-1d6680245c10"`
	Name     *string `json:"name"      example:"Pixel 8"`
	Platform *string `json:"platform"  example:"android"`
}

// TokenPair is the data field in auth responses.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// Device is the data field in device responses.
type Device struct {
	ID         string  `json:"id"`
	DeviceID   string  `json:"device_id"`
	Name       *string `json:"name"`
	Platform   *string `json:"platform"`
	LastSeenAt string  `json:"last_seen_at"`
	CreatedAt  string  `json:"created_at"`
}

// Me is the data field in GET /v1/me response.
type Me struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
}

// Healthz is the response for GET /healthz.
type Healthz struct {
	Status string `json:"status" example:"ok"`
}

// ApiError is the error envelope response.
type ApiError struct {
	Data  *string `json:"data"`
	Error string  `json:"error" example:"unauthorized"`
}

// Typed envelope wrappers.

// TokenPairEnv is the envelope for auth token responses.
type TokenPairEnv struct {
	Data  TokenPair `json:"data"`
	Error *string   `json:"error"`
}

// DeviceEnv is the envelope for single-device responses.
type DeviceEnv struct {
	Data  Device  `json:"data"`
	Error *string `json:"error"`
}

// DevicesEnv is the envelope for device list responses.
type DevicesEnv struct {
	Data  []Device `json:"data"`
	Error *string  `json:"error"`
}

// MeEnv is the envelope for GET /v1/me responses.
type MeEnv struct {
	Data  Me      `json:"data"`
	Error *string `json:"error"`
}

// HealthzEnv is the envelope for GET /healthz responses.
type HealthzEnv struct {
	Data  Healthz `json:"data"`
	Error *string `json:"error"`
}

// TrackingPoint is a single raw GPS point.
type TrackingPoint struct {
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

// TrackingPointsResponse is the data payload for GET /v1/tracking/points.
type TrackingPointsResponse struct {
	Items      []TrackingPoint `json:"items"`
	NextCursor *string         `json:"next_cursor"`
}

// TrackingSummary is the data payload for GET /v1/tracking/summary.
type TrackingSummary struct {
	PointsCount     int      `json:"points_count"      example:"123"`
	FirstRecordedAt *string  `json:"first_recorded_at" example:"2026-06-10T12:00:01Z"`
	LastRecordedAt  *string  `json:"last_recorded_at"  example:"2026-06-10T12:30:01Z"`
	DurationSec     int64    `json:"duration_sec"      example:"1800"`
	AvgSpeedMps     *float64 `json:"avg_speed_mps"`
	MaxSpeedMps     *float64 `json:"max_speed_mps"`
}

// TrackingPointsEnv is the envelope for GET /v1/tracking/points.
type TrackingPointsEnv struct {
	Data  TrackingPointsResponse `json:"data"`
	Error *string                `json:"error"`
}

// TrackingSummaryEnv is the envelope for GET /v1/tracking/summary.
type TrackingSummaryEnv struct {
	Data  TrackingSummary `json:"data"`
	Error *string         `json:"error"`
}
