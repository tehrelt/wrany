package observ

import (
	"github.com/prometheus/client_golang/prometheus"
	obsmetrics "github.com/wrany/libs/observability/metrics"
)

// WorkerMetrics holds all Prometheus instruments for tracking-worker.
type WorkerMetrics struct {
	reg *prometheus.Registry

	// NATS consumer
	NATSMessagesConsumed       *prometheus.CounterVec
	NATSProcessingErrors       *prometheus.CounterVec

	// Raw points
	RawPointsInserted  prometheus.Counter
	RawPointsDuplicate prometheus.Counter
	DeadLetterPublished *prometheus.CounterVec

	// Trip detection
	TripDetectionRuns     *prometheus.CounterVec
	TripDetectionDuration prometheus.Histogram
	TripDetectionErrors   *prometheus.CounterVec
	TripsCreated          prometheus.Counter
	TripsCompleted        prometheus.Counter

	// Route matching
	RouteMatchingRuns     *prometheus.CounterVec
	RouteMatchingDuration prometheus.Histogram
	RouteMatchingErrors   *prometheus.CounterVec
	RoutesCreated         prometheus.Counter
	RouteMatches          prometheus.Counter
}

const ns = "worker"

// NewWorkerMetrics registers all worker metrics on a custom Prometheus registry.
func NewWorkerMetrics() *WorkerMetrics {
	reg := obsmetrics.NewRegistry()
	m := &WorkerMetrics{reg: reg}

	m.NATSMessagesConsumed = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns, Name: "nats_messages_consumed_total",
		Help: "Total NATS messages consumed by subject.",
	}, []string{"subject"})
	m.NATSProcessingErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns, Name: "nats_message_processing_errors_total",
		Help: "Total NATS message processing errors by subject and error_type.",
	}, []string{"subject", "error_type"})
	m.RawPointsInserted = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns, Name: "raw_points_inserted_total",
		Help: "Total raw location points inserted into the database.",
	})
	m.RawPointsDuplicate = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns, Name: "raw_points_duplicate_total",
		Help: "Total raw location points skipped as duplicates.",
	})
	m.DeadLetterPublished = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns, Name: "dead_letter_published_total",
		Help: "Total messages sent to dead-letter by reason.",
	}, []string{"reason"})
	m.TripDetectionRuns = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns, Name: "trip_detection_runs_total",
		Help: "Total trip detection job runs by result.",
	}, []string{"result"})
	m.TripDetectionDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: ns, Name: "trip_detection_run_duration_seconds",
		Help:    "Trip detection job run duration.",
		Buckets: prometheus.DefBuckets,
	})
	m.TripDetectionErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns, Name: "trip_detection_errors_total",
		Help: "Total trip detection errors by error_type.",
	}, []string{"error_type"})
	m.TripsCreated = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns, Name: "trips_created_total",
		Help: "Total trips created.",
	})
	m.TripsCompleted = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns, Name: "trips_completed_total",
		Help: "Total trips completed.",
	})
	m.RouteMatchingRuns = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns, Name: "route_matching_runs_total",
		Help: "Total route matching job runs by result.",
	}, []string{"result"})
	m.RouteMatchingDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: ns, Name: "route_matching_run_duration_seconds",
		Help:    "Route matching job run duration.",
		Buckets: prometheus.DefBuckets,
	})
	m.RouteMatchingErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns, Name: "route_matching_errors_total",
		Help: "Total route matching errors by error_type.",
	}, []string{"error_type"})
	m.RoutesCreated = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns, Name: "routes_created_total",
		Help: "Total new routes created.",
	})
	m.RouteMatches = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns, Name: "route_matches_total",
		Help: "Total route matches recorded.",
	})

	reg.MustRegister(
		m.NATSMessagesConsumed,
		m.NATSProcessingErrors,
		m.RawPointsInserted,
		m.RawPointsDuplicate,
		m.DeadLetterPublished,
		m.TripDetectionRuns,
		m.TripDetectionDuration,
		m.TripDetectionErrors,
		m.TripsCreated,
		m.TripsCompleted,
		m.RouteMatchingRuns,
		m.RouteMatchingDuration,
		m.RouteMatchingErrors,
		m.RoutesCreated,
		m.RouteMatches,
	)

	return m
}

// Registry returns the Prometheus registry for use with promhttp.HandlerFor.
func (m *WorkerMetrics) Registry() *prometheus.Registry { return m.reg }
