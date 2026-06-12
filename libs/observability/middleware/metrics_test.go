package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	dto "github.com/prometheus/client_model/go"
	"github.com/wrany/libs/observability/metrics"
	"github.com/wrany/libs/observability/middleware"
)

func TestHTTPMetrics_IncrementCounter(t *testing.T) {
	reg := metrics.NewRegistry()
	m := middleware.NewHTTPMetrics(reg, "test")

	handler := middleware.Metrics(m, "/test")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	var found bool
	for _, mf := range mfs {
		if mf.GetName() == "test_http_requests_total" {
			for _, m := range mf.GetMetric() {
				if labelValue(m.GetLabel(), "endpoint") == "/test" {
					if m.GetCounter().GetValue() == 1 {
						found = true
					}
				}
			}
		}
	}
	if !found {
		t.Error("expected test_http_requests_total counter = 1 for endpoint=/test")
	}
}

func labelValue(labels []*dto.LabelPair, name string) string {
	for _, l := range labels {
		if l.GetName() == name {
			return l.GetValue()
		}
	}
	return ""
}
