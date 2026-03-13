package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/auth"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/service"
	"github.com/lawale/quorum/internal/testutil"
)

func issueTestToken(t *testing.T) (*service.OperatorService, string) {
	t.Helper()
	store := &testutil.MockOperatorStore{
		CountFunc:  func(ctx context.Context) (int, error) { return 0, nil },
		CreateFunc: func(ctx context.Context, op *model.Operator) error { op.ID = uuid.New(); return nil },
	}
	svc := service.NewOperatorService(store, "test-jwt-secret-for-middleware")
	_, token, err := svc.Setup(context.Background(), "admin", "pass", "Admin")
	if err != nil {
		t.Fatalf("issuing test token: %v", err)
	}
	return svc, token
}

func TestConsoleJWTMiddleware_Success(t *testing.T) {
	svc, token := issueTestToken(t)

	var capturedUserID string
	var capturedOperatorID uuid.UUID
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserID = auth.UserIDFromContext(r.Context())
		capturedOperatorID = operatorIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := consoleJWTMiddleware(svc)(next)

	req := httptest.NewRequest("GET", "/api/v1/console/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if capturedUserID != "admin" {
		t.Errorf("UserID = %q, want %q", capturedUserID, "admin")
	}
	if capturedOperatorID == uuid.Nil {
		t.Error("expected non-nil operator ID in context")
	}
}

func TestConsoleJWTMiddleware_MissingHeader(t *testing.T) {
	svc, _ := issueTestToken(t)

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	handler := consoleJWTMiddleware(svc)(next)

	req := httptest.NewRequest("GET", "/api/v1/console/me", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if nextCalled {
		t.Error("next handler should not be called on auth failure")
	}
}

func TestConsoleJWTMiddleware_InvalidFormat(t *testing.T) {
	svc, _ := issueTestToken(t)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	handler := consoleJWTMiddleware(svc)(next)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestConsoleJWTMiddleware_InvalidToken(t *testing.T) {
	svc, _ := issueTestToken(t)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	handler := consoleJWTMiddleware(svc)(next)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer invalid-token-here")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestConsoleJWTMiddleware_WrongSecret(t *testing.T) {
	// Issue token with one secret, validate with another
	store := &testutil.MockOperatorStore{
		CountFunc:  func(ctx context.Context) (int, error) { return 0, nil },
		CreateFunc: func(ctx context.Context, op *model.Operator) error { op.ID = uuid.New(); return nil },
	}
	svc1 := service.NewOperatorService(store, "secret-1")
	_, token, _ := svc1.Setup(context.Background(), "admin", "pass", "Admin")

	store.CountFunc = func(ctx context.Context) (int, error) { return 0, nil }
	store.CreateFunc = func(ctx context.Context, op *model.Operator) error { op.ID = uuid.New(); return nil }
	svc2 := service.NewOperatorService(store, "secret-2")

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	handler := consoleJWTMiddleware(svc2)(next)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestOperatorIDFromContext_NilContext(t *testing.T) {
	ctx := context.Background()
	id := operatorIDFromContext(ctx)
	if id != uuid.Nil {
		t.Errorf("expected uuid.Nil, got %v", id)
	}
}
