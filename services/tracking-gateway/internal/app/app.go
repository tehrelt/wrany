package app

import (
	"context"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wrany/tracking-gateway/internal/config"
	"github.com/wrany/tracking-gateway/internal/storage/postgres"
	httptransport "github.com/wrany/tracking-gateway/internal/transport/http"
	"github.com/wrany/tracking-gateway/internal/usecase"
)

type App struct {
	srv *http.Server
}

func New(cfg config.Config, db *pgxpool.Pool) *App {
	userRepo := postgres.NewUserRepo(db)
	deviceRepo := postgres.NewDeviceRepo(db)
	tokenRepo := postgres.NewTokenRepo(db)

	authUC := usecase.NewAuthUsecase(userRepo, tokenRepo, usecase.AuthConfig{
		JWTSecret:  []byte(cfg.JWTSecret),
		AccessTTL:  cfg.JWTAccessTTL,
		RefreshTTL: cfg.JWTRefreshTTL,
	})
	deviceUC := usecase.NewDeviceUsecase(deviceRepo)
	meUC := usecase.NewMeUsecase(userRepo)

	router := httptransport.NewRouter(httptransport.RouterDeps{
		Auth:      authUC,
		Device:    deviceUC,
		Me:        meUC,
		JWTSecret: []byte(cfg.JWTSecret),
	})

	return &App{
		srv: &http.Server{
			Addr:    ":" + cfg.Port,
			Handler: router,
		},
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
	return a.srv.Shutdown(ctx)
}
