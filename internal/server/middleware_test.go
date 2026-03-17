package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/lawale/quorum/internal/auth"
	"github.com/lawale/quorum/internal/logging"
	"github.com/lawale/quorum/internal/testutil"
)

func TestAuthMiddleware_Success(t *testing.T) {
	provider := &testutil.MockAuthProvider{
		AuthenticateFunc: func(r *http.Request) (*auth.Identity, error) {
			return &auth.Identity{UserID: "user-1", Roles: []string{"admin"}}, nil
		},
	}

	var capturedUserID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserID = auth.UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := authMiddleware(provider)(next)
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if capturedUserID != "user-1" {
		t.Errorf("UserID = %q, want %q", capturedUserID, "user-1")
	}
}

func TestAuthMiddleware_Failure(t *testing.T) {
	provider := &testutil.MockAuthProvider{
		AuthenticateFunc: func(r *http.Request) (*auth.Identity, error) {
			return nil, errors.New("unauthorized")
		},
	}

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	handler := authMiddleware(provider)(next)
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if nextCalled {
		t.Error("next handler should not be called on auth failure")
	}
}

func TestStatusWriter_CapturesStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rec, status: http.StatusOK}

	sw.WriteHeader(http.StatusNotFound)
	if sw.status != http.StatusNotFound {
		t.Errorf("status = %d, want %d", sw.status, http.StatusNotFound)
	}
}

func TestWriteJSON_ContentType(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, map[string]string{"key": "value"})

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestWriteJSON_Status(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusCreated, map[string]string{"key": "value"})

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
}

func TestWriteError_Format(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusBadRequest, "something went wrong")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)
	if body["error"] != "something went wrong" {
		t.Errorf("error = %q, want %q", body["error"], "something went wrong")
	}
}

func TestLoggingMiddleware_LogsStartAndEnd(t *testing.T) {
	var buf bytes.Buffer
	origLogger := slog.Default()
	handler := logging.NewContextHandler(slog.NewJSONHandler(&buf, nil))
	slog.SetDefault(slog.New(handler))
	defer slog.SetDefault(origLogger)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	chain := middleware.RequestID(loggingMiddleware(next))
	req := httptest.NewRequest("GET", "/api/policies", nil)
	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, req)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 log lines, got %d: %s", len(lines), buf.String())
	}

	var startEntry, endEntry map[string]any
	json.Unmarshal([]byte(lines[0]), &startEntry)
	json.Unmarshal([]byte(lines[1]), &endEntry)

	if startEntry["msg"] != "request started" {
		t.Errorf("first log msg = %v, want request started", startEntry["msg"])
	}
	if startEntry["method"] != "GET" {
		t.Errorf("first log method = %v, want GET", startEntry["method"])
	}
	if startEntry["path"] != "/api/policies" {
		t.Errorf("first log path = %v, want /api/policies", startEntry["path"])
	}
	if startEntry["request_id"] == nil || startEntry["request_id"] == "" {
		t.Error("first log should have a request_id")
	}

	if endEntry["msg"] != "request completed" {
		t.Errorf("second log msg = %v, want request completed", endEntry["msg"])
	}
	if endEntry["status"] != float64(200) {
		t.Errorf("second log status = %v, want 200", endEntry["status"])
	}
	if endEntry["duration_ms"] == nil {
		t.Error("second log should have duration_ms")
	}
	if endEntry["request_id"] != startEntry["request_id"] {
		t.Errorf("request_id mismatch: start=%v end=%v", startEntry["request_id"], endEntry["request_id"])
	}
}

func TestLoggingMiddleware_ErrorStatusLogsAtErrorLevel(t *testing.T) {
	var buf bytes.Buffer
	origLogger := slog.Default()
	handler := logging.NewContextHandler(slog.NewJSONHandler(&buf, nil))
	slog.SetDefault(slog.New(handler))
	defer slog.SetDefault(origLogger)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	chain := middleware.RequestID(loggingMiddleware(next))
	req := httptest.NewRequest("GET", "/fail", nil)
	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, req)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 log lines, got %d", len(lines))
	}

	var endEntry map[string]any
	json.Unmarshal([]byte(lines[len(lines)-1]), &endEntry)

	if endEntry["level"] != "ERROR" {
		t.Errorf("level = %v, want ERROR for 5xx status", endEntry["level"])
	}
}

func TestLoggingMiddleware_CorrelatesRequestID(t *testing.T) {
	var buf bytes.Buffer
	origLogger := slog.Default()
	handler := logging.NewContextHandler(slog.NewJSONHandler(&buf, nil))
	slog.SetDefault(slog.New(handler))
	defer slog.SetDefault(origLogger)

	var capturedRequestID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the context has logging attrs set for downstream slog calls
		attrs := logging.AttrsFromContext(r.Context())
		capturedRequestID = attrs.RequestID
		w.WriteHeader(http.StatusOK)
	})

	chain := middleware.RequestID(loggingMiddleware(next))
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, req)

	if capturedRequestID == "" {
		t.Error("handler should have a request_id in logging context")
	}
}

func TestIntParam_Default(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	got := intParam(req, "page", 1)
	if got != 1 {
		t.Errorf("intParam() = %d, want 1 (default)", got)
	}
}

func TestIntParam_Valid(t *testing.T) {
	req := httptest.NewRequest("GET", "/?page=5", nil)
	got := intParam(req, "page", 1)
	if got != 5 {
		t.Errorf("intParam() = %d, want 5", got)
	}
}

func TestIntParam_Invalid(t *testing.T) {
	req := httptest.NewRequest("GET", "/?page=abc", nil)
	got := intParam(req, "page", 1)
	if got != 1 {
		t.Errorf("intParam() = %d, want 1 (default for invalid)", got)
	}
}
