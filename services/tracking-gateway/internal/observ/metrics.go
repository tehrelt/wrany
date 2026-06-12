package observ

import (
	"github.com/prometheus/client_golang/prometheus"
	obsmetrics "github.com/wrany/libs/observability/metrics"
)

// GatewayMetrics holds all Prometheus instruments for tracking-gateway.
type GatewayMetrics struct {
	reg *prometheus.Registry

	// WebSocket
	WSConnectionsActive    prometheus.Gauge
	WSConnectionsTotal     prometheus.Counter
	WSSessionsAccepted     prometheus.Counter
	WSSessionsRejected     prometheus.Counter

	// Location ingestion
	LocationBatchesReceived prometheus.Counter
	LocationBatchesAcked    prometheus.Counter
	LocationBatchesRejected prometheus.Counter
	LocationPointsReceived  prometheus.Counter

	// NATS
	NATSPublishTotal  *prometheus.CounterVec
	NATSPublishErrors *prometheus.CounterVec

	// Auth
	AuthRequestsTotal *prometheus.CounterVec

	// HTTP (populated by middleware)
	HTTP *HTTPMetrics
}

// HTTPMetrics wraps the shared HTTP counters from libs/observability/middleware.
// Kept here so callers can pass GatewayMetrics.HTTP to the middleware constructor.
type HTTPMetrics struct {
	RequestsTotal    *prometheus.CounterVec
	RequestDuration  *prometheus.HistogramVec
	RequestsInFlight prometheus.Gauge
}

const ns = "gateway"

// NewGatewayMetrics registers all gateway metrics on a custom Prometheus registry.
func NewGatewayMetrics() *GatewayMetrics {
	reg := obsmetrics.NewRegistry()
	m := &GatewayMetrics{reg: reg}

	m.WSConnectionsActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: ns, Name: "ws_connections_active",
		Help: "Current number of active WebSocket connections.",
	})
	m.WSConnectionsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns, Name: "ws_connections_total",
		Help: "Total WebSocket connections established.",
	})
	m.WSSessionsAccepted = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns, Name: "ws_sessions_accepted_total",
		Help: "Total tracker sessions accepted.",
	})
	m.WSSessionsRejected = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns, Name: "ws_sessions_rejected_total",
		Help: "Total tracker sessions rejected.",
	})
	m.LocationBatchesReceived = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns, Name: "location_batches_received_total",
		Help: "Total location batches received over WebSocket.",
	})
	m.LocationBatchesAcked = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns, Name: "location_batches_acked_total",
		Help: "Total location batches successfully acknowledged.",
	})
	m.LocationBatchesRejected = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns, Name: "location_batches_rejected_total",
		Help: "Total location batches rejected (error or bus unavailable).",
	})
	m.LocationPointsReceived = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: ns, Name: "location_points_received_total",
		Help: "Total location points received across all batches.",
	})
	m.NATSPublishTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns, Name: "nats_publish_total",
		Help: "Total NATS publish calls by subject.",
	}, []string{"subject"})
	m.NATSPublishErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns, Name: "nats_publish_errors_total",
		Help: "Total NATS publish errors by subject and error_type.",
	}, []string{"subject", "error_type"})
	m.AuthRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: ns, Name: "auth_requests_total",
		Help: "Total auth requests by result (success|failure).",
	}, []string{"result"})

	m.HTTP = &HTTPMetrics{
		RequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns, Name: "http_requests_total",
			Help: "Total HTTP requests by method, endpoint, and status_code.",
		}, []string{"method", "endpoint", "status_code"}),
		RequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: ns, Name: "http_request_duration_seconds",
			Help:    "HTTP request latency by method and endpoint.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "endpoint"}),
		RequestsInFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns, Name: "http_requests_in_flight",
			Help: "Current number of in-flight HTTP requests.",
		}),
	}

	reg.MustRegister(
		m.WSConnectionsActive,
		m.WSConnectionsTotal,
		m.WSSessionsAccepted,
		m.WSSessionsRejected,
		m.LocationBatchesReceived,
		m.LocationBatchesAcked,
		m.LocationBatchesRejected,
		m.LocationPointsReceived,
		m.NATSPublishTotal,
		m.NATSPublishErrors,
		m.AuthRequestsTotal,
		m.HTTP.RequestsTotal,
		m.HTTP.RequestDuration,
		m.HTTP.RequestsInFlight,
	)

	return m
}

// Registry returns the Prometheus registry for use with promhttp.HandlerFor.
func (m *GatewayMetrics) Registry() *prometheus.Registry { return m.reg }
