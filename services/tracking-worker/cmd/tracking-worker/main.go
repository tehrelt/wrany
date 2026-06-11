package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/wrany/tracking-worker/internal/app"
	"github.com/wrany/tracking-worker/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	a, err := app.New(ctx, cfg)
	if err != nil {
		log.Fatalf("init: %v", err)
	}

	if err := a.Run(ctx); err != nil {
		log.Printf("run: %v", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := a.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
}
