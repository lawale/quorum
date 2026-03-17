package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/lawale/quorum/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestNew_RegistersAllCollectors(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.New(reg)
	if m == nil {
		t.Fatal("expected non-nil Metrics")
	}

	// Touch all collectors so they appear in Gather output
	m.HTTPRequestsTotal.WithLabelValues("GET", "/test", "200")
	m.HTTPRequestDuration.WithLabelValues("GET", "/test", "200")
	m.RequestsTotal.WithLabelValues("created")
	m.WebhookDeliveriesTotal.WithLabelValues("success")
	m.AuthHookTotal.WithLabelValues("allowed")

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	// 10 collectors registered (7 original + auth_hook_total + auth_hook_duration + expiry_errors_total)
	if len(families) != 10 {
		t.Errorf("registered families = %d, want 10", len(families))
		for _, f := range families {
			t.Logf("  %s", *f.Name)
		}
	}
}

func TestNew_PanicsOnDuplicateRegistration(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics.New(reg)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate registration")
		}
	}()
	metrics.New(reg)
}

func TestHTTPMiddleware_RecordsRequestCount(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.New(reg)

	r := chi.NewRouter()
	r.Use(metrics.HTTPMiddleware(m))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	count := testutil.ToFloat64(m.HTTPRequestsTotal.WithLabelValues("GET", "/test", "200"))
	if count != 1 {
		t.Errorf("request count = %f, want 1", count)
	}
}

func TestHTTPMiddleware_CapturesStatusCode(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.New(reg)

	r := chi.NewRouter()
	r.Use(metrics.HTTPMiddleware(m))
	r.Get("/fail", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	req := httptest.NewRequest("GET", "/fail", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	count := testutil.ToFloat64(m.HTTPRequestsTotal.WithLabelValues("GET", "/fail", "404"))
	if count != 1 {
		t.Errorf("404 count = %f, want 1", count)
	}
}

func TestHTTPMiddleware_UsesRoutePattern(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.New(reg)

	r := chi.NewRouter()
	r.Use(metrics.HTTPMiddleware(m))
	r.Get("/items/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Hit with two different IDs — should both map to the same route pattern
	for _, id := range []string{"abc", "def"} {
		req := httptest.NewRequest("GET", "/items/"+id, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
	}

	count := testutil.ToFloat64(m.HTTPRequestsTotal.WithLabelValues("GET", "/items/{id}", "200"))
	if count != 2 {
		t.Errorf("route pattern count = %f, want 2", count)
	}
}

func TestRequestsTotal_Counter(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.New(reg)

	m.RequestsTotal.WithLabelValues("created").Inc()
	m.RequestsTotal.WithLabelValues("approved").Inc()
	m.RequestsTotal.WithLabelValues("approved").Inc()

	if v := testutil.ToFloat64(m.RequestsTotal.WithLabelValues("created")); v != 1 {
		t.Errorf("created = %f, want 1", v)
	}
	if v := testutil.ToFloat64(m.RequestsTotal.WithLabelValues("approved")); v != 2 {
		t.Errorf("approved = %f, want 2", v)
	}
}

func TestPendingGauge_IncDec(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.New(reg)

	m.PendingRequestsGauge.Inc()
	m.PendingRequestsGauge.Inc()
	m.PendingRequestsGauge.Dec()

	if v := testutil.ToFloat64(m.PendingRequestsGauge); v != 1 {
		t.Errorf("pending = %f, want 1", v)
	}
}

func TestWebhookDeliveriesTotal_Counter(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.New(reg)

	m.WebhookDeliveriesTotal.WithLabelValues("success").Inc()
	m.WebhookDeliveriesTotal.WithLabelValues("failure").Inc()
	m.WebhookDeliveriesTotal.WithLabelValues("failure").Inc()

	if v := testutil.ToFloat64(m.WebhookDeliveriesTotal.WithLabelValues("success")); v != 1 {
		t.Errorf("success = %f, want 1", v)
	}
	if v := testutil.ToFloat64(m.WebhookDeliveriesTotal.WithLabelValues("failure")); v != 2 {
		t.Errorf("failure = %f, want 2", v)
	}
}
