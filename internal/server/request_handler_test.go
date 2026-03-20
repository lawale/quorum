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
	"github.com/lawale/quorum/internal/auth"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/service"
	"github.com/lawale/quorum/internal/store"
	"github.com/lawale/quorum/internal/testutil"
)

// chiContext returns a context with a chi URL param set.
func chiContext(ctx context.Context, key, value string) context.Context {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return context.WithValue(ctx, chi.RouteCtxKey, rctx)
}

// newRequestHandler creates a RequestHandler backed by mock stores.
func newTestRequestHandler(
	requests *testutil.MockRequestStore,
	approvals *testutil.MockApprovalStore,
	policies *testutil.MockPolicyStore,
	audits *testutil.MockAuditStore,
) *RequestHandler {
	svc := service.NewRequestService(requests, approvals, policies, audits, nil)
	return NewRequestHandler(svc)
}

func TestRequestHandler_Create_Success(t *testing.T) {
	policy := testutil.NewPolicy()
	requests := &testutil.MockRequestStore{
		CreateFunc: func(ctx context.Context, req *model.Request) error { return nil },
	}
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) { return policy, nil },
	}
	audits := &testutil.MockAuditStore{
		CreateFunc: func(ctx context.Context, log *model.AuditLog) error { return nil },
	}
	handler := newTestRequestHandler(requests, &testutil.MockApprovalStore{}, policies, audits)

	body := `{"type":"transfer","payload":{"amount":100}}`
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
	req = req.WithContext(auth.WithIdentity(req.Context(), &auth.Identity{UserID: "maker-1"}))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
}

func TestRequestHandler_Create_InvalidBody(t *testing.T) {
	handler := newTestRequestHandler(&testutil.MockRequestStore{}, &testutil.MockApprovalStore{}, &testutil.MockPolicyStore{}, &testutil.MockAuditStore{})

	req := httptest.NewRequest("POST", "/", bytes.NewBufferString("not json"))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRequestHandler_Create_MissingType(t *testing.T) {
	handler := newTestRequestHandler(&testutil.MockRequestStore{}, &testutil.MockApprovalStore{}, &testutil.MockPolicyStore{}, &testutil.MockAuditStore{})

	body := `{"payload":{"amount":100}}`
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRequestHandler_Create_MissingPayload(t *testing.T) {
	handler := newTestRequestHandler(&testutil.MockRequestStore{}, &testutil.MockApprovalStore{}, &testutil.MockPolicyStore{}, &testutil.MockAuditStore{})

	body := `{"type":"transfer"}`
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRequestHandler_Create_PolicyNotFound(t *testing.T) {
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) { return nil, nil },
	}
	handler := newTestRequestHandler(&testutil.MockRequestStore{}, &testutil.MockApprovalStore{}, policies, &testutil.MockAuditStore{})

	body := `{"type":"transfer","payload":{"amount":100}}`
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
	req = req.WithContext(auth.WithIdentity(req.Context(), &auth.Identity{UserID: "maker-1"}))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRequestHandler_Create_DuplicateRequest(t *testing.T) {
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.IdentityFields = []string{"account_id"}
	})
	requests := &testutil.MockRequestStore{
		FindPendingByFingerprintFunc: func(ctx context.Context, rt, fp string) (*model.Request, error) {
			return testutil.NewRequest(), nil // Duplicate found
		},
	}
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) { return policy, nil },
	}
	handler := newTestRequestHandler(requests, &testutil.MockApprovalStore{}, policies, &testutil.MockAuditStore{})

	body := `{"type":"transfer","payload":{"account_id":"acc-1"}}`
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
	req = req.WithContext(auth.WithIdentity(req.Context(), &auth.Identity{UserID: "maker-1"}))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestRequestHandler_Create_WithEligibleReviewers(t *testing.T) {
	policy := testutil.NewPolicy()
	var createdReq *model.Request
	requests := &testutil.MockRequestStore{
		CreateFunc: func(ctx context.Context, req *model.Request) error {
			createdReq = req
			return nil
		},
	}
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) { return policy, nil },
	}
	audits := &testutil.MockAuditStore{
		CreateFunc: func(ctx context.Context, log *model.AuditLog) error { return nil },
	}
	handler := newTestRequestHandler(requests, &testutil.MockApprovalStore{}, policies, audits)

	body := `{"type":"transfer","payload":{"amount":100},"eligible_reviewers":["reviewer-1","reviewer-2"]}`
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
	req = req.WithContext(auth.WithIdentity(req.Context(), &auth.Identity{UserID: "maker-1"}))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if len(createdReq.EligibleReviewers) != 2 {
		t.Errorf("EligibleReviewers = %v, want 2 items", createdReq.EligibleReviewers)
	}
}

