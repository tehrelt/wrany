package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

type mockNATS struct{ err error }

func (m *mockNATS) Ping(_ context.Context) error { return m.err }

var pingOK = func(_ context.Context, _ *pgxpool.Pool) error { return nil }
var pingFail = func(_ context.Context, _ *pgxpool.Pool) error { return errors.New("connection refused") }

func TestHealthHandler_Liveness(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	h := &HealthHandler{}
	h.Liveness(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
	}
}

func TestHealthHandler_Readiness_OK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	h := &HealthHandler{nats: &mockNATS{}}
	h.readinessWithPinger(w, req, pingOK)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHealthHandler_Readiness_NATSDown(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	h := &HealthHandler{nats: &mockNATS{err: errors.New("connection refused")}}
	h.readinessWithPinger(w, req, pingOK)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "degraded") {
		t.Errorf("expected degraded in body, got %s", w.Body.String())
	}
}

func TestHealthHandler_Readiness_DBDown(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	h := &HealthHandler{nats: &mockNATS{}}
	h.readinessWithPinger(w, req, pingFail)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}
