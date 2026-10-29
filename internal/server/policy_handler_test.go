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
	"github.com/lawale/quorum/internal/store"
	"github.com/lawale/quorum/internal/testutil"
)

func newTestPolicyHandler(policies *testutil.MockPolicyStore) *PolicyHandler {
	svc := service.NewPolicyService(policies)
	return NewPolicyHandler(svc)
}

func TestPolicyHandler_Create_Success(t *testing.T) {
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) { return nil, nil },
		CreateFunc:           func(ctx context.Context, policy *model.Policy) error { return nil },
	}
	handler := newTestPolicyHandler(policies)

	body := `{"name":"Test Policy","request_type":"transfer","stages":[{"index":0,"required_approvals":2,"rejection_policy":"any"}]}`
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
}

func TestPolicyHandler_Create_InvalidBody(t *testing.T) {
	handler := newTestPolicyHandler(&testutil.MockPolicyStore{})

	req := httptest.NewRequest("POST", "/", bytes.NewBufferString("not json"))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestPolicyHandler_Create_MissingName(t *testing.T) {
	handler := newTestPolicyHandler(&testutil.MockPolicyStore{})

	body := `{"request_type":"transfer","stages":[{"index":0,"required_approvals":2}]}`
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestPolicyHandler_Create_MissingRequestType(t *testing.T) {
	handler := newTestPolicyHandler(&testutil.MockPolicyStore{})

	body := `{"name":"Test Policy","stages":[{"index":0,"required_approvals":2}]}`
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestPolicyHandler_Create_MissingStages(t *testing.T) {
	handler := newTestPolicyHandler(&testutil.MockPolicyStore{})

	body := `{"name":"Test Policy","request_type":"transfer"}`
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestPolicyHandler_Create_TypeConflict(t *testing.T) {
	existing := testutil.NewPolicy()
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) { return existing, nil },
	}
	handler := newTestPolicyHandler(policies)

	body := `{"name":"Test Policy","request_type":"transfer","stages":[{"index":0,"required_approvals":2}]}`
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestPolicyHandler_Create_InvalidAutoExpire(t *testing.T) {
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) { return nil, nil },
	}
	handler := newTestPolicyHandler(policies)

	body := `{"name":"Test","request_type":"transfer","stages":[{"index":0,"required_approvals":1}],"auto_expire_duration":"invalid"}`
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestPolicyHandler_Create_WithDynamicAuthorizationURL(t *testing.T) {
	var createdPolicy *model.Policy
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) { return nil, nil },
		CreateFunc: func(ctx context.Context, policy *model.Policy) error {
			createdPolicy = policy
			return nil
		},
	}
	handler := newTestPolicyHandler(policies)

	body := `{"name":"Test","request_type":"transfer","stages":[{"index":0,"required_approvals":1}],"dynamic_authorization_url":"https://example.com/check"}`
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if createdPolicy.DynamicAuthorizationURL == nil || *createdPolicy.DynamicAuthorizationURL != "https://example.com/check" {
		t.Errorf("DynamicAuthorizationURL = %v, want https://example.com/check", createdPolicy.DynamicAuthorizationURL)
	}
}

func TestPolicyHandler_Create_WithDynamicAuthorizationSecret(t *testing.T) {
	var createdPolicy *model.Policy
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) { return nil, nil },
		CreateFunc: func(ctx context.Context, policy *model.Policy) error {
			createdPolicy = policy
			return nil
		},
	}
	handler := newTestPolicyHandler(policies)

	body := `{"name":"Test","request_type":"transfer","stages":[{"index":0,"required_approvals":1}],"dynamic_authorization_url":"https://example.com/check","dynamic_authorization_secret":"my-secret"}`
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if createdPolicy.DynamicAuthorizationSecret == nil || *createdPolicy.DynamicAuthorizationSecret != "my-secret" {
		t.Errorf("DynamicAuthorizationSecret = %v, want my-secret", createdPolicy.DynamicAuthorizationSecret)
	}
}

func TestPolicyHandler_Get_Success(t *testing.T) {
	policyID := uuid.New()
	expected := testutil.NewPolicy(func(p *model.Policy) { p.ID = policyID })
	policies := &testutil.MockPolicyStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Policy, error) { return expected, nil },
	}
	handler := newTestPolicyHandler(policies)

	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(chiContext(req.Context(), "id", policyID.String()))
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestPolicyHandler_Get_InvalidID(t *testing.T) {
	handler := newTestPolicyHandler(&testutil.MockPolicyStore{})

	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(chiContext(req.Context(), "id", "not-a-uuid"))
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestPolicyHandler_Get_NotFound(t *testing.T) {
	policies := &testutil.MockPolicyStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Policy, error) { return nil, nil },
	}
	handler := newTestPolicyHandler(policies)

	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(chiContext(req.Context(), "id", uuid.New().String()))
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestPolicyHandler_List_Success(t *testing.T) {
	policies := &testutil.MockPolicyStore{
		ListFunc: func(ctx context.Context, filter store.PolicyFilter) ([]model.Policy, int, error) {
			return []model.Policy{*testutil.NewPolicy()}, 1, nil
		},
	}
	handler := newTestPolicyHandler(policies)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]json.RawMessage
	json.NewDecoder(rec.Body).Decode(&body)
	if body["data"] == nil {
		t.Error("expected 'data' field in response")
	}
}

