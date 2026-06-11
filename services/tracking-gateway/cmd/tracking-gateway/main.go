package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	_ "github.com/wrany/tracking-gateway/docs"
	"github.com/wrany/tracking-gateway/internal/app"
	"github.com/wrany/tracking-gateway/internal/config"
	"github.com/wrany/tracking-gateway/internal/migrations"
)

// @title          WR any% API
// @version        0.1.0
// @description    Backend API for the WR any% automatic route tracking application.
// @description    WebSocket tracker: ws://host/v1/ws/tracker — see docs/contracts/websocket-tracker-protocol.md
// @host           localhost:8080
// @BasePath       /
// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                 Bearer JWT access token. For WebSocket upgrade ?access_token=<jwt> is also accepted.
func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))

	cfg := config.Load()

	db, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		slog.Error("connect to database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(context.Background()); err != nil {
		slog.Error("ping database", "err", err)
		os.Exit(1)
	}

	if err := migrations.Run(cfg.DatabaseURL, cfg.MigrationsPath); err != nil {
		slog.Error("migrations failed", "err", err)
		os.Exit(1)
	}

	a := app.New(cfg, db)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		if err := a.Run(); err != nil {
			slog.Error("server error", "err", err)
		}
	}()

	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := a.Shutdown(ctx); err != nil {
		slog.Error("shutdown error", "err", err)
	}
}
