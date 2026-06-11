package http_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	httptransport "github.com/wrany/tracking-gateway/internal/transport/http"
)

var testJWTSecret = []byte("test-secret-at-least-32-bytes!!!!")

// makeToken generates a signed JWT with the given userID and expiry.
func makeToken(t *testing.T, userID uuid.UUID, ttl time.Duration) string {
	t.Helper()
	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(testJWTSecret)
	require.NoError(t, err)
	return tok
}

// echoUserID is a handler that writes 200 and the injected user ID.
func echoUserID(w http.ResponseWriter, r *http.Request) {
	id, ok := httptransport.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "no user", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(id.String()))
}

func TestAuthMiddleware_Header_Valid(t *testing.T) {
	userID := uuid.New()
	token := makeToken(t, userID, 15*time.Minute)

	mux := http.NewServeMux()
	auth := httptransport.AuthMiddleware(testJWTSecret)
	mux.Handle("GET /protected", auth(http.HandlerFunc(echoUserID)))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, userID.String(), rec.Body.String())
}

func TestAuthMiddleware_MissingToken_Rejected(t *testing.T) {
	mux := http.NewServeMux()
	auth := httptransport.AuthMiddleware(testJWTSecret)
	mux.Handle("GET /protected", auth(http.HandlerFunc(echoUserID)))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAuthMiddleware_QueryParam_NotAccepted(t *testing.T) {
	// Regular REST endpoints must NOT accept ?access_token= — header only.
	userID := uuid.New()
	token := makeToken(t, userID, 15*time.Minute)

	mux := http.NewServeMux()
	auth := httptransport.AuthMiddleware(testJWTSecret)
	mux.Handle("GET /protected", auth(http.HandlerFunc(echoUserID)))

	req := httptest.NewRequest(http.MethodGet, "/protected?access_token="+token, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestWSAuthMiddleware_Header_Valid(t *testing.T) {
	userID := uuid.New()
	token := makeToken(t, userID, 15*time.Minute)

	mux := http.NewServeMux()
	wsAuth := httptransport.WSAuthMiddleware(testJWTSecret)
	mux.Handle("GET /v1/ws/tracker", wsAuth(http.HandlerFunc(echoUserID)))

	req := httptest.NewRequest(http.MethodGet, "/v1/ws/tracker", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, userID.String(), rec.Body.String())
}

func TestWSAuthMiddleware_QueryParam_Valid(t *testing.T) {
	userID := uuid.New()
	token := makeToken(t, userID, 15*time.Minute)

	mux := http.NewServeMux()
	wsAuth := httptransport.WSAuthMiddleware(testJWTSecret)
	mux.Handle("GET /v1/ws/tracker", wsAuth(http.HandlerFunc(echoUserID)))

	req := httptest.NewRequest(http.MethodGet, "/v1/ws/tracker?access_token="+token, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, userID.String(), rec.Body.String())
}

func TestWSAuthMiddleware_HeaderPriorityOverQueryParam(t *testing.T) {
	// Header contains a valid token; query param contains an invalid one.
	// Header must win.
	userID := uuid.New()
	validToken := makeToken(t, userID, 15*time.Minute)

	mux := http.NewServeMux()
	wsAuth := httptransport.WSAuthMiddleware(testJWTSecret)
	mux.Handle("GET /v1/ws/tracker", wsAuth(http.HandlerFunc(echoUserID)))

	req := httptest.NewRequest(http.MethodGet, "/v1/ws/tracker?access_token=bad-token", nil)
	req.Header.Set("Authorization", "Bearer "+validToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, userID.String(), rec.Body.String())
}

func TestWSAuthMiddleware_MissingToken_Rejected(t *testing.T) {
	mux := http.NewServeMux()
	wsAuth := httptransport.WSAuthMiddleware(testJWTSecret)
	mux.Handle("GET /v1/ws/tracker", wsAuth(http.HandlerFunc(echoUserID)))

	req := httptest.NewRequest(http.MethodGet, "/v1/ws/tracker", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestWSAuthMiddleware_InvalidQueryToken_Rejected(t *testing.T) {
	mux := http.NewServeMux()
	wsAuth := httptransport.WSAuthMiddleware(testJWTSecret)
	mux.Handle("GET /v1/ws/tracker", wsAuth(http.HandlerFunc(echoUserID)))

	req := httptest.NewRequest(http.MethodGet, "/v1/ws/tracker?access_token=not-a-jwt", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestWSAuthMiddleware_ExpiredQueryToken_Rejected(t *testing.T) {
	userID := uuid.New()
	expiredToken := makeToken(t, userID, -1*time.Minute)

	mux := http.NewServeMux()
	wsAuth := httptransport.WSAuthMiddleware(testJWTSecret)
	mux.Handle("GET /v1/ws/tracker", wsAuth(http.HandlerFunc(echoUserID)))

	req := httptest.NewRequest(http.MethodGet, "/v1/ws/tracker?access_token="+expiredToken, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
