package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wrany/libs/observability/logger"
	"github.com/wrany/libs/observability/middleware"
)

func TestRequestID_PreservesExisting(t *testing.T) {
	const existing = "test-request-id-123"
	var captured string

	handler := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = logger.RequestIDFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-Id", existing)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if captured != existing {
		t.Errorf("got request_id %q, want %q", captured, existing)
	}
	if rr.Header().Get("X-Request-Id") != existing {
		t.Errorf("response header X-Request-Id = %q, want %q", rr.Header().Get("X-Request-Id"), existing)
	}
}

func TestRequestID_GeneratesWhenMissing(t *testing.T) {
	var captured string

	handler := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = logger.RequestIDFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if captured == "" {
		t.Error("expected generated request_id, got empty")
	}
	if rr.Header().Get("X-Request-Id") != captured {
		t.Errorf("response header X-Request-Id = %q, want %q", rr.Header().Get("X-Request-Id"), captured)
	}
}
