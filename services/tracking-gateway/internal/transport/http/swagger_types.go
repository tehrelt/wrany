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

// TrackSegmentItem is one element of a simplified track from GET /v1/tracking/track.
// kind="move": a single GPS point; kind="stay": a centroid of a stationary cluster.
type TrackSegmentItem struct {
	Kind            string   `json:"kind"              example:"stay"`
	EventID         string   `json:"event_id"`
	RecordedAt      string   `json:"recorded_at"       example:"2026-06-10T12:00:01Z"`
	PeriodEnd       string   `json:"period_end"        example:"2026-06-10T14:30:00Z"`
	Lat             float64  `json:"lat"               example:"55.751244"`
	Lon             float64  `json:"lon"               example:"37.618423"`
	SpeedMps        *float64 `json:"speed_mps"`
	AccuracyM       *float64 `json:"accuracy_m"`
	StayDurationSec int      `json:"stay_duration_sec" example:"9000"`
	MergedCount     int      `json:"merged_count"      example:"180"`
}

// TrackResponse is the data payload for GET /v1/tracking/track.
type TrackResponse struct {
	Items []TrackSegmentItem `json:"items"`
}

// TrackEnv is the envelope for GET /v1/tracking/track.
type TrackEnv struct {
	Data  TrackResponse `json:"data"`
	Error *string       `json:"error"`
}

// TripItem is a single trip in list/detail responses.
type TripItem struct {
	ID          string   `json:"id"`
	UserID      string   `json:"user_id"`
	DeviceID    string   `json:"device_id"`
	Status      string   `json:"status"       example:"TRIP_COMPLETED"`
	StartedAt   string   `json:"started_at"   example:"2026-06-10T08:00:00Z"`
	EndedAt     *string  `json:"ended_at"`
	StartLat    float64  `json:"start_lat"    example:"55.7558"`
	StartLon    float64  `json:"start_lon"    example:"37.6173"`
	EndLat      *float64 `json:"end_lat"`
	EndLon      *float64 `json:"end_lon"`
	DistanceM   float64  `json:"distance_m"   example:"4200.5"`
	DurationSec int64    `json:"duration_sec" example:"1800"`
	PointsCount int      `json:"points_count" example:"540"`
	CreatedAt   string   `json:"created_at"   example:"2026-06-10T08:00:01Z"`
	UpdatedAt   string   `json:"updated_at"   example:"2026-06-10T08:30:00Z"`
}

// TripListResponse is the data payload for GET /v1/trips.
type TripListResponse struct {
	Items      []TripItem `json:"items"`
	NextCursor *string    `json:"next_cursor"`
}

// TripPointItem is a single GPS point in the trip polyline.
type TripPointItem struct {
	EventID    string  `json:"event_id"`
	TripID     string  `json:"trip_id"`
	RecordedAt string  `json:"recorded_at" example:"2026-06-10T08:01:00Z"`
	Lat        float64 `json:"lat"         example:"55.7558"`
	Lon        float64 `json:"lon"         example:"37.6173"`
}

// TripPointsResponse is the data payload for GET /v1/trips/{id}/points.
type TripPointsResponse struct {
	Items      []TripPointItem `json:"items"`
	NextCursor *string         `json:"next_cursor"`
}

// TripEnv is the envelope for GET /v1/trips/{id}.
type TripEnv struct {
	Data  TripItem `json:"data"`
	Error *string  `json:"error"`
}

// TripListEnv is the envelope for GET /v1/trips.
type TripListEnv struct {
	Data  TripListResponse `json:"data"`
	Error *string          `json:"error"`
}

// TripPointsEnv is the envelope for GET /v1/trips/{id}/points.
type TripPointsEnv struct {
	Data  TripPointsResponse `json:"data"`
	Error *string            `json:"error"`
}


