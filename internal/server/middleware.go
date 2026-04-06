package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/lawale/quorum/internal/auth"
	"github.com/lawale/quorum/internal/logging"
	"github.com/lawale/quorum/internal/service"
)

// corsMiddleware adds permissive CORS headers so the embeddable approval
// widget can call the Quorum API from any consumer-app origin.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
		}

		// Handle preflight
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-User-ID, X-User-Id, X-User-Roles, X-User-Permissions, X-Tenant-ID")
			w.Header().Set("Access-Control-Max-Age", "86400")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

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

		// Inject request ID into logging context so all downstream slog
		// calls automatically include it. User ID and tenant ID are pulled
		// from context dynamically by the ContextHandler extractors, so they
		// appear even though auth middleware runs after this point.
		ctx := logging.WithAttrs(r.Context(), logging.ContextAttrs{
			RequestID: middleware.GetReqID(r.Context()),
		})
		r = r.WithContext(ctx)

		slog.InfoContext(ctx, "request started",
			"method", r.Method,
			"path", r.URL.Path,
		)

		wrapped := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		level := slog.LevelInfo
		if wrapped.status >= 500 {
			level = slog.LevelError
		} else if wrapped.status >= 400 {
			level = slog.LevelWarn
		}

		// Use r.Context() here (not the original ctx) because auth
		// middleware may have enriched it with user_id and tenant_id.
		slog.Log(r.Context(), level, "request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.status,
			"duration_ms", float64(duration.Microseconds())/1000.0,
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

// Flush implements http.Flusher by delegating to the wrapped ResponseWriter
// if it supports flushing (e.g. for streaming or SSE).
func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap returns the underlying ResponseWriter so that middleware further
// up the chain can access optional interfaces via http.ResponseController.
func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v) //nolint:errcheck // response already committed
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// writeServerError logs the actual error with request context before returning
// a generic error message to the client. Use this instead of writeError for all
// 5xx responses so the root cause is always visible in logs.
//
// The request_id, user_id, and tenant_id are automatically included via the
// context-aware slog handler — no need to pass them explicitly.
func writeServerError(w http.ResponseWriter, r *http.Request, err error, message string) {
	slog.ErrorContext(r.Context(), message,
		"error", err,
	)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": message})
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
// data by tenant without requiring an X-Tenant-ID header. If the tenant is
// provided, it is validated against the registered tenant cache.
func consoleTenantMiddleware(tenants *service.TenantService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := r.URL.Query().Get("tenant_id")
			if tenantID != "" {
				if tenants != nil && !tenants.IsRegistered(tenantID) {
					writeError(w, http.StatusBadRequest, "unknown tenant: "+tenantID)
					return
				}
				ctx := auth.WithTenantID(r.Context(), tenantID)
				r = r.WithContext(ctx)
			}
			next.ServeHTTP(w, r)
		})
	}
}
