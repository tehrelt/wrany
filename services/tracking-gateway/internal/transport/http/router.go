package http

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	obsmiddleware "github.com/wrany/libs/observability/middleware"
	"github.com/wrany/tracking-gateway/internal/config"
	"github.com/wrany/tracking-gateway/internal/observ"
	"github.com/wrany/tracking-gateway/internal/usecase"
)

type RouterDeps struct {
	Auth          *usecase.AuthUsecase
	Device        *usecase.DeviceUsecase
	Me            *usecase.MeUsecase
	Tracker       *usecase.TrackerIngestionUseCase
	TrackingQuery *usecase.TrackingQueryUsecase
	Trips         *usecase.TripQueryUsecase
	Routes        *usecase.RouteQueryUsecase
	RouteResults  *usecase.RouteResultQueryUsecase
	JWTSecret     []byte
	Config        config.Config
	Metrics       *observ.GatewayMetrics
	DB            *pgxpool.Pool
	NATS          NATSPinger
}

// NewRouter returns the full HTTP handler chain:
// RequestID → Logging/Metrics → CORSMiddleware → mux.
func NewRouter(deps RouterDeps) http.Handler {
	mux := http.NewServeMux()
	auth := AuthMiddleware(deps.JWTSecret)

	authH := NewAuthHandler(deps.Auth)
	deviceH := NewDeviceHandler(deps.Device)
	meH := NewMeHandler(deps.Me)
	trackerH := NewTrackerHandler(deps.Tracker, deps.Config, deps.Metrics)
	trackingQueryH := NewTrackingQueryHandler(deps.TrackingQuery)
	tripH := NewTripHandler(deps.Trips)
	routeH := NewRouteHandler(deps.Routes)
	routeResultH := NewRouteResultHandler(deps.RouteResults)

	// Observability endpoints — no auth, no logging overhead.
	mux.Handle("GET /metrics", promhttp.HandlerFor(deps.Metrics.Registry(), promhttp.HandlerOpts{}))
	healthH := &HealthHandler{db: deps.DB, nats: deps.NATS}
	mux.HandleFunc("/healthz", healthH.Liveness)
	mux.HandleFunc("GET /readyz", healthH.Readiness)
	mux.HandleFunc("GET /swagger/doc.json", SwaggerDocHandler)

	mux.HandleFunc("POST /v1/auth/register", authH.Register)
	mux.HandleFunc("POST /v1/auth/login", authH.Login)
	mux.HandleFunc("POST /v1/auth/refresh", authH.Refresh)
	mux.Handle("POST /v1/devices/register", auth(http.HandlerFunc(deviceH.RegisterDevice)))
	mux.Handle("GET /v1/devices", auth(http.HandlerFunc(deviceH.ListDevices)))
	mux.Handle("GET /v1/me", auth(http.HandlerFunc(meH.GetMe)))
	mux.Handle("GET /v1/tracking/points", auth(http.HandlerFunc(trackingQueryH.GetPoints)))
	mux.Handle("GET /v1/tracking/summary", auth(http.HandlerFunc(trackingQueryH.GetSummary)))
	mux.Handle("GET /v1/tracking/track", auth(http.HandlerFunc(trackingQueryH.GetTrack)))
	mux.Handle("GET /v1/tracking/fast-segments", auth(http.HandlerFunc(trackingQueryH.GetFastSegments)))
	mux.Handle("DELETE /v1/tracking/points/{event_id}", auth(http.HandlerFunc(trackingQueryH.DeletePoint)))
	mux.Handle("GET /v1/trips", auth(http.HandlerFunc(tripH.ListTrips)))
	mux.Handle("GET /v1/trips/{id}", auth(http.HandlerFunc(tripH.GetTrip)))
	mux.Handle("DELETE /v1/trips/{id}", auth(http.HandlerFunc(tripH.DeleteTrip)))
	mux.Handle("GET /v1/trips/{id}/points", auth(http.HandlerFunc(tripH.GetTripPoints)))
	mux.Handle("GET /v1/routes", auth(http.HandlerFunc(routeH.ListRoutes)))
	mux.Handle("GET /v1/routes/{id}", auth(http.HandlerFunc(routeH.GetRoute)))
	mux.Handle("DELETE /v1/routes/{id}", auth(http.HandlerFunc(routeH.DeleteRoute)))
	mux.Handle("GET /v1/routes/{id}/trips", auth(http.HandlerFunc(routeH.ListRouteTrips)))
	mux.Handle("GET /v1/routes/{id}/points", auth(http.HandlerFunc(routeH.GetRoutePoints)))
	mux.Handle("GET /v1/routes/{route_id}/results", auth(http.HandlerFunc(routeResultH.GetRouteResult)))
	mux.Handle("GET /v1/routes/{route_id}/attempts", auth(http.HandlerFunc(routeResultH.ListRouteAttempts)))
	wsAuth := WSAuthMiddleware(deps.JWTSecret)
	mux.Handle("GET /v1/ws/tracker", wsAuth(trackerH))

	var h http.Handler = mux
	h = CORSMiddleware(h)
	h = LoggingMiddleware(h, deps.Metrics)
	h = obsmiddleware.RequestID(h)
	return h
}
