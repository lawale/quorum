package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/model"
)

func newTestCheckReq() model.PermissionCheckRequest {
	return model.PermissionCheckRequest{
		RequestID:    uuid.New(),
		RequestType:  "transfer",
		CheckerID:    "checker-1",
		CheckerRoles: []string{"admin"},
		MakerID:      "maker-1",
		Payload:      json.RawMessage(`{"amount":100}`),
	}
}

func TestPermissionChecker_Check_Allowed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(model.PermissionCheckResponse{Allowed: true})
	}))
	defer server.Close()

	checker := NewPermissionChecker(5 * time.Second)
	err := checker.Check(context.Background(), server.URL, newTestCheckReq())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestPermissionChecker_Check_Denied(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(model.PermissionCheckResponse{Allowed: false})
	}))
	defer server.Close()

	checker := NewPermissionChecker(5 * time.Second)
	err := checker.Check(context.Background(), server.URL, newTestCheckReq())
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied, got: %v", err)
	}
}

func TestPermissionChecker_Check_DeniedWithReason(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(model.PermissionCheckResponse{
			Allowed: false,
			Reason:  "insufficient seniority",
		})
	}))
	defer server.Close()

	checker := NewPermissionChecker(5 * time.Second)
	err := checker.Check(context.Background(), server.URL, newTestCheckReq())
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied, got: %v", err)
	}
	if err.Error() != "permission denied by external check: insufficient seniority" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestPermissionChecker_Check_NonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	checker := NewPermissionChecker(5 * time.Second)
	err := checker.Check(context.Background(), server.URL, newTestCheckReq())
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestPermissionChecker_Check_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	checker := NewPermissionChecker(5 * time.Second)
	err := checker.Check(context.Background(), server.URL, newTestCheckReq())
	if err == nil {
		t.Fatal("expected error for 500 status")
	}
}

func TestPermissionChecker_Check_InvalidResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	checker := NewPermissionChecker(5 * time.Second)
	err := checker.Check(context.Background(), server.URL, newTestCheckReq())
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

func TestPermissionChecker_Check_NetworkError(t *testing.T) {
	checker := NewPermissionChecker(5 * time.Second)
	err := checker.Check(context.Background(), "http://localhost:1/nonexistent", newTestCheckReq())
	if err == nil {
		t.Fatal("expected error for network failure")
	}
}

func TestPermissionChecker_Check_RequestPayload(t *testing.T) {
	checkReq := newTestCheckReq()

	var received model.PermissionCheckRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		if ua := r.Header.Get("User-Agent"); ua != "Quorum/1.0" {
			t.Errorf("User-Agent = %q, want Quorum/1.0", ua)
		}
		json.NewDecoder(r.Body).Decode(&received)
		json.NewEncoder(w).Encode(model.PermissionCheckResponse{Allowed: true})
	}))
	defer server.Close()

	checker := NewPermissionChecker(5 * time.Second)
	err := checker.Check(context.Background(), server.URL, checkReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if received.RequestID != checkReq.RequestID {
		t.Errorf("RequestID = %v, want %v", received.RequestID, checkReq.RequestID)
	}
	if received.CheckerID != checkReq.CheckerID {
		t.Errorf("CheckerID = %q, want %q", received.CheckerID, checkReq.CheckerID)
	}
	if received.MakerID != checkReq.MakerID {
		t.Errorf("MakerID = %q, want %q", received.MakerID, checkReq.MakerID)
	}
}
