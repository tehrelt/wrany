package http

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	obslogger "github.com/wrany/libs/observability/logger"
	"github.com/wrany/tracking-gateway/internal/observ"
)

var reUUID = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

func normalizePath(p string) string {
	return reUUID.ReplaceAllString(p, "{id}")
}

type contextKey int

const userIDKey contextKey = iota

// UserIDFromContext extracts the authenticated user ID from a request context.
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDKey).(uuid.UUID)
	return id, ok
}

// WithUserID injects a user ID into a context. Used in tests to bypass AuthMiddleware.
func WithUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// parseJWT validates a raw JWT string and returns the subject UUID.
func parseJWT(raw string, secret []byte) (uuid.UUID, error) {
	token, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return secret, nil
	}, jwt.WithExpirationRequired())
	if err != nil || !token.Valid {
		return uuid.Nil, jwt.ErrSignatureInvalid
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return uuid.Nil, jwt.ErrSignatureInvalid
	}
	sub, _ := claims.GetSubject()
	return uuid.Parse(sub)
}

// CORSMiddleware adds permissive CORS headers for local development (Swagger UI, etc.).
// Not intended for production — configure a real CORS policy before going live.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// AuthMiddleware validates Bearer JWT from the Authorization header only.
// Use for all regular protected REST endpoints.
func AuthMiddleware(jwtSecret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := bearerToken(r)
			if raw == "" {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			userID, err := parseJWT(raw, jwtSecret)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// WSAuthMiddleware validates JWT for the WebSocket tracker endpoint.
// Header takes priority; falls back to ?access_token= query param.
// The query param fallback exists because React Native Android's WebSocket
// API does not support custom headers on the HTTP upgrade request.
// Never apply this middleware to regular REST endpoints.
func WSAuthMiddleware(jwtSecret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := bearerToken(r)
			if raw == "" {
				// Fallback: ?access_token= query param (WS upgrade only).
				// Do NOT log r.URL or query string — the token would leak.
				raw = r.URL.Query().Get("access_token")
			}
			if raw == "" {
				slog.Warn("ws: auth: no token")
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			userID, err := parseJWT(raw, jwtSecret)
			if err != nil {
				slog.Warn("ws: auth: invalid token", "err", err)
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			ctx := context.WithValue(r.Context(), userIDKey, userID)
			ctx = obslogger.WithUserID(ctx, userID.String())
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.status = code
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := sw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker")
	}
	return h.Hijack()
}

// LoggingMiddleware logs method, path, status, and latency for every request.
// When metrics is non-nil it also records Prometheus counters and histograms.
func LoggingMiddleware(next http.Handler, metrics *observ.GatewayMetrics) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		duration := time.Since(start)
		requestID := obslogger.RequestIDFromContext(r.Context())
		slog.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"latency_ms", duration.Milliseconds(),
			"request_id", requestID,
		)

		if metrics != nil {
			endpoint := normalizePath(r.URL.Path)
			code := strconv.Itoa(sw.status)
			metrics.HTTP.RequestsTotal.WithLabelValues(r.Method, endpoint, code).Inc()
			metrics.HTTP.RequestDuration.WithLabelValues(r.Method, endpoint).Observe(duration.Seconds())
		}
	})
}

// bearerToken extracts the raw JWT from the Authorization: Bearer header.
// Returns empty string if the header is absent or malformed.
func bearerToken(r *http.Request) string {
	raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok {
		return ""
	}
	return raw
}