func TestRequestHandler_Create_IdempotencyKeyHeader(t *testing.T) {
	policy := testutil.NewPolicy()
	var createdReq *model.Request
	requests := &testutil.MockRequestStore{
		GetByIdempotencyKeyFunc: func(ctx context.Context, key string) (*model.Request, error) { return nil, nil },
		CreateFunc: func(ctx context.Context, req *model.Request) error {
			createdReq = req
			return nil
		},
	}
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) { return policy, nil },
	}
	audits := &testutil.MockAuditStore{
		CreateFunc: func(ctx context.Context, log *model.AuditLog) error { return nil },
	}
	handler := newTestRequestHandler(requests, &testutil.MockApprovalStore{}, policies, audits)

	body := `{"type":"transfer","payload":{"amount":100}}`
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
	req.Header.Set("Idempotency-Key", "test-key-123")
	req = req.WithContext(auth.WithIdentity(req.Context(), &auth.Identity{UserID: "maker-1"}))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if createdReq.IdempotencyKey == nil || *createdReq.IdempotencyKey != "test-key-123" {
		t.Errorf("IdempotencyKey = %v, want test-key-123", createdReq.IdempotencyKey)
	}
}

func TestRequestHandler_Get_Success(t *testing.T) {
	reqID := uuid.New()
	expected := testutil.NewRequest(func(r *model.Request) { r.ID = reqID })
	requests := &testutil.MockRequestStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Request, error) { return expected, nil },
	}
	approvals := &testutil.MockApprovalStore{
		ListByRequestIDFunc: func(ctx context.Context, id uuid.UUID) ([]model.Approval, error) { return nil, nil },
	}
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, requestType string) (*model.Policy, error) {
			return &model.Policy{
				Stages: []model.ApprovalStage{{Index: 0, Name: "review", RequiredApprovals: 1}},
			}, nil
		},
	}
	handler := newTestRequestHandler(requests, approvals, policies, &testutil.MockAuditStore{})

	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(chiContext(req.Context(), "id", reqID.String()))
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRequestHandler_Get_InvalidID(t *testing.T) {
	handler := newTestRequestHandler(&testutil.MockRequestStore{}, &testutil.MockApprovalStore{}, &testutil.MockPolicyStore{}, &testutil.MockAuditStore{})

	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(chiContext(req.Context(), "id", "not-a-uuid"))
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRequestHandler_Get_NotFound(t *testing.T) {
	requests := &testutil.MockRequestStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Request, error) { return nil, nil },
	}
	handler := newTestRequestHandler(requests, &testutil.MockApprovalStore{}, &testutil.MockPolicyStore{}, &testutil.MockAuditStore{})

	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(chiContext(req.Context(), "id", uuid.New().String()))
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestRequestHandler_List_DefaultPagination(t *testing.T) {
	var capturedFilter store.RequestFilter
	requests := &testutil.MockRequestStore{
		ListFunc: func(ctx context.Context, filter store.RequestFilter) ([]model.Request, int, error) {
			capturedFilter = filter
			return nil, 0, nil
		},
	}
	handler := newTestRequestHandler(requests, &testutil.MockApprovalStore{}, &testutil.MockPolicyStore{}, &testutil.MockAuditStore{})

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if capturedFilter.Page != 1 {
		t.Errorf("default page = %d, want 1", capturedFilter.Page)
	}
	if capturedFilter.PerPage != 20 {
		t.Errorf("default per_page = %d, want 20", capturedFilter.PerPage)
	}
}

