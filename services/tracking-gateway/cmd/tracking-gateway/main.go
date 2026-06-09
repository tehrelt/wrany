package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wrany/tracking-gateway/internal/app"
	"github.com/wrany/tracking-gateway/internal/config"
	"github.com/wrany/tracking-gateway/internal/migrations"
)

func main() {
	cfg := config.Load()

	db, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(context.Background()); err != nil {
		log.Fatalf("ping database: %v", err)
	}

	if err := migrations.Run(cfg.DatabaseURL, cfg.MigrationsPath); err != nil {
		log.Fatalf("migrations failed: %v", err)
	}

	a := app.New(cfg, db)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		if err := a.Run(); err != nil {
			log.Printf("server error: %v", err)
		}
	}()

	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := a.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}
