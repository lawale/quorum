package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/lawale/quorum/internal/auth"
	"github.com/lawale/quorum/internal/service"
)

func authMiddleware(provider auth.Provider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity, err := provider.Authenticate(r)
			if err != nil {
				writeError(w, http.StatusUnauthorized, err.Error())
				return
			}

			ctx := auth.WithIdentity(r.Context(), identity)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.status,
			"duration", time.Since(start).String(),
			"user_id", auth.UserIDFromContext(r.Context()),
			"tenant_id", auth.TenantIDFromContext(r.Context()),
		)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// tenantValidationMiddleware verifies the tenant from the request context is a
// registered slug using the in-memory cache, avoiding a database round-trip on
// every API request. Must be applied after authMiddleware which sets the tenant ID.
func tenantValidationMiddleware(tenants *service.TenantService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := auth.TenantIDFromContext(r.Context())
			if tenantID == "" || !tenants.IsRegistered(tenantID) {
				writeError(w, http.StatusForbidden, "unknown tenant")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// consoleTenantMiddleware reads an optional ?tenant_id= query parameter and
// injects it into the request context. This allows console operators to filter
// data by tenant without requiring an X-Tenant-ID header.
func consoleTenantMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.URL.Query().Get("tenant_id")
		if tenantID != "" {
			ctx := auth.WithTenantID(r.Context(), tenantID)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}
