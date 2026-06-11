package http

import (
	"net/http"

	"github.com/wrany/tracking-gateway/internal/config"
	"github.com/wrany/tracking-gateway/internal/usecase"
)

type RouterDeps struct {
	Auth           *usecase.AuthUsecase
	Device         *usecase.DeviceUsecase
	Me             *usecase.MeUsecase
	Tracker        *usecase.TrackerIngestionUseCase
	TrackingQuery  *usecase.TrackingQueryUsecase
	JWTSecret      []byte
	Config         config.Config
}

func NewRouter(deps RouterDeps) *http.ServeMux {
	mux := http.NewServeMux()
	auth := AuthMiddleware(deps.JWTSecret)

	authH := NewAuthHandler(deps.Auth)
	deviceH := NewDeviceHandler(deps.Device)
	meH := NewMeHandler(deps.Me)
	trackerH := NewTrackerHandler(deps.Tracker, deps.Config)
	trackingQueryH := NewTrackingQueryHandler(deps.TrackingQuery)

	mux.HandleFunc("/healthz", HealthzHandler)
	mux.HandleFunc("GET /swagger/doc.json", SwaggerDocHandler)
	mux.HandleFunc("POST /v1/auth/register", authH.Register)
	mux.HandleFunc("POST /v1/auth/login", authH.Login)
	mux.HandleFunc("POST /v1/auth/refresh", authH.Refresh)
	mux.Handle("POST /v1/devices/register", auth(http.HandlerFunc(deviceH.RegisterDevice)))
	mux.Handle("GET /v1/devices", auth(http.HandlerFunc(deviceH.ListDevices)))
	mux.Handle("GET /v1/me", auth(http.HandlerFunc(meH.GetMe)))
	mux.Handle("GET /v1/tracking/points", auth(http.HandlerFunc(trackingQueryH.GetPoints)))
	mux.Handle("GET /v1/tracking/summary", auth(http.HandlerFunc(trackingQueryH.GetSummary)))
	mux.Handle("DELETE /v1/tracking/points/{event_id}", auth(http.HandlerFunc(trackingQueryH.DeletePoint)))
	wsAuth := WSAuthMiddleware(deps.JWTSecret)
	mux.Handle("GET /v1/ws/tracker", wsAuth(trackerH))

	return mux
}
