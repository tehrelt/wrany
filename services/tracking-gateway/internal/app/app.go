package app

import (
	"context"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wrany/libs/eventbus"
	natseventbus "github.com/wrany/libs/eventbus/nats"
	"github.com/wrany/tracking-gateway/internal/config"
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

	authUC := usecase.NewAuthUsecase(userRepo, tokenRepo, usecase.AuthConfig{
		JWTSecret:  []byte(cfg.JWTSecret),
		AccessTTL:  cfg.JWTAccessTTL,
		RefreshTTL: cfg.JWTRefreshTTL,
	})
	deviceUC := usecase.NewDeviceUsecase(deviceRepo)
	meUC := usecase.NewMeUsecase(userRepo)

	if cfg.NatsURL == "" {
		log.Fatal("nats: NATS_URL is required — set NATS_URL to connect to JetStream")
	}
	bus, err := natseventbus.Connect(natseventbus.Config{
		URL:    cfg.NatsURL,
		Stream: cfg.NatsStream,
	})
	if err != nil {
		log.Fatalf("nats: connect: %v", err)
	}
	if err := bus.EnsureStream(context.Background()); err != nil {
		log.Fatalf("nats: ensure stream: %v", err)
	}
	natsBus := bus
	log.Printf("nats: connected to %s stream=%s", cfg.NatsURL, cfg.NatsStream)
	var pub eventbus.Publisher = natsBus

	trackerUC := usecase.NewTrackerIngestionUseCase(
		deviceRepo,
		dedupRepo,
		pub,
		"tracking-gateway",
		cfg.WSMaxBatchSize,
	)

	router := httptransport.NewRouter(httptransport.RouterDeps{
		Auth:      authUC,
		Device:    deviceUC,
		Me:        meUC,
		Tracker:   trackerUC,
		JWTSecret: []byte(cfg.JWTSecret),
		Config:    cfg,
	})

	return &App{
		srv: &http.Server{
			Addr:    ":" + cfg.Port,
			Handler: router,
		},
		natsBus: natsBus,
	}
}

func (a *App) Run() error {
	log.Printf("tracking-gateway listening on %s", a.srv.Addr)
	if err := a.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (a *App) Shutdown(ctx context.Context) error {
	log.Println("tracking-gateway shutting down")
	if err := a.srv.Shutdown(ctx); err != nil {
		return err
	}
	if a.natsBus != nil {
		if err := a.natsBus.Close(); err != nil {
			log.Printf("nats: close: %v", err)
		}
	}
	return nil
}
