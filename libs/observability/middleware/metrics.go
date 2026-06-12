package middleware

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// HTTPMetrics holds the Prometheus instruments for HTTP traffic.
type HTTPMetrics struct {
	RequestsTotal    *prometheus.CounterVec
	RequestDuration  *prometheus.HistogramVec
	RequestsInFlight prometheus.Gauge
}

// NewHTTPMetrics registers and returns HTTP metrics on the given registry.
func NewHTTPMetrics(reg prometheus.Registerer, namespace string) *HTTPMetrics {
	m := &HTTPMetrics{
		RequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "http_requests_total",
			Help:      "Total HTTP requests by method, endpoint, and status_code.",
		}, []string{"method", "endpoint", "status_code"}),

		RequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request latency by method and endpoint.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"method", "endpoint"}),

		RequestsInFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "http_requests_in_flight",
			Help:      "Current number of in-flight HTTP requests.",
		}),
	}
	reg.MustRegister(m.RequestsTotal, m.RequestDuration, m.RequestsInFlight)
	return m
}

// Metrics returns middleware that records per-request counters and histograms.
// endpoint should be a low-cardinality route template like "/v1/auth/login".
func Metrics(m *HTTPMetrics, endpoint string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m.RequestsInFlight.Inc()
			defer m.RequestsInFlight.Dec()

			start := time.Now()
			sw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)

			duration := time.Since(start).Seconds()
			code := strconv.Itoa(sw.status)
			m.RequestsTotal.WithLabelValues(r.Method, endpoint, code).Inc()
			m.RequestDuration.WithLabelValues(r.Method, endpoint).Observe(duration)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := s.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("ResponseWriter does not implement http.Hijacker")
	}
	return h.Hijack()
}
