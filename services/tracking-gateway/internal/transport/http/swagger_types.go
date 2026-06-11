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
