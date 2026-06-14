package http

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/wrany/tracking-worker/internal/observ"
)

func NewRouter(metrics *observ.WorkerMetrics, db *pgxpool.Pool, nats NATSPinger) *http.ServeMux {
	mux := http.NewServeMux()
	healthH := &HealthHandler{db: db, nats: nats}
	mux.HandleFunc("/healthz", healthH.Liveness)
	mux.HandleFunc("GET /readyz", healthH.Readiness)
	mux.Handle("GET /metrics", promhttp.HandlerFor(metrics.Registry(), promhttp.HandlerOpts{}))
	return mux
}
