package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wrany/tracking-worker/internal/app"
	"github.com/wrany/tracking-worker/internal/config"
	"github.com/wrany/libs/observability/tracing"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})).With("service", "tracking-worker"))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	shutdownTracing, err := tracing.Init(ctx, tracing.Config{
		ServiceName:  "tracking-worker",
		OTLPEndpoint: cfg.OTELEndpoint,
		Enabled:      cfg.OTELEnabled,
	})
	if err != nil {
		slog.Error("tracing init", "err", err)
		os.Exit(1)
	}
	defer shutdownTracing(context.Background())

	a, err := app.New(ctx, cfg)
	if err != nil {
		slog.Error("init", "err", err)
		os.Exit(1)
	}

	if err := a.Run(ctx); err != nil {
		slog.Error("run", "err", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := a.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown", "err", err)
		os.Exit(1)
	}
}
