package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wrany/tracking-worker/internal/app"
	"github.com/wrany/tracking-worker/internal/config"
)

func main() {
	cfg := config.Load()

	a := app.New(cfg)

	go func() {
		if err := a.Run(); err != nil {
			log.Fatalf("run: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
}
