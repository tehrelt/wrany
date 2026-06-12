package app

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wrany/libs/eventbus"
	natseventbus "github.com/wrany/libs/eventbus/nats"
	"github.com/wrany/tracking-gateway/internal/config"
	"github.com/wrany/tracking-gateway/internal/observ"
	"github.com/wrany/tracking-gateway/internal/storage/postgres"
	httptransport "github.com/wrany/tracking-gateway/internal/transport/http"
	"github.com/wrany/tracking-gateway/internal/usecase"
)

type App struct {
	srv     *http.Server
	natsBus *natseventbus.Bus
}

func New(cfg config.Config, db *pgxpool.Pool) *App {
	userRepo := postgres.NewUserRepo(db)
	deviceRepo := postgres.NewDeviceRepo(db)
	tokenRepo := postgres.NewTokenRepo(db)
	dedupRepo := postgres.NewIngestionDedupRepo(db)
	trackingQueryRepo := postgres.NewTrackingQueryRepo(db)
	tripQueryRepo := postgres.NewTripQueryRepo(db)
	routeQueryRepo := postgres.NewRouteQueryRepo(db)
	routeResultQueryRepo := postgres.NewRouteResultQueryRepo(db)

	authUC := usecase.NewAuthUsecase(userRepo, tokenRepo, usecase.AuthConfig{
		JWTSecret:  []byte(cfg.JWTSecret),
		AccessTTL:  cfg.JWTAccessTTL,
		RefreshTTL: cfg.JWTRefreshTTL,
	})
	deviceUC := usecase.NewDeviceUsecase(deviceRepo)
	meUC := usecase.NewMeUsecase(userRepo)
	trackingQueryUC := usecase.NewTrackingQueryUsecase(trackingQueryRepo)
	tripQueryUC := usecase.NewTripQueryUsecase(tripQueryRepo)
	routeQueryUC := usecase.NewRouteQueryUsecase(routeQueryRepo)
	routeResultQueryUC := usecase.NewRouteResultQueryUsecase(routeResultQueryRepo, routeQueryRepo)

	if cfg.NatsURL == "" {
		slog.Error("nats: NATS_URL is required — set NATS_URL to connect to JetStream")
		os.Exit(1)
	}
	bus, err := natseventbus.Connect(natseventbus.Config{
		URL:    cfg.NatsURL,
		Stream: cfg.NatsStream,
	})
	if err != nil {
		slog.Error("nats: connect", "err", err)
		os.Exit(1)
	}
	if err := bus.EnsureStream(context.Background()); err != nil {
		slog.Error("nats: ensure stream", "err", err)
		os.Exit(1)
	}
	natsBus := bus
	slog.Info("nats: connected", "url", cfg.NatsURL, "stream", cfg.NatsStream)
	var pub eventbus.Publisher = natsBus

	trackerUC := usecase.NewTrackerIngestionUseCase(
		deviceRepo,
		dedupRepo,
		pub,
		"tracking-gateway",
		cfg.WSMaxBatchSize,
	)

	metrics := observ.NewGatewayMetrics()

	handler := httptransport.NewRouter(httptransport.RouterDeps{
		Auth:          authUC,
		Device:        deviceUC,
		Me:            meUC,
		Tracker:       trackerUC,
		TrackingQuery: trackingQueryUC,
		Trips:         tripQueryUC,
		Routes:        routeQueryUC,
		RouteResults:  routeResultQueryUC,
		JWTSecret:     []byte(cfg.JWTSecret),
		Config:        cfg,
		Metrics:       metrics,
	})

	return &App{
		srv: &http.Server{
			Addr:    ":" + cfg.Port,
			Handler: handler,
		},
		natsBus: natsBus,
	}
}

func (a *App) Run() error {
	slog.Info("tracking-gateway listening", "addr", a.srv.Addr)
	if err := a.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (a *App) Shutdown(ctx context.Context) error {
	slog.Info("tracking-gateway shutting down")
	if err := a.srv.Shutdown(ctx); err != nil {
		return err
	}
	if a.natsBus != nil {
		if err := a.natsBus.Close(); err != nil {
			slog.Error("nats: close", "err", err)
		}
	}
	return nil
}