func TestRequestHandler_List_WithFilters(t *testing.T) {
	var capturedFilter store.RequestFilter
	requests := &testutil.MockRequestStore{
		ListFunc: func(ctx context.Context, filter store.RequestFilter) ([]model.Request, int, error) {
			capturedFilter = filter
			return nil, 0, nil
		},
	}
	handler := newTestRequestHandler(requests, &testutil.MockApprovalStore{}, &testutil.MockPolicyStore{}, &testutil.MockAuditStore{})

	req := httptest.NewRequest("GET", "/?status=pending&type=transfer&maker_id=user-1&page=2&per_page=10", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if capturedFilter.Status == nil || string(*capturedFilter.Status) != "pending" {
		t.Errorf("status filter = %v, want pending", capturedFilter.Status)
	}
	if capturedFilter.Type == nil || *capturedFilter.Type != "transfer" {
		t.Errorf("type filter = %v, want transfer", capturedFilter.Type)
	}
	if capturedFilter.MakerID == nil || *capturedFilter.MakerID != "user-1" {
		t.Errorf("maker_id filter = %v, want user-1", capturedFilter.MakerID)
	}
	if capturedFilter.Page != 2 {
		t.Errorf("page = %d, want 2", capturedFilter.Page)
	}
	if capturedFilter.PerPage != 10 {
		t.Errorf("per_page = %d, want 10", capturedFilter.PerPage)
	}
}

func TestRequestHandler_Approve_Success(t *testing.T) {
	reqID := uuid.New()
	req := testutil.NewRequest(func(r *model.Request) { r.ID = reqID })
	policy := testutil.NewPolicy(func(p *model.Policy) { p.Stages[0].RequiredApprovals = 1 })

	requests := &testutil.MockRequestStore{
		GetByIDFunc:      func(ctx context.Context, id uuid.UUID) (*model.Request, error) { return req, nil },
		UpdateStatusFunc: func(ctx context.Context, id uuid.UUID, status model.RequestStatus) error { return nil },
	}
	approvalsStore := &testutil.MockApprovalStore{
		ExistsByCheckerAndStageFunc: func(ctx context.Context, reqID uuid.UUID, checkerID string, stageIndex int) (bool, error) {
			return false, nil
		},
		CreateFunc: func(ctx context.Context, approval *model.Approval) error { return nil },
		CountByDecisionAndStageFunc: func(ctx context.Context, reqID uuid.UUID, decision model.Decision, stageIndex int) (int, error) {
			if decision == model.DecisionApproved {
				return 1, nil
			}
			return 0, nil
		},
		ListByRequestIDFunc: func(ctx context.Context, id uuid.UUID) ([]model.Approval, error) { return nil, nil },
	}
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) { return policy, nil },
	}
	audits := &testutil.MockAuditStore{
		CreateFunc: func(ctx context.Context, log *model.AuditLog) error { return nil },
	}

	handler := newTestRequestHandler(requests, approvalsStore, policies, audits)

	httpReq := httptest.NewRequest("POST", "/", bytes.NewBufferString(`{"comment":"looks good"}`))
	ctx := chiContext(httpReq.Context(), "id", reqID.String())
	ctx = auth.WithIdentity(ctx, &auth.Identity{UserID: "checker-1", Roles: []string{"admin"}})
	httpReq = httpReq.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.Approve(rec, httpReq)

	if rec.Code != http.StatusOK {
		var errBody map[string]string
		json.NewDecoder(rec.Body).Decode(&errBody)
		t.Errorf("status = %d, want %d, body: %v", rec.Code, http.StatusOK, errBody)
	}
}

