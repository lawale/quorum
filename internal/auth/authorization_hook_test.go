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

func newTestHookReq() model.AuthorizationHookRequest {
	return model.AuthorizationHookRequest{
		RequestID:   uuid.New(),
		RequestType: "transfer",
		CheckerID:   "checker-1",
		MakerID:     "maker-1",
		Payload:     json.RawMessage(`{"amount":100}`),
	}
}

func TestAuthorizationHook_Check_Allowed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(model.AuthorizationHookResponse{Allowed: true})
	}))
	defer server.Close()

	hook := NewAuthorizationHook(5 * time.Second)
	err := hook.Check(context.Background(), server.URL, newTestHookReq())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestAuthorizationHook_Check_Denied(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(model.AuthorizationHookResponse{Allowed: false})
	}))
	defer server.Close()

	hook := NewAuthorizationHook(5 * time.Second)
	err := hook.Check(context.Background(), server.URL, newTestHookReq())
	if !errors.Is(err, ErrAuthorizationDenied) {
		t.Fatalf("expected ErrAuthorizationDenied, got: %v", err)
	}
}

func TestAuthorizationHook_Check_DeniedWithReason(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(model.AuthorizationHookResponse{
			Allowed: false,
			Reason:  "insufficient seniority",
		})
	}))
	defer server.Close()

	hook := NewAuthorizationHook(5 * time.Second)
	err := hook.Check(context.Background(), server.URL, newTestHookReq())
	if !errors.Is(err, ErrAuthorizationDenied) {
		t.Fatalf("expected ErrAuthorizationDenied, got: %v", err)
	}
	if err.Error() != "authorization denied by dynamic hook: insufficient seniority" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestAuthorizationHook_Check_NonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	hook := NewAuthorizationHook(5 * time.Second)
	err := hook.Check(context.Background(), server.URL, newTestHookReq())
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestAuthorizationHook_Check_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	hook := NewAuthorizationHook(5 * time.Second)
	err := hook.Check(context.Background(), server.URL, newTestHookReq())
	if err == nil {
		t.Fatal("expected error for 500 status")
	}
}

func TestAuthorizationHook_Check_InvalidResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	hook := NewAuthorizationHook(5 * time.Second)
	err := hook.Check(context.Background(), server.URL, newTestHookReq())
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

func TestAuthorizationHook_Check_NetworkError(t *testing.T) {
	hook := NewAuthorizationHook(5 * time.Second)
	err := hook.Check(context.Background(), "http://localhost:1/nonexistent", newTestHookReq())
	if err == nil {
		t.Fatal("expected error for network failure")
	}
}

func TestAuthorizationHook_Check_RequestPayload(t *testing.T) {
	hookReq := newTestHookReq()

	var received model.AuthorizationHookRequest
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
		json.NewEncoder(w).Encode(model.AuthorizationHookResponse{Allowed: true})
	}))
	defer server.Close()

	hook := NewAuthorizationHook(5 * time.Second)
	err := hook.Check(context.Background(), server.URL, hookReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if received.RequestID != hookReq.RequestID {
		t.Errorf("RequestID = %v, want %v", received.RequestID, hookReq.RequestID)
	}
	if received.CheckerID != hookReq.CheckerID {
		t.Errorf("CheckerID = %q, want %q", received.CheckerID, hookReq.CheckerID)
	}
	if received.MakerID != hookReq.MakerID {
		t.Errorf("MakerID = %q, want %q", received.MakerID, hookReq.MakerID)
	}
}
