package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NATSPinger is satisfied by *natseventbus.Bus.
type NATSPinger interface {
	Ping(ctx context.Context) error
}

type dbPinger func(ctx context.Context, db *pgxpool.Pool) error

func defaultDBPinger(ctx context.Context, db *pgxpool.Pool) error {
	return db.Ping(ctx)
}

// HealthHandler exposes liveness and readiness endpoints.
type HealthHandler struct {
	db   *pgxpool.Pool
	nats NATSPinger
}

// Liveness godoc
// @Summary      Liveness probe
// @Tags         system
// @Produce      json
// @Success      200  {object}  map[string]string
// @Router       /healthz [get]
func (h *HealthHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// Readiness godoc
// @Summary      Readiness probe — checks DB and NATS connectivity
// @Tags         system
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Failure      503  {object}  map[string]interface{}
// @Router       /readyz [get]
func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	h.readinessWithPinger(w, r, defaultDBPinger)
}

func (h *HealthHandler) readinessWithPinger(w http.ResponseWriter, r *http.Request, ping dbPinger) {
	ctx := r.Context()
	checks := map[string]string{}
	degraded := false

	if err := ping(ctx, h.db); err != nil {
		checks["postgres"] = err.Error()
		degraded = true
	} else {
		checks["postgres"] = "ok"
	}

	if err := h.nats.Ping(ctx); err != nil {
		checks["nats"] = err.Error()
		degraded = true
	} else {
		checks["nats"] = "ok"
	}

	w.Header().Set("Content-Type", "application/json")
	if degraded {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]any{"status": "degraded", "checks": checks})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"status": "ok", "checks": checks})
}
