package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	eventbusnats "github.com/wrany/libs/eventbus/nats"
	"github.com/wrany/tracking-worker/internal/config"
	"github.com/wrany/tracking-worker/internal/domain"
	"github.com/wrany/tracking-worker/internal/observ"
	"github.com/wrany/tracking-worker/internal/storage/postgres"
	httptransport "github.com/wrany/tracking-worker/internal/transport/http"
	natstransport "github.com/wrany/tracking-worker/internal/transport/nats"
	"github.com/wrany/tracking-worker/internal/usecase"
)

// App wires all components and manages the service lifecycle.
type App struct {
	httpSrv            *http.Server
	locationConsumer   *natstransport.LocationConsumer
	tripDetectionJob   *usecase.TripDetectionJob
	tripDetectionIvl   time.Duration
	routeMatchingJob   *usecase.RouteMatchingJob
	routeMatchingIvl   time.Duration
	bus                *eventbusnats.Bus
	db                 *pgxpool.Pool
	metrics            *observ.WorkerMetrics
}

// New builds the App from config: connects to Postgres and NATS,
// creates the durable consumer, wires the processor.
func New(ctx context.Context, cfg config.Config) (*App, error) {
	// Postgres connection pool.
	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("app: connect postgres: %w", err)
	}
	if err := db.Ping(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("app: ping postgres: %w", err)
	}

	// NATS JetStream bus (publisher + consumer substrate).
	natsCfg := eventbusnats.Config{URL: cfg.NatsURL, Stream: cfg.NatsStream}
	bus, err := eventbusnats.Connect(natsCfg)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("app: connect nats: %w", err)
	}
	if err := bus.EnsureStream(ctx); err != nil {
		_ = bus.Close()
		db.Close()
		return nil, fmt.Errorf("app: ensure stream: %w", err)
	}
	slog.Info("nats: connected", "url", cfg.NatsURL, "stream", cfg.NatsStream)

	// Durable pull consumer for location.events.v1.
	consumerCfg := eventbusnats.ConsumerConfig{
		Stream:         cfg.NatsStream,
		FilterSubject:  cfg.NatsLocationSubject,
		DurableName:    cfg.NatsLocationConsumerDurable,
		AckWait:        time.Duration(cfg.NatsConsumerAckWaitSec) * time.Second,
		MaxDeliver:     cfg.NatsConsumerMaxDeliver,
		FetchBatchSize: cfg.NatsConsumerBatchSize,
		FetchTimeout:   time.Duration(cfg.NatsConsumerPollTimeoutSec) * time.Second,
	}
	consumer, err := eventbusnats.NewJetStreamConsumer(ctx, bus, consumerCfg)
	if err != nil {
		_ = bus.Close()
		db.Close()
		return nil, fmt.Errorf("app: create jetstream consumer: %w", err)
	}

	// Wire storage → usecase → transport.
	rawRepo := postgres.NewRawLocationRepo(db)
	processor := usecase.NewLocationEventProcessor(
		rawRepo,
		bus,
		"tracking-worker",
		cfg.NatsLocationConsumerDurable,
	)
	locationConsumer := natstransport.NewLocationConsumer(consumer, processor)

	tripRepo := postgres.NewTripRepo(db)
	tripJob := usecase.NewTripDetectionJob(tripRepo, bus, "tracking-worker", domain.DefaultTripDetectionConfig())

	routeRepo := postgres.NewRouteRepo(db)
	routeJob := usecase.NewRouteMatchingJob(routeRepo, bus, "tracking-worker", domain.DefaultRouteMatchConfig())

	metrics := observ.NewWorkerMetrics()

	return &App{
		httpSrv: &http.Server{
			Addr:    ":" + cfg.Port,
			Handler: httptransport.NewRouter(metrics),
		},
		locationConsumer: locationConsumer,
		tripDetectionJob: tripJob,
		tripDetectionIvl: time.Duration(cfg.TripDetectionIntervalSec) * time.Second,
		routeMatchingJob: routeJob,
		routeMatchingIvl: time.Duration(cfg.RouteMatchingIntervalSec) * time.Second,
		bus:              bus,
		db:               db,
		metrics:          metrics,
	}, nil
}

// Run starts the HTTP health endpoint and the NATS consumer loop.
// Blocks until ctx is cancelled.
func (a *App) Run(ctx context.Context) error {
	go func() {
		slog.Info("tracking-worker: HTTP health server", "addr", a.httpSrv.Addr)
		if err := a.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("tracking-worker: HTTP server error", "err", err)
		}
	}()

	go func() {
		ticker := time.NewTicker(a.tripDetectionIvl)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := a.tripDetectionJob.RunOnce(ctx); err != nil {
					slog.Error("trip_detection_job: run error", "err", err)
				}
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(a.routeMatchingIvl)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := a.routeMatchingJob.RunOnce(ctx); err != nil {
					slog.Error("route_matching_job: run error", "err", err)
				}
			}
		}
	}()

	slog.Info("tracking-worker: starting location consumer")
	if err := a.locationConsumer.Run(ctx); err != nil {
		return fmt.Errorf("location consumer: %w", err)
	}
	return nil
}

// Shutdown drains in-flight work and closes all connections.
func (a *App) Shutdown(ctx context.Context) error {
	slog.Info("tracking-worker: shutting down")
	if err := a.httpSrv.Shutdown(ctx); err != nil {
		slog.Error("tracking-worker: HTTP shutdown error", "err", err)
	}
	if err := a.locationConsumer.Close(); err != nil {
		slog.Error("tracking-worker: consumer close error", "err", err)
	}
	if err := a.bus.Close(); err != nil {
		slog.Error("tracking-worker: nats close error", "err", err)
	}
	a.db.Close()
	return nil
}
