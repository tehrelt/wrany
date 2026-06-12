package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/wrany/tracking-gateway/internal/config"
	"github.com/wrany/tracking-gateway/internal/migrations"
	"github.com/wrany/tracking-gateway/internal/observ"
	"github.com/wrany/tracking-gateway/internal/storage/postgres"
	httptransport "github.com/wrany/tracking-gateway/internal/transport/http"
	"github.com/wrany/tracking-gateway/internal/usecase"
)

// migrationsDir returns absolute path to infra/migrations relative to this test file.
func migrationsDir() string {
	_, file, _, _ := runtime.Caller(0)
	// file is .../internal/transport/http/integration_test.go
	// migrations are at .../infra/migrations
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "infra", "migrations")
}

func setupTestServer(t *testing.T) (*httptest.Server, func()) {
	t.Helper()
	ctx := context.Background()

	ctr, err := tcpostgres.Run(ctx,
		"postgis/postgis:16-3.4",
		tcpostgres.WithDatabase("wrany_test"),
		tcpostgres.WithUsername("wrany"),
		tcpostgres.WithPassword("wrany"),
		// postgis image restarts postgres after running init scripts;
		// wait for the second "ready" to ensure the custom DB is fully initialized.
		testcontainers.CustomizeRequest(testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				WaitingFor: wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).
					WithStartupTimeout(120 * time.Second),
			},
		}),
	)
	require.NoError(t, err)

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	// Use 127.0.0.1 explicitly to avoid Windows IPv6 firewall blocks.
	dsn = strings.ReplaceAll(dsn, "localhost", "127.0.0.1")

	require.NoError(t, migrations.Run(dsn, migrationsDir()))

	db, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)

	cfg := config.Config{
		JWTSecret:     "integration-test-secret-32bytes!!",
		JWTAccessTTL:  15 * time.Minute,
		JWTRefreshTTL: 168 * time.Hour,
	}

	userRepo := postgres.NewUserRepo(db)
	deviceRepo := postgres.NewDeviceRepo(db)
	tokenRepo := postgres.NewTokenRepo(db)

	authUC := usecase.NewAuthUsecase(userRepo, tokenRepo, usecase.AuthConfig{
		JWTSecret:  []byte(cfg.JWTSecret),
		AccessTTL:  cfg.JWTAccessTTL,
		RefreshTTL: cfg.JWTRefreshTTL,
	})

	router := httptransport.NewRouter(httptransport.RouterDeps{
		Auth:      authUC,
		Device:    usecase.NewDeviceUsecase(deviceRepo),
		Me:        usecase.NewMeUsecase(userRepo),
		JWTSecret: []byte(cfg.JWTSecret),
		Metrics:   observ.NewGatewayMetrics(),
	})

	srv := httptest.NewServer(router)
	cleanup := func() {
		srv.Close()
		db.Close()
		_ = ctr.Terminate(ctx)
	}
	return srv, cleanup
}

func postJSON(t *testing.T, url string, body any) *http.Response {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	require.NoError(t, err)
	return resp
}

func getWithBearer(t *testing.T, url, token string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func decodeBody(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	_ = resp.Body.Close()
	return out
}

// --- tests ---

func TestIntegration_Register(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	resp := postJSON(t, srv.URL+"/v1/auth/register", map[string]any{
		"email": "alice@example.com", "password": "securepass",
	})
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	body := decodeBody(t, resp)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	assert.NotEmpty(t, data["access_token"])
	assert.NotEmpty(t, data["refresh_token"])
}

func TestIntegration_Register_DuplicateEmail(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	payload := map[string]any{"email": "bob@example.com", "password": "securepass"}
	postJSON(t, srv.URL+"/v1/auth/register", payload)

	resp := postJSON(t, srv.URL+"/v1/auth/register", payload)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)

	body := decodeBody(t, resp)
	assert.Equal(t, "unable to register with provided credentials", body["error"])
}

func TestIntegration_Login(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	postJSON(t, srv.URL+"/v1/auth/register", map[string]any{"email": "charlie@example.com", "password": "pass1234"})

	resp := postJSON(t, srv.URL+"/v1/auth/login", map[string]any{"email": "charlie@example.com", "password": "pass1234"})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeBody(t, resp)
	assert.NotEmpty(t, body["data"].(map[string]any)["access_token"])
}

