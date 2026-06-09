package app

import (
	"context"
	"log"
	"net/http"

	"github.com/wrany/tracking-gateway/internal/config"
	httptransport "github.com/wrany/tracking-gateway/internal/transport/http"
)

type App struct {
	srv *http.Server
}

func New(cfg config.Config) *App {
	return &App{
		srv: &http.Server{
			Addr:    ":" + cfg.Port,
			Handler: httptransport.NewRouter(),
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