func TestPolicyHandler_Update_Success(t *testing.T) {
	policyID := uuid.New()
	existing := testutil.NewPolicy(func(p *model.Policy) { p.ID = policyID })
	policies := &testutil.MockPolicyStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Policy, error) { return existing, nil },
		UpdateFunc:  func(ctx context.Context, policy *model.Policy) error { return nil },
	}
	handler := newTestPolicyHandler(policies)

	body := `{"name":"Updated Name"}`
	req := httptest.NewRequest("PUT", "/", bytes.NewBufferString(body))
	req = req.WithContext(chiContext(req.Context(), "id", policyID.String()))
	rec := httptest.NewRecorder()

	handler.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestPolicyHandler_Update_NotFound(t *testing.T) {
	policies := &testutil.MockPolicyStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Policy, error) { return nil, nil },
	}
	handler := newTestPolicyHandler(policies)

	body := `{"name":"Updated Name"}`
	req := httptest.NewRequest("PUT", "/", bytes.NewBufferString(body))
	req = req.WithContext(chiContext(req.Context(), "id", uuid.New().String()))
	rec := httptest.NewRecorder()

	handler.Update(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestPolicyHandler_Delete_Success(t *testing.T) {
	policyID := uuid.New()
	existing := testutil.NewPolicy(func(p *model.Policy) { p.ID = policyID })
	policies := &testutil.MockPolicyStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Policy, error) { return existing, nil },
		DeleteFunc:  func(ctx context.Context, id uuid.UUID) error { return nil },
	}
	handler := newTestPolicyHandler(policies)

	req := httptest.NewRequest("DELETE", "/", nil)
	req = req.WithContext(chiContext(req.Context(), "id", policyID.String()))
	rec := httptest.NewRecorder()

	handler.Delete(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestPolicyHandler_Delete_NotFound(t *testing.T) {
	policies := &testutil.MockPolicyStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Policy, error) { return nil, nil },
	}
	handler := newTestPolicyHandler(policies)

	req := httptest.NewRequest("DELETE", "/", nil)
	req = req.WithContext(chiContext(req.Context(), "id", uuid.New().String()))
	rec := httptest.NewRecorder()

	handler.Delete(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestPolicyHandler_Delete_InvalidID(t *testing.T) {
	handler := newTestPolicyHandler(&testutil.MockPolicyStore{})

	req := httptest.NewRequest("DELETE", "/", nil)
	req = req.WithContext(chiContext(req.Context(), "id", "not-a-uuid"))
	rec := httptest.NewRecorder()

	handler.Delete(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestPolicyHandler_Create_CarriesScopeRules(t *testing.T) {
	var stored *model.Policy
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) { return nil, nil },
		CreateFunc:           func(ctx context.Context, policy *model.Policy) error { stored = policy; return nil },
	}
	handler := newTestPolicyHandler(policies)

	body := `{"name":"Scoped","request_type":"transfer","stages":[{"index":0,"required_approvals":1,"rejection_policy":"any",
		"scope":[{"attribute":"branch_id","payload_path":"destination_branch_id","values":["HQ"],"allow_unscoped_checkers":true}]}]}`
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if stored == nil || len(stored.Stages[0].Scope) != 1 {
		t.Fatalf("scope rules not carried through handler: %+v", stored)
	}
	rule := stored.Stages[0].Scope[0]
	if rule.Attribute != "branch_id" || rule.PayloadPath != "destination_branch_id" || len(rule.Values) != 1 || rule.Values[0] != "HQ" || !rule.AllowUnscopedCheckers {
		t.Errorf("rule = %+v", rule)
	}
}

func TestPolicyHandler_Update_CarriesScopeRules(t *testing.T) {
	existing := testutil.NewPolicy()
	var stored *model.Policy
	policies := &testutil.MockPolicyStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Policy, error) { return existing, nil },
		UpdateFunc:  func(ctx context.Context, policy *model.Policy) error { stored = policy; return nil },
	}
	handler := newTestPolicyHandler(policies)

	body := `{"stages":[{"index":0,"required_approvals":1,"rejection_policy":"any","scope":[{"attribute":"branch_id"}]}]}`
	req := httptest.NewRequest("PUT", "/"+existing.ID.String(), bytes.NewBufferString(body))
	req = withURLParam(req, "id", existing.ID.String())
	rec := httptest.NewRecorder()

	handler.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if stored == nil || len(stored.Stages[0].Scope) != 1 || stored.Stages[0].Scope[0].Attribute != "branch_id" {
		t.Fatalf("scope rules not carried through update handler: %+v", stored)
	}
	if !stored.Stages[0].Scope[0].MatchMaker {
		t.Error("rule with no source should have been defaulted to match_maker")
	}
}

func withURLParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}