func TestRequestHandler_Approve_SelfApproval(t *testing.T) {
	reqID := uuid.New()
	req := testutil.NewRequest(func(r *model.Request) {
		r.ID = reqID
		r.MakerID = "user-1"
	})
	requests := &testutil.MockRequestStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Request, error) { return req, nil },
	}

	handler := newTestRequestHandler(requests, &testutil.MockApprovalStore{}, &testutil.MockPolicyStore{}, &testutil.MockAuditStore{})

	httpReq := httptest.NewRequest("POST", "/", nil)
	ctx := chiContext(httpReq.Context(), "id", reqID.String())
	ctx = auth.WithIdentity(ctx, &auth.Identity{UserID: "user-1"})
	httpReq = httpReq.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.Approve(rec, httpReq)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestRequestHandler_Approve_NotFound(t *testing.T) {
	requests := &testutil.MockRequestStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Request, error) { return nil, nil },
	}

	handler := newTestRequestHandler(requests, &testutil.MockApprovalStore{}, &testutil.MockPolicyStore{}, &testutil.MockAuditStore{})

	httpReq := httptest.NewRequest("POST", "/", nil)
	ctx := chiContext(httpReq.Context(), "id", uuid.New().String())
	ctx = auth.WithIdentity(ctx, &auth.Identity{UserID: "checker-1"})
	httpReq = httpReq.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.Approve(rec, httpReq)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestRequestHandler_Approve_NotPending(t *testing.T) {
	reqID := uuid.New()
	req := testutil.NewRequest(func(r *model.Request) {
		r.ID = reqID
		r.Status = model.StatusApproved
	})
	requests := &testutil.MockRequestStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Request, error) { return req, nil },
	}

	handler := newTestRequestHandler(requests, &testutil.MockApprovalStore{}, &testutil.MockPolicyStore{}, &testutil.MockAuditStore{})

	httpReq := httptest.NewRequest("POST", "/", nil)
	ctx := chiContext(httpReq.Context(), "id", reqID.String())
	ctx = auth.WithIdentity(ctx, &auth.Identity{UserID: "checker-1"})
	httpReq = httpReq.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.Approve(rec, httpReq)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestRequestHandler_Reject_Success(t *testing.T) {
	reqID := uuid.New()
	req := testutil.NewRequest(func(r *model.Request) { r.ID = reqID })
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.Stages[0].RejectionPolicy = model.RejectionPolicyAny
	})

	requests := &testutil.MockRequestStore{
		GetByIDFunc:      func(ctx context.Context, id uuid.UUID) (*model.Request, error) { return req, nil },
		UpdateStatusFunc: func(ctx context.Context, id uuid.UUID, status model.RequestStatus) error { return nil },
	}
	approvalsStore := &testutil.MockApprovalStore{
		ExistsByCheckerAndStageFunc: func(ctx context.Context, reqID uuid.UUID, checkerID string, stageIndex int) (bool, error) {
			return false, nil
		},
		CreateFunc: func(ctx context.Context, approval *model.Approval) error { return nil },
		CountByDecisionAndStageFunc: func(ctx context.Context, reqID uuid.UUID, decision model.Decision, stageIndex int) (int, error) {
			if decision == model.DecisionRejected {
				return 1, nil
			}
			return 0, nil
		},
		ListByRequestIDFunc: func(ctx context.Context, id uuid.UUID) ([]model.Approval, error) { return nil, nil },
	}
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) { return policy, nil },
	}
	audits := &testutil.MockAuditStore{
		CreateFunc: func(ctx context.Context, log *model.AuditLog) error { return nil },
	}

	handler := newTestRequestHandler(requests, approvalsStore, policies, audits)

	httpReq := httptest.NewRequest("POST", "/", nil)
	ctx := chiContext(httpReq.Context(), "id", reqID.String())
	ctx = auth.WithIdentity(ctx, &auth.Identity{UserID: "checker-1"})
	httpReq = httpReq.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.Reject(rec, httpReq)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRequestHandler_Cancel_Success(t *testing.T) {
	reqID := uuid.New()
	req := testutil.NewRequest(func(r *model.Request) {
		r.ID = reqID
		r.MakerID = "maker-1"
	})
	requests := &testutil.MockRequestStore{
		GetByIDFunc:      func(ctx context.Context, id uuid.UUID) (*model.Request, error) { return req, nil },
		UpdateStatusFunc: func(ctx context.Context, id uuid.UUID, status model.RequestStatus) error { return nil },
	}
	approvalsStore := &testutil.MockApprovalStore{
		ListByRequestIDFunc: func(ctx context.Context, id uuid.UUID) ([]model.Approval, error) { return nil, nil },
	}
	audits := &testutil.MockAuditStore{
		CreateFunc: func(ctx context.Context, log *model.AuditLog) error { return nil },
	}

	handler := newTestRequestHandler(requests, approvalsStore, &testutil.MockPolicyStore{}, audits)

	httpReq := httptest.NewRequest("POST", "/", nil)
	ctx := chiContext(httpReq.Context(), "id", reqID.String())
	ctx = auth.WithIdentity(ctx, &auth.Identity{UserID: "maker-1"})
	httpReq = httpReq.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.Cancel(rec, httpReq)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRequestHandler_Cancel_NotFound(t *testing.T) {
	requests := &testutil.MockRequestStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Request, error) { return nil, nil },
	}

	handler := newTestRequestHandler(requests, &testutil.MockApprovalStore{}, &testutil.MockPolicyStore{}, &testutil.MockAuditStore{})

	httpReq := httptest.NewRequest("POST", "/", nil)
	ctx := chiContext(httpReq.Context(), "id", uuid.New().String())
	ctx = auth.WithIdentity(ctx, &auth.Identity{UserID: "maker-1"})
	httpReq = httpReq.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.Cancel(rec, httpReq)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
