package http

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

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

// bearerToken extracts the raw JWT from the Authorization: Bearer header.
// Returns empty string if the header is absent or malformed.
func bearerToken(r *http.Request) string {
	raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok {
		return ""
	}
	return raw
}
