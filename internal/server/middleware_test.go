package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lawale/quorum/internal/auth"
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
