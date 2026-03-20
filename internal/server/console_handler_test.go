package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/service"
	storepkg "github.com/lawale/quorum/internal/store"
	"github.com/lawale/quorum/internal/testutil"
	"golang.org/x/crypto/bcrypt"
)

func setupOperatorService(store *testutil.MockOperatorStore) *service.OperatorService {
	return service.NewOperatorService(store, "test-secret-key-for-jwt-min-32b!")
}

func TestConsoleHandler_Setup_Success(t *testing.T) {
	store := &testutil.MockOperatorStore{
		CountFunc: func(ctx context.Context) (int, error) { return 0, nil },
		CreateFunc: func(ctx context.Context, op *model.Operator) error {
			op.ID = uuid.New()
			return nil
		},
	}
	svc := setupOperatorService(store)
	handler := NewConsoleHandler(svc, nil, false, "", "")

	body := `{"username":"admin","password":"password123","display_name":"Admin"}`
	req := httptest.NewRequest("POST", "/api/v1/console/auth/setup", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.Setup(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["token"] == nil || resp["token"] == "" {
		t.Error("expected non-empty token in response")
	}
	if resp["operator"] == nil {
		t.Error("expected operator in response")
	}
}

func TestConsoleHandler_Setup_AlreadyDone(t *testing.T) {
	store := &testutil.MockOperatorStore{
		CountFunc: func(ctx context.Context) (int, error) { return 1, nil },
	}
	svc := setupOperatorService(store)
	handler := NewConsoleHandler(svc, nil, false, "", "")

	body := `{"username":"admin","password":"password123"}`
	req := httptest.NewRequest("POST", "/api/v1/console/auth/setup", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.Setup(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestConsoleHandler_Setup_MissingFields(t *testing.T) {
	store := &testutil.MockOperatorStore{}
	svc := setupOperatorService(store)
	handler := NewConsoleHandler(svc, nil, false, "", "")

	body := `{"username":"admin"}`
	req := httptest.NewRequest("POST", "/api/v1/console/auth/setup", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.Setup(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestConsoleHandler_Login_Success(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	store := &testutil.MockOperatorStore{
		GetByUsernameFunc: func(ctx context.Context, username string) (*model.Operator, error) {
			return &model.Operator{
				ID:           uuid.New(),
				Username:     "admin",
				PasswordHash: string(hash),
				DisplayName:  "Admin",
			}, nil
		},
	}
	svc := setupOperatorService(store)
	handler := NewConsoleHandler(svc, nil, false, "", "")

	body := `{"username":"admin","password":"password123"}`
	req := httptest.NewRequest("POST", "/api/v1/console/auth/login", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["token"] == nil || resp["token"] == "" {
		t.Error("expected non-empty token in response")
	}
}

func TestConsoleHandler_Login_InvalidCredentials(t *testing.T) {
	store := &testutil.MockOperatorStore{
		GetByUsernameFunc: func(ctx context.Context, username string) (*model.Operator, error) {
			return nil, nil // user not found
		},
	}
	svc := setupOperatorService(store)
	handler := NewConsoleHandler(svc, nil, false, "", "")

	body := `{"username":"nobody","password":"wrong"}`
	req := httptest.NewRequest("POST", "/api/v1/console/auth/login", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestConsoleHandler_NeedsSetup(t *testing.T) {
	store := &testutil.MockOperatorStore{
		CountFunc: func(ctx context.Context) (int, error) { return 0, nil },
	}
	svc := setupOperatorService(store)
	handler := NewConsoleHandler(svc, nil, false, "", "")

	req := httptest.NewRequest("GET", "/api/v1/console/auth/status", nil)
	rec := httptest.NewRecorder()

	handler.NeedsSetup(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["needs_setup"] != true {
		t.Error("expected needs_setup=true")
	}
}

func TestConsoleHandler_Me(t *testing.T) {
	opID := uuid.New()
	store := &testutil.MockOperatorStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Operator, error) {
			return &model.Operator{
				ID:          opID,
				Username:    "admin",
				DisplayName: "Admin User",
			}, nil
		},
	}
	svc := setupOperatorService(store)
	handler := NewConsoleHandler(svc, nil, false, "", "")

	req := httptest.NewRequest("GET", "/api/v1/console/me", nil)
	ctx := context.WithValue(req.Context(), operatorIDCtxKey, opID)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.Me(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestConsoleHandler_Me_Unauthenticated(t *testing.T) {
	store := &testutil.MockOperatorStore{}
	svc := setupOperatorService(store)
	handler := NewConsoleHandler(svc, nil, false, "", "")

	req := httptest.NewRequest("GET", "/api/v1/console/me", nil)
	rec := httptest.NewRecorder()

	handler.Me(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestConsoleHandler_CreateOperator_Success(t *testing.T) {
	store := &testutil.MockOperatorStore{
		GetByUsernameFunc: func(ctx context.Context, username string) (*model.Operator, error) {
			return nil, nil
		},
		CreateFunc: func(ctx context.Context, op *model.Operator) error {
			op.ID = uuid.New()
			return nil
		},
	}
	svc := setupOperatorService(store)
	handler := NewConsoleHandler(svc, nil, false, "", "")

	body := `{"username":"newuser","password":"pass123","display_name":"New User"}`
	req := httptest.NewRequest("POST", "/api/v1/console/operators", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.CreateOperator(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
}

func TestConsoleHandler_CreateOperator_UsernameConflict(t *testing.T) {
	store := &testutil.MockOperatorStore{
		GetByUsernameFunc: func(ctx context.Context, username string) (*model.Operator, error) {
			return &model.Operator{Username: "existing"}, nil
		},
	}
	svc := setupOperatorService(store)
	handler := NewConsoleHandler(svc, nil, false, "", "")

	body := `{"username":"existing","password":"pass123"}`
	req := httptest.NewRequest("POST", "/api/v1/console/operators", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.CreateOperator(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestConsoleHandler_DeleteOperator_Success(t *testing.T) {
	callerID := uuid.New()
	targetID := uuid.New()

	store := &testutil.MockOperatorStore{
		CountFunc: func(ctx context.Context) (int, error) { return 2, nil },
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Operator, error) {
			return &model.Operator{ID: targetID, Username: "target"}, nil
		},
		DeleteFunc: func(ctx context.Context, id uuid.UUID) error { return nil },
	}
	svc := setupOperatorService(store)
	handler := NewConsoleHandler(svc, nil, false, "", "")

	req := httptest.NewRequest("DELETE", "/api/v1/console/operators/"+targetID.String(), nil)
	ctx := context.WithValue(req.Context(), operatorIDCtxKey, callerID)

	// Set chi URL param
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", targetID.String())
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)

	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.DeleteOperator(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestConsoleHandler_DeleteOperator_CannotDeleteSelf(t *testing.T) {
	selfID := uuid.New()

	store := &testutil.MockOperatorStore{}
	svc := setupOperatorService(store)
	handler := NewConsoleHandler(svc, nil, false, "", "")

	req := httptest.NewRequest("DELETE", "/api/v1/console/operators/"+selfID.String(), nil)
	ctx := context.WithValue(req.Context(), operatorIDCtxKey, selfID)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", selfID.String())
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)

	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.DeleteOperator(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestConsoleHandler_ListOperators(t *testing.T) {
	store := &testutil.MockOperatorStore{
		ListFunc: func(ctx context.Context, filter storepkg.OperatorFilter) ([]model.Operator, int, error) {
			return []model.Operator{
				{ID: uuid.New(), Username: "admin", DisplayName: "Admin"},
				{ID: uuid.New(), Username: "dev", DisplayName: "Dev"},
			}, 2, nil
		},
	}
	svc := setupOperatorService(store)
	handler := NewConsoleHandler(svc, nil, false, "", "")

	req := httptest.NewRequest("GET", "/api/v1/console/operators", nil)
	rec := httptest.NewRecorder()

	handler.ListOperators(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	data, ok := resp["data"].([]any)
	if !ok || len(data) != 2 {
		t.Errorf("expected 2 operators in response, got %v", resp["data"])
	}
}

func TestConsoleHandler_ChangePassword_Success(t *testing.T) {
	opID := uuid.New()
	currentHash, _ := bcrypt.GenerateFromPassword([]byte("old-pass"), bcrypt.DefaultCost)

	store := &testutil.MockOperatorStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Operator, error) {
			return &model.Operator{
				ID:                 opID,
				Username:           "admin",
				PasswordHash:       string(currentHash),
				MustChangePassword: true,
			}, nil
		},
		UpdateFunc: func(ctx context.Context, op *model.Operator) error { return nil },
	}
	svc := setupOperatorService(store)
	handler := NewConsoleHandler(svc, nil, false, "", "")

	body := `{"current_password":"old-pass","new_password":"new-pass"}`
	req := httptest.NewRequest("PUT", "/api/v1/console/me/password", bytes.NewBufferString(body))
	ctx := context.WithValue(req.Context(), operatorIDCtxKey, opID)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ChangePassword(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestConsoleHandler_ChangePassword_WrongCurrent(t *testing.T) {
	opID := uuid.New()
	currentHash, _ := bcrypt.GenerateFromPassword([]byte("correct-pass"), bcrypt.DefaultCost)

	store := &testutil.MockOperatorStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Operator, error) {
			return &model.Operator{
				ID:           opID,
				PasswordHash: string(currentHash),
			}, nil
		},
	}
	svc := setupOperatorService(store)
	handler := NewConsoleHandler(svc, nil, false, "", "")

	body := `{"current_password":"wrong-pass","new_password":"new-pass"}`
	req := httptest.NewRequest("PUT", "/api/v1/console/me/password", bytes.NewBufferString(body))
	ctx := context.WithValue(req.Context(), operatorIDCtxKey, opID)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ChangePassword(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