// RouteItem is a single route in list/detail responses.
type RouteItem struct {
	ID         string  `json:"id"`
	UserID     string  `json:"user_id"`
	Name       *string `json:"name"`
	Status     string  `json:"status"`
	StartLat   float64 `json:"start_lat"`
	StartLon   float64 `json:"start_lon"`
	EndLat     float64 `json:"end_lat"`
	EndLon     float64 `json:"end_lon"`
	DistanceM  float64 `json:"distance_m"`
	TripsCount int     `json:"trips_count"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
}

// RouteListResponse is the data payload for GET /v1/routes.
type RouteListResponse struct {
	Items      []RouteItem `json:"items"`
	NextCursor *string     `json:"next_cursor"`
}

// RouteTripItem is a trip attached to a route.
type RouteTripItem struct {
	TripID      string  `json:"trip_id"`
	MatchScore  float64 `json:"match_score"`
	MatchedAt   string  `json:"matched_at"`
	DurationSec int64   `json:"duration_sec"`
	DistanceM   float64 `json:"distance_m"`
	StartedAt   string  `json:"started_at"`
	EndedAt     *string `json:"ended_at"`
}

// RouteTripListResponse is the data payload for GET /v1/routes/{id}/trips.
type RouteTripListResponse struct {
	Items      []RouteTripItem `json:"items"`
	NextCursor *string         `json:"next_cursor"`
}

// RoutePointItem is a single GPS point of the route template polyline.
type RoutePointItem struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// RouteEnv is the envelope for GET /v1/routes/{id}.
type RouteEnv struct {
	Data  RouteItem `json:"data"`
	Error *string   `json:"error"`
}

// RouteListEnv is the envelope for GET /v1/routes.
type RouteListEnv struct {
	Data  RouteListResponse `json:"data"`
	Error *string           `json:"error"`
}

// RouteTripListEnv is the envelope for GET /v1/routes/{id}/trips.
type RouteTripListEnv struct {
	Data  RouteTripListResponse `json:"data"`
	Error *string               `json:"error"`
}

// RoutePointsEnv is the envelope for GET /v1/routes/{id}/points.
type RoutePointsEnv struct {
	Data  []RoutePointItem `json:"data"`
	Error *string          `json:"error"`
}

// TripResultItem holds best or latest trip metrics.
type TripResultItem struct {
	TripID      string  `json:"trip_id"`
	StartedAt   string  `json:"started_at"`
	DurationSec int64   `json:"duration_sec"`
	DistanceM   float64 `json:"distance_m"`
	AvgSpeedMps float64 `json:"avg_speed_mps"`
}

// RouteResultComparisonItem compares the latest trip against the best.
type RouteResultComparisonItem struct {
	LatestVsBestSec     int64   `json:"latest_vs_best_sec"`
	LatestVsBestPercent float64 `json:"latest_vs_best_percent"`
}

// RouteResultResponse is the data payload for GET /v1/routes/{route_id}/results.
type RouteResultResponse struct {
	RouteID       string                     `json:"route_id"`
	AttemptsCount int                        `json:"attempts_count"`
	Best          *TripResultItem            `json:"best"`
	Latest        *TripResultItem            `json:"latest"`
	Comparison    *RouteResultComparisonItem `json:"comparison"`
}

// TripAttemptItem is a single attempt in the attempts list.
type TripAttemptItem struct {
	TripID      string  `json:"trip_id"`
	StartedAt   string  `json:"started_at"`
	EndedAt     *string `json:"ended_at"`
	DurationSec int64   `json:"duration_sec"`
	DistanceM   float64 `json:"distance_m"`
	AvgSpeedMps float64 `json:"avg_speed_mps"`
	MatchScore  float64 `json:"match_score"`
	IsBest      bool    `json:"is_best"`
}

// RouteAttemptListResponse is the data payload for GET /v1/routes/{route_id}/attempts.
type RouteAttemptListResponse struct {
	Items      []TripAttemptItem `json:"items"`
	NextCursor *string           `json:"next_cursor"`
}

// RouteResultEnv is the envelope for GET /v1/routes/{route_id}/results.
type RouteResultEnv struct {
	Data  RouteResultResponse `json:"data"`
	Error *string             `json:"error"`
}

// RouteAttemptListEnv is the envelope for GET /v1/routes/{route_id}/attempts.
type RouteAttemptListEnv struct {
	Data  RouteAttemptListResponse `json:"data"`
	Error *string                  `json:"error"`
}