func TestIntegration_Login_WrongPassword(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	postJSON(t, srv.URL+"/v1/auth/register", map[string]any{"email": "dana@example.com", "password": "pass1234"})
	resp := postJSON(t, srv.URL+"/v1/auth/login", map[string]any{"email": "dana@example.com", "password": "wrong"})
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	body := decodeBody(t, resp)
	assert.Equal(t, "invalid credentials", body["error"])
}

func TestIntegration_Me_Unauthenticated(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	resp, err := http.Get(srv.URL + "/v1/me")
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestIntegration_Me_Authenticated(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	regResp := postJSON(t, srv.URL+"/v1/auth/register", map[string]any{"email": "eve@example.com", "password": "pass1234"})
	regBody := decodeBody(t, regResp)
	token := regBody["data"].(map[string]any)["access_token"].(string)

	resp := getWithBearer(t, srv.URL+"/v1/me", token)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeBody(t, resp)
	data := body["data"].(map[string]any)
	assert.Equal(t, "eve@example.com", data["email"])
}

func TestIntegration_Refresh(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	regResp := postJSON(t, srv.URL+"/v1/auth/register", map[string]any{"email": "frank@example.com", "password": "pass1234"})
	regBody := decodeBody(t, regResp)
	refreshToken := regBody["data"].(map[string]any)["refresh_token"].(string)

	resp := postJSON(t, srv.URL+"/v1/auth/refresh", map[string]any{"refresh_token": refreshToken})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeBody(t, resp)
	newToken := body["data"].(map[string]any)["refresh_token"].(string)
	assert.NotEqual(t, refreshToken, newToken)
}

func TestIntegration_Refresh_RevokedToken(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	regResp := postJSON(t, srv.URL+"/v1/auth/register", map[string]any{"email": "grace@example.com", "password": "pass1234"})
	regBody := decodeBody(t, regResp)
	refreshToken := regBody["data"].(map[string]any)["refresh_token"].(string)

	// first refresh — revokes original token
	postJSON(t, srv.URL+"/v1/auth/refresh", map[string]any{"refresh_token": refreshToken})

	// second refresh with same token — must fail
	resp := postJSON(t, srv.URL+"/v1/auth/refresh", map[string]any{"refresh_token": refreshToken})
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestIntegration_Device_RegisterAndList(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	regResp := postJSON(t, srv.URL+"/v1/auth/register", map[string]any{"email": "ivan@example.com", "password": "pass1234"})
	regBody := decodeBody(t, regResp)
	token := regBody["data"].(map[string]any)["access_token"].(string)

	deviceID := "550e8400-e29b-41d4-a716-446655440000"
	devReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/devices/register",
		bytes.NewReader(mustMarshal(map[string]any{
			"device_id": deviceID, "name": "Pixel 7", "platform": "android",
		})))
	devReq.Header.Set("Authorization", "Bearer "+token)
	devReq.Header.Set("Content-Type", "application/json")
	devResp, _ := http.DefaultClient.Do(devReq)
	assert.Equal(t, http.StatusCreated, devResp.StatusCode)

	listResp := getWithBearer(t, srv.URL+"/v1/devices", token)
	assert.Equal(t, http.StatusOK, listResp.StatusCode)
	listBody := decodeBody(t, listResp)
	devices := listBody["data"].([]any)
	assert.Len(t, devices, 1)
	assert.Equal(t, deviceID, devices[0].(map[string]any)["DeviceID"])
}

func TestIntegration_Device_Upsert(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	regResp := postJSON(t, srv.URL+"/v1/auth/register", map[string]any{"email": "judy@example.com", "password": "pass1234"})
	regBody := decodeBody(t, regResp)
	token := regBody["data"].(map[string]any)["access_token"].(string)

	deviceID := fmt.Sprintf("%s", "660e8400-e29b-41d4-a716-446655440001")

	registerDevice := func() *http.Response {
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/devices/register",
			bytes.NewReader(mustMarshal(map[string]any{"device_id": deviceID})))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		r, _ := http.DefaultClient.Do(req)
		return r
	}

	registerDevice()
	registerDevice() // second registration = upsert

	listResp := getWithBearer(t, srv.URL+"/v1/devices", token)
	listBody := decodeBody(t, listResp)
	devices := listBody["data"].([]any)
	assert.Len(t, devices, 1, "upsert should not create duplicates")
}

func mustMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
