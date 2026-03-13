package metrics

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// HTTPMiddleware returns a chi-compatible middleware that records HTTP request
// count and duration metrics. The path label uses chi's route pattern to
// avoid cardinality explosion from path parameters like UUIDs.
func HTTPMiddleware(m *Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			duration := time.Since(start).Seconds()
			status := fmt.Sprintf("%d", ww.Status())

			pattern := chi.RouteContext(r.Context()).RoutePattern()
			if pattern == "" {
				pattern = "unknown"
			}

			m.HTTPRequestsTotal.WithLabelValues(r.Method, pattern, status).Inc()
			m.HTTPRequestDuration.WithLabelValues(r.Method, pattern, status).Observe(duration)
		})
	}
}
