package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/auth"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/store"
	"github.com/lawale/quorum/internal/testutil"
)

// newTestRequestService creates a RequestService with mock stores and optional AuthorizationHook.
func newTestRequestService(
	requests *testutil.MockRequestStore,
	approvals *testutil.MockApprovalStore,
	policies *testutil.MockPolicyStore,
	audits *testutil.MockAuditStore,
	hook *auth.AuthorizationHook,
) *RequestService {
	return NewRequestService(requests, approvals, policies, audits, hook)
}

// --- Create Tests ---

func TestCreate_Success_NoIdentityFields(t *testing.T) {
	policy := testutil.NewPolicy()
	requests := &testutil.MockRequestStore{
		CreateFunc: func(ctx context.Context, req *model.Request) error { return nil },
	}
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) {
			return policy, nil
		},
	}
	audits := &testutil.MockAuditStore{
		CreateFunc: func(ctx context.Context, log *model.AuditLog) error { return nil },
	}
	svc := newTestRequestService(requests, &testutil.MockApprovalStore{}, policies, audits, nil)

	req := testutil.NewRequest()
	result, err := svc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Fingerprint != nil {
		t.Error("expected nil fingerprint when no identity fields")
	}
}

func TestCreate_WithIdempotencyKey_New(t *testing.T) {
	policy := testutil.NewPolicy()
	requests := &testutil.MockRequestStore{
		GetByIdempotencyKeyFunc: func(ctx context.Context, key string) (*model.Request, error) {
			return nil, nil // Not found
		},
		CreateFunc: func(ctx context.Context, req *model.Request) error { return nil },
	}
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) {
			return policy, nil
		},
	}
	audits := &testutil.MockAuditStore{
		CreateFunc: func(ctx context.Context, log *model.AuditLog) error { return nil },
	}
	svc := newTestRequestService(requests, &testutil.MockApprovalStore{}, policies, audits, nil)

	req := testutil.NewRequest(func(r *model.Request) {
		r.IdempotencyKey = testutil.StringPtr("idem-key-1")
	})
	result, err := svc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
}

func TestCreate_IdempotencyKey_ReturnsExisting(t *testing.T) {
	existing := testutil.NewRequest(func(r *model.Request) {
		r.IdempotencyKey = testutil.StringPtr("idem-key-1")
		r.Status = model.StatusApproved
	})
	requests := &testutil.MockRequestStore{
		GetByIdempotencyKeyFunc: func(ctx context.Context, key string) (*model.Request, error) {
			return existing, nil
		},
	}
	svc := newTestRequestService(requests, &testutil.MockApprovalStore{}, &testutil.MockPolicyStore{}, &testutil.MockAuditStore{}, nil)

	req := testutil.NewRequest(func(r *model.Request) {
		r.IdempotencyKey = testutil.StringPtr("idem-key-1")
	})
	result, err := svc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != existing.ID {
		t.Errorf("expected existing request ID %v, got %v", existing.ID, result.ID)
	}
	if result.Status != model.StatusApproved {
		t.Errorf("expected existing status, got %v", result.Status)
	}
}

func TestCreate_PolicyNotFound(t *testing.T) {
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) {
			return nil, nil
		},
	}
	svc := newTestRequestService(&testutil.MockRequestStore{}, &testutil.MockApprovalStore{}, policies, &testutil.MockAuditStore{}, nil)

	req := testutil.NewRequest()
	_, err := svc.Create(context.Background(), req)
	if !errors.Is(err, ErrPolicyNotFound) {
		t.Fatalf("expected ErrPolicyNotFound, got: %v", err)
	}
}

func TestCreate_WithIdentityFields_ComputesFingerprint(t *testing.T) {
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.IdentityFields = []string{"account_id"}
	})
	var createdReq *model.Request
	requests := &testutil.MockRequestStore{
		FindPendingByFingerprintFunc: func(ctx context.Context, reqType, fp string) (*model.Request, error) {
			return nil, nil // No duplicate
		},
		CreateFunc: func(ctx context.Context, req *model.Request) error {
			createdReq = req
			return nil
		},
	}
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) {
			return policy, nil
		},
	}
	audits := &testutil.MockAuditStore{
		CreateFunc: func(ctx context.Context, log *model.AuditLog) error { return nil },
	}
	svc := newTestRequestService(requests, &testutil.MockApprovalStore{}, policies, audits, nil)

	req := testutil.NewRequest(func(r *model.Request) {
		r.Payload = json.RawMessage(`{"account_id":"acc-123","amount":100}`)
	})
	_, err := svc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if createdReq.Fingerprint == nil {
		t.Fatal("expected fingerprint to be set")
	}
	if *createdReq.Fingerprint == "" {
		t.Error("expected non-empty fingerprint")
	}
}

func TestCreate_WithIdentityFields_DuplicateDetected(t *testing.T) {
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.IdentityFields = []string{"account_id"}
	})
	existingReq := testutil.NewRequest()
	requests := &testutil.MockRequestStore{
		FindPendingByFingerprintFunc: func(ctx context.Context, reqType, fp string) (*model.Request, error) {
			return existingReq, nil // Duplicate found
		},
	}
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) {
			return policy, nil
		},
	}
	svc := newTestRequestService(requests, &testutil.MockApprovalStore{}, policies, &testutil.MockAuditStore{}, nil)

	req := testutil.NewRequest(func(r *model.Request) {
		r.Payload = json.RawMessage(`{"account_id":"acc-123"}`)
	})
	_, err := svc.Create(context.Background(), req)
	if !errors.Is(err, ErrDuplicateRequest) {
		t.Fatalf("expected ErrDuplicateRequest, got: %v", err)
	}
}

func TestCreate_WithIdentityFields_MissingField(t *testing.T) {
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.IdentityFields = []string{"account_id", "missing_field"}
	})
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) {
			return policy, nil
		},
	}
	svc := newTestRequestService(&testutil.MockRequestStore{}, &testutil.MockApprovalStore{}, policies, &testutil.MockAuditStore{}, nil)

	req := testutil.NewRequest(func(r *model.Request) {
		r.Payload = json.RawMessage(`{"account_id":"acc-123"}`)
	})
	_, err := svc.Create(context.Background(), req)
	if !errors.Is(err, ErrMissingIdentityFields) {
		t.Fatalf("expected ErrMissingIdentityFields, got: %v", err)
	}
}

func TestCreate_SetsExpiryFromPolicy(t *testing.T) {
	dur := 2 * time.Hour
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.AutoExpireDuration = &dur
	})
	var createdReq *model.Request
	requests := &testutil.MockRequestStore{
		CreateFunc: func(ctx context.Context, req *model.Request) error {
			createdReq = req
			return nil
		},
	}
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) {
			return policy, nil
		},
	}
	audits := &testutil.MockAuditStore{
		CreateFunc: func(ctx context.Context, log *model.AuditLog) error { return nil },
	}
	svc := newTestRequestService(requests, &testutil.MockApprovalStore{}, policies, audits, nil)

	req := testutil.NewRequest()
	_, err := svc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if createdReq.ExpiresAt == nil {
		t.Fatal("expected ExpiresAt to be set")
	}
	// Should be roughly 2 hours from now
	diff := time.Until(*createdReq.ExpiresAt)
	if diff < 1*time.Hour || diff > 3*time.Hour {
		t.Errorf("ExpiresAt is not ~2h from now: %v", diff)
	}
}

func TestCreate_NoExpiry_WhenPolicyHasNone(t *testing.T) {
	policy := testutil.NewPolicy() // No AutoExpireDuration
	var createdReq *model.Request
	requests := &testutil.MockRequestStore{
		CreateFunc: func(ctx context.Context, req *model.Request) error {
			createdReq = req
			return nil
		},
	}
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) {
			return policy, nil
		},
	}
	audits := &testutil.MockAuditStore{
		CreateFunc: func(ctx context.Context, log *model.AuditLog) error { return nil },
	}
	svc := newTestRequestService(requests, &testutil.MockApprovalStore{}, policies, audits, nil)

	req := testutil.NewRequest()
	_, err := svc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if createdReq.ExpiresAt != nil {
		t.Error("expected nil ExpiresAt when policy has no auto_expire_duration")
	}
}

func TestCreate_Fingerprint_Deterministic(t *testing.T) {
	payload := json.RawMessage(`{"account_id":"acc-123","amount":500}`)
	fields := []string{"account_id", "amount"}

	fp1, err := computeFingerprint(payload, fields)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fp2, err := computeFingerprint(payload, fields)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fp1 != fp2 {
		t.Errorf("fingerprints differ for same input: %s vs %s", fp1, fp2)
	}
}

func TestCreate_Fingerprint_DifferentValues(t *testing.T) {
	fields := []string{"account_id"}

	fp1, _ := computeFingerprint(json.RawMessage(`{"account_id":"acc-123"}`), fields)
	fp2, _ := computeFingerprint(json.RawMessage(`{"account_id":"acc-456"}`), fields)

	if fp1 == fp2 {
		t.Error("fingerprints should differ for different values")
	}
}

func TestCreate_Fingerprint_FieldOrderDoesNotMatter(t *testing.T) {
	fields1 := []string{"amount", "account_id"}
	fields2 := []string{"account_id", "amount"}

	payload := json.RawMessage(`{"account_id":"acc-123","amount":100}`)

	fp1, _ := computeFingerprint(payload, fields1)
	fp2, _ := computeFingerprint(payload, fields2)

	if fp1 != fp2 {
		t.Error("fingerprints should be the same regardless of field order")
	}
}

// --- GetByID Tests ---

func TestGetByID_Success(t *testing.T) {
	reqID := uuid.New()
	expected := testutil.NewRequest(func(r *model.Request) { r.ID = reqID })
	approvals := []model.Approval{*testutil.NewApproval(func(a *model.Approval) { a.RequestID = reqID })}

	requests := &testutil.MockRequestStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Request, error) {
			return expected, nil
		},
	}
	approvalStore := &testutil.MockApprovalStore{
		ListByRequestIDFunc: func(ctx context.Context, id uuid.UUID) ([]model.Approval, error) {
			return approvals, nil
		},
	}
	svc := newTestRequestService(requests, approvalStore, &testutil.MockPolicyStore{}, &testutil.MockAuditStore{}, nil)

	result, err := svc.GetByID(context.Background(), reqID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != reqID {
		t.Errorf("ID = %v, want %v", result.ID, reqID)
	}
	if len(result.Approvals) != 1 {
		t.Errorf("expected 1 approval, got %d", len(result.Approvals))
	}
}

func TestGetByID_NotFound(t *testing.T) {
	requests := &testutil.MockRequestStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Request, error) {
			return nil, nil
		},
	}
	svc := newTestRequestService(requests, &testutil.MockApprovalStore{}, &testutil.MockPolicyStore{}, &testutil.MockAuditStore{}, nil)

	_, err := svc.GetByID(context.Background(), uuid.New())
	if !errors.Is(err, ErrRequestNotFound) {
		t.Fatalf("expected ErrRequestNotFound, got: %v", err)
	}
}

// --- List Tests ---

func TestList_DelegatesToStore(t *testing.T) {
	expected := []model.Request{*testutil.NewRequest()}
	requests := &testutil.MockRequestStore{
		ListFunc: func(ctx context.Context, filter store.RequestFilter) ([]model.Request, int, error) {
			return expected, 1, nil
		},
	}
	svc := newTestRequestService(requests, &testutil.MockApprovalStore{}, &testutil.MockPolicyStore{}, &testutil.MockAuditStore{}, nil)

	result, total, err := svc.List(context.Background(), store.RequestFilter{Page: 1, PerPage: 20})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || total != 1 {
		t.Errorf("expected 1 result with total 1, got %d results with total %d", len(result), total)
	}
}

// --- Approve Tests ---

func setupApproveTest(req *model.Request, policy *model.Policy) (*testutil.MockRequestStore, *testutil.MockApprovalStore, *testutil.MockPolicyStore, *testutil.MockAuditStore) {
	requests := &testutil.MockRequestStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Request, error) {
			return req, nil
		},
		UpdateStatusFunc: func(ctx context.Context, id uuid.UUID, status model.RequestStatus) error {
			return nil
		},
		UpdateStageAndStatusFunc: func(ctx context.Context, id uuid.UUID, stage int, status model.RequestStatus) error {
			return nil
		},
	}
	approvals := &testutil.MockApprovalStore{
		ExistsByCheckerAndStageFunc: func(ctx context.Context, reqID uuid.UUID, checkerID string, stageIndex int) (bool, error) {
			return false, nil
		},
		CreateFunc: func(ctx context.Context, approval *model.Approval) error {
			return nil
		},
		CountByDecisionAndStageFunc: func(ctx context.Context, reqID uuid.UUID, decision model.Decision, stageIndex int) (int, error) {
			return 0, nil // Counts are read before the vote; 0 existing votes
		},
		ListByRequestIDFunc: func(ctx context.Context, id uuid.UUID) ([]model.Approval, error) {
			return []model.Approval{}, nil
		},
	}
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) {
			return policy, nil
		},
	}
	audits := &testutil.MockAuditStore{
		CreateFunc: func(ctx context.Context, log *model.AuditLog) error { return nil },
	}
	return requests, approvals, policies, audits
}

func TestApprove_Success_MeetsThreshold(t *testing.T) {
	req := testutil.NewRequest()
	policy := testutil.NewPolicy() // Default: 1 stage, 1 required approval
	requestStore, approvalStore, policyStore, auditStore := setupApproveTest(req, policy)

	svc := newTestRequestService(requestStore, approvalStore, policyStore, auditStore, nil)

	result, err := svc.Approve(context.Background(), req.ID, "checker-1", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != model.StatusApproved {
		t.Errorf("status = %v, want approved", result.Status)
	}
}

func TestApprove_Success_BelowThreshold(t *testing.T) {
	req := testutil.NewRequest()
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.Stages[0].RequiredApprovals = 3
	})
	requestStore, approvalStore, policyStore, auditStore := setupApproveTest(req, policy)

	// Override: 1 existing approval, this vote adds another → 2 total, need 3
	approvalStore.CountByDecisionAndStageFunc = func(ctx context.Context, reqID uuid.UUID, decision model.Decision, stageIndex int) (int, error) {
		if decision == model.DecisionApproved {
			return 1, nil
		}
		return 0, nil
	}

	svc := newTestRequestService(requestStore, approvalStore, policyStore, auditStore, nil)

	result, err := svc.Approve(context.Background(), req.ID, "checker-1", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != model.StatusPending {
		t.Errorf("status = %v, want pending (below threshold)", result.Status)
	}
}

func TestApprove_RequestNotFound(t *testing.T) {
	requests := &testutil.MockRequestStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Request, error) {
			return nil, nil
		},
	}
	svc := newTestRequestService(requests, &testutil.MockApprovalStore{}, &testutil.MockPolicyStore{}, &testutil.MockAuditStore{}, nil)

	_, err := svc.Approve(context.Background(), uuid.New(), "checker-1", nil, nil)
	if !errors.Is(err, ErrRequestNotFound) {
		t.Fatalf("expected ErrRequestNotFound, got: %v", err)
	}
}

func TestApprove_RequestNotPending(t *testing.T) {
	req := testutil.NewRequest(func(r *model.Request) { r.Status = model.StatusApproved })
	requests := &testutil.MockRequestStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Request, error) {
			return req, nil
		},
	}
	svc := newTestRequestService(requests, &testutil.MockApprovalStore{}, &testutil.MockPolicyStore{}, &testutil.MockAuditStore{}, nil)

	_, err := svc.Approve(context.Background(), req.ID, "checker-1", nil, nil)
	if !errors.Is(err, ErrRequestNotPending) {
		t.Fatalf("expected ErrRequestNotPending, got: %v", err)
	}
}

func TestApprove_SelfApproval(t *testing.T) {
	req := testutil.NewRequest(func(r *model.Request) { r.MakerID = "user-1" })
	requests := &testutil.MockRequestStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Request, error) {
			return req, nil
		},
	}
	svc := newTestRequestService(requests, &testutil.MockApprovalStore{}, &testutil.MockPolicyStore{}, &testutil.MockAuditStore{}, nil)

	_, err := svc.Approve(context.Background(), req.ID, "user-1", nil, nil)
	if !errors.Is(err, ErrSelfApproval) {
		t.Fatalf("expected ErrSelfApproval, got: %v", err)
	}
}

func TestApprove_AlreadyActioned(t *testing.T) {
	req := testutil.NewRequest()
	policy := testutil.NewPolicy()
	requests := &testutil.MockRequestStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Request, error) {
			return req, nil
		},
	}
	approvals := &testutil.MockApprovalStore{
		ExistsByCheckerAndStageFunc: func(ctx context.Context, reqID uuid.UUID, checkerID string, stageIndex int) (bool, error) {
			return true, nil // Already acted
		},
	}
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) {
			return policy, nil
		},
	}
	svc := newTestRequestService(requests, approvals, policies, &testutil.MockAuditStore{}, nil)

	_, err := svc.Approve(context.Background(), req.ID, "checker-1", nil, nil)
	if !errors.Is(err, ErrAlreadyActioned) {
		t.Fatalf("expected ErrAlreadyActioned, got: %v", err)
	}
}

func TestApprove_InvalidCheckerRole(t *testing.T) {
	req := testutil.NewRequest()
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.Stages[0].AllowedCheckerRoles = json.RawMessage(`["admin","manager"]`)
	})
	requestStore, approvalStore, policyStore, auditStore := setupApproveTest(req, policy)

	svc := newTestRequestService(requestStore, approvalStore, policyStore, auditStore, nil)

	_, err := svc.Approve(context.Background(), req.ID, "checker-1", []string{"viewer"}, nil)
	if !errors.Is(err, ErrInvalidCheckerRole) {
		t.Fatalf("expected ErrInvalidCheckerRole, got: %v", err)
	}
}

func TestApprove_ValidCheckerRole(t *testing.T) {
	req := testutil.NewRequest()
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.Stages[0].AllowedCheckerRoles = json.RawMessage(`["admin","manager"]`)
	})
	requestStore, approvalStore, policyStore, auditStore := setupApproveTest(req, policy)

	svc := newTestRequestService(requestStore, approvalStore, policyStore, auditStore, nil)

	_, err := svc.Approve(context.Background(), req.ID, "checker-1", []string{"manager"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApprove_NoAllowedRoles_SkipsRoleCheck(t *testing.T) {
	req := testutil.NewRequest()
	policy := testutil.NewPolicy() // No AllowedCheckerRoles on stage
	requestStore, approvalStore, policyStore, auditStore := setupApproveTest(req, policy)

	svc := newTestRequestService(requestStore, approvalStore, policyStore, auditStore, nil)

	_, err := svc.Approve(context.Background(), req.ID, "checker-1", []string{"anything"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApprove_NotEligibleReviewer(t *testing.T) {
	req := testutil.NewRequest(func(r *model.Request) {
		r.EligibleReviewers = []string{"reviewer-1", "reviewer-2"}
	})
	policy := testutil.NewPolicy()
	requestStore, approvalStore, policyStore, auditStore := setupApproveTest(req, policy)

	svc := newTestRequestService(requestStore, approvalStore, policyStore, auditStore, nil)

	_, err := svc.Approve(context.Background(), req.ID, "checker-1", nil, nil)
	if !errors.Is(err, ErrNotEligibleReviewer) {
		t.Fatalf("expected ErrNotEligibleReviewer, got: %v", err)
	}
}

func TestApprove_EligibleReviewer_Accepted(t *testing.T) {
	req := testutil.NewRequest(func(r *model.Request) {
		r.EligibleReviewers = []string{"checker-1", "checker-2"}
	})
	policy := testutil.NewPolicy()
	requestStore, approvalStore, policyStore, auditStore := setupApproveTest(req, policy)

	svc := newTestRequestService(requestStore, approvalStore, policyStore, auditStore, nil)

	_, err := svc.Approve(context.Background(), req.ID, "checker-1", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApprove_NoEligibleReviewers_Skips(t *testing.T) {
	req := testutil.NewRequest() // No eligible reviewers
	policy := testutil.NewPolicy()
	requestStore, approvalStore, policyStore, auditStore := setupApproveTest(req, policy)

	svc := newTestRequestService(requestStore, approvalStore, policyStore, auditStore, nil)

	_, err := svc.Approve(context.Background(), req.ID, "checker-1", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApprove_AuthorizationHook_Denied(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(model.AuthorizationHookResponse{Allowed: false, Reason: "not authorized"})
	}))
	defer server.Close()

	req := testutil.NewRequest()
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.DynamicAuthorizationURL = testutil.StringPtr(server.URL)
	})
	requestStore, approvalStore, policyStore, auditStore := setupApproveTest(req, policy)
	hook := auth.NewAuthorizationHook(5 * time.Second)

	svc := newTestRequestService(requestStore, approvalStore, policyStore, auditStore, hook)

	_, err := svc.Approve(context.Background(), req.ID, "checker-1", nil, nil)
	if !errors.Is(err, auth.ErrAuthorizationDenied) {
		t.Fatalf("expected ErrAuthorizationDenied, got: %v", err)
	}
}

func TestApprove_AuthorizationHook_Allowed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(model.AuthorizationHookResponse{Allowed: true})
	}))
	defer server.Close()

	req := testutil.NewRequest()
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.DynamicAuthorizationURL = testutil.StringPtr(server.URL)
	})
	requestStore, approvalStore, policyStore, auditStore := setupApproveTest(req, policy)
	hook := auth.NewAuthorizationHook(5 * time.Second)

	svc := newTestRequestService(requestStore, approvalStore, policyStore, auditStore, hook)

	_, err := svc.Approve(context.Background(), req.ID, "checker-1", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApprove_AuthorizationHook_NoURL_Skips(t *testing.T) {
	req := testutil.NewRequest()
	policy := testutil.NewPolicy() // No DynamicAuthorizationURL
	requestStore, approvalStore, policyStore, auditStore := setupApproveTest(req, policy)
	hook := auth.NewAuthorizationHook(5 * time.Second)

	svc := newTestRequestService(requestStore, approvalStore, policyStore, auditStore, hook)

	_, err := svc.Approve(context.Background(), req.ID, "checker-1", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApprove_WebhookDispatch_CalledOnTerminal(t *testing.T) {
	req := testutil.NewRequest()
	policy := testutil.NewPolicy() // 1 stage, 1 required
	requestStore, approvalStore, policyStore, auditStore := setupApproveTest(req, policy)

	svc := newTestRequestService(requestStore, approvalStore, policyStore, auditStore, nil)

	enqueueCalled := false
	signalCalled := false
	svc.SetWebhookDispatch(
		func(ctx context.Context, fn func(tx *store.Stores) error) error {
			// Inside the tx, counts reflect the just-inserted vote.
			txApprovals := &testutil.MockApprovalStore{
				CreateFunc:                  approvalStore.CreateFunc,
				ListByRequestIDFunc:         approvalStore.ListByRequestIDFunc,
				ExistsByCheckerAndStageFunc: approvalStore.ExistsByCheckerAndStageFunc,
				CountByDecisionAndStageFunc: func(ctx context.Context, reqID uuid.UUID, decision model.Decision, stageIndex int) (int, error) {
					if decision == model.DecisionApproved {
						return 1, nil // 1 approved vote (just inserted)
					}
					return 0, nil
				},
			}
			txStores := &store.Stores{
				Requests:  requestStore,
				Approvals: txApprovals,
				Outbox:    &testutil.MockOutboxStore{},
				Webhooks:  &testutil.MockWebhookStore{},
			}
			return fn(txStores)
		},
		func(ctx context.Context, outbox store.OutboxStore, webhooks store.WebhookStore, r *model.Request, approvals []model.Approval) error {
			enqueueCalled = true
			return nil
		},
		func() { signalCalled = true },
	)

	_, err := svc.Approve(context.Background(), req.ID, "checker-1", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !enqueueCalled {
		t.Error("expected enqueueWebhooks to be called on terminal status")
	}
	if !signalCalled {
		t.Error("expected signalWebhooks to be called after commit")
	}
}

func TestApprove_WebhookDispatch_NotCalledWhenPending(t *testing.T) {
	req := testutil.NewRequest()
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.Stages[0].RequiredApprovals = 3
	})
	requestStore, approvalStore, policyStore, auditStore := setupApproveTest(req, policy)

	svc := newTestRequestService(requestStore, approvalStore, policyStore, auditStore, nil)

	enqueueCalled := false
	svc.SetWebhookDispatch(
		func(ctx context.Context, fn func(tx *store.Stores) error) error {
			// Inside the tx, counts reflect: 1 existing + 1 just inserted = 2, need 3 → still pending
			txApprovals := &testutil.MockApprovalStore{
				CreateFunc:                  approvalStore.CreateFunc,
				ListByRequestIDFunc:         approvalStore.ListByRequestIDFunc,
				ExistsByCheckerAndStageFunc: approvalStore.ExistsByCheckerAndStageFunc,
				CountByDecisionAndStageFunc: func(ctx context.Context, reqID uuid.UUID, decision model.Decision, stageIndex int) (int, error) {
					if decision == model.DecisionApproved {
						return 2, nil // 2 approved (1 existing + 1 just inserted), need 3
					}
					return 0, nil
				},
			}
			return fn(&store.Stores{
				Requests:  requestStore,
				Approvals: txApprovals,
			})
		},
		func(ctx context.Context, outbox store.OutboxStore, webhooks store.WebhookStore, r *model.Request, approvals []model.Approval) error {
			enqueueCalled = true
			return nil
		},
		func() {},
	)

	_, err := svc.Approve(context.Background(), req.ID, "checker-1", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if enqueueCalled {
		t.Error("enqueueWebhooks should not be called when status is still pending")
	}
}

// --- Reject Tests ---

func TestReject_RejectionPolicyAny_ImmediateReject(t *testing.T) {
	req := testutil.NewRequest()
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.Stages[0].RequiredApprovals = 2
		p.Stages[0].RejectionPolicy = model.RejectionPolicyAny
	})
	requestStore, approvalStore, policyStore, auditStore := setupApproveTest(req, policy)

	// Override counts for rejection: 0 approvals, 1 rejection
	approvalStore.CountByDecisionAndStageFunc = func(ctx context.Context, reqID uuid.UUID, decision model.Decision, stageIndex int) (int, error) {
		if decision == model.DecisionRejected {
			return 1, nil
		}
		return 0, nil
	}

	svc := newTestRequestService(requestStore, approvalStore, policyStore, auditStore, nil)

	result, err := svc.Reject(context.Background(), req.ID, "checker-1", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != model.StatusRejected {
		t.Errorf("status = %v, want rejected", result.Status)
	}
}

func TestReject_RejectionPolicyThreshold_StillPossible(t *testing.T) {
	req := testutil.NewRequest()
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.Stages[0].RequiredApprovals = 2
		p.Stages[0].RejectionPolicy = model.RejectionPolicyThreshold
		p.Stages[0].MaxCheckers = testutil.IntPtr(3)
	})
	requestStore, approvalStore, policyStore, auditStore := setupApproveTest(req, policy)

	// 0 existing rejections; this vote adds 1 → predicted: 0 approvals, 1 rejection
	// Remaining = 3 - 0 - 1 = 2. 0 + 2 = 2 >= 2 → still possible → pending
	// (default mock returns 0 for all counts, which is correct here)

	svc := newTestRequestService(requestStore, approvalStore, policyStore, auditStore, nil)

	result, err := svc.Reject(context.Background(), req.ID, "checker-1", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != model.StatusPending {
		t.Errorf("status = %v, want pending (still achievable)", result.Status)
	}
}

func TestReject_RejectionPolicyThreshold_Impossible(t *testing.T) {
	req := testutil.NewRequest()
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.Stages[0].RequiredApprovals = 3
		p.Stages[0].RejectionPolicy = model.RejectionPolicyThreshold
		p.Stages[0].MaxCheckers = testutil.IntPtr(3)
	})
	requestStore, approvalStore, policyStore, auditStore := setupApproveTest(req, policy)

	// 1 existing rejection; this vote adds another → predicted: 0 approvals, 2 rejections
	// Remaining = 3 - 0 - 2 = 1. 0 + 1 < 3 → impossible → rejected
	approvalStore.CountByDecisionAndStageFunc = func(ctx context.Context, reqID uuid.UUID, decision model.Decision, stageIndex int) (int, error) {
		if decision == model.DecisionRejected {
			return 1, nil
		}
		return 0, nil
	}

	svc := newTestRequestService(requestStore, approvalStore, policyStore, auditStore, nil)

	result, err := svc.Reject(context.Background(), req.ID, "checker-1", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != model.StatusRejected {
		t.Errorf("status = %v, want rejected (mathematically impossible)", result.Status)
	}
}

func TestReject_SelfApproval(t *testing.T) {
	req := testutil.NewRequest(func(r *model.Request) { r.MakerID = "user-1" })
	requests := &testutil.MockRequestStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Request, error) {
			return req, nil
		},
	}
	svc := newTestRequestService(requests, &testutil.MockApprovalStore{}, &testutil.MockPolicyStore{}, &testutil.MockAuditStore{}, nil)

	_, err := svc.Reject(context.Background(), req.ID, "user-1", nil, nil)
	if !errors.Is(err, ErrSelfApproval) {
		t.Fatalf("expected ErrSelfApproval, got: %v", err)
	}
}

// --- Cancel Tests ---

func TestCancel_Success(t *testing.T) {
	req := testutil.NewRequest(func(r *model.Request) { r.MakerID = "maker-1" })
	requests := &testutil.MockRequestStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Request, error) {
			return req, nil
		},
		UpdateStatusFunc: func(ctx context.Context, id uuid.UUID, status model.RequestStatus) error {
			return nil
		},
	}
	approvals := &testutil.MockApprovalStore{
		ListByRequestIDFunc: func(ctx context.Context, id uuid.UUID) ([]model.Approval, error) {
			return nil, nil
		},
	}
	audits := &testutil.MockAuditStore{
		CreateFunc: func(ctx context.Context, log *model.AuditLog) error { return nil },
	}
	svc := newTestRequestService(requests, approvals, &testutil.MockPolicyStore{}, audits, nil)

	result, err := svc.Cancel(context.Background(), req.ID, "maker-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != model.StatusCancelled {
		t.Errorf("status = %v, want cancelled", result.Status)
	}
}

func TestCancel_NotFound(t *testing.T) {
	requests := &testutil.MockRequestStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Request, error) {
			return nil, nil
		},
	}
	svc := newTestRequestService(requests, &testutil.MockApprovalStore{}, &testutil.MockPolicyStore{}, &testutil.MockAuditStore{}, nil)

	_, err := svc.Cancel(context.Background(), uuid.New(), "maker-1")
	if !errors.Is(err, ErrRequestNotFound) {
		t.Fatalf("expected ErrRequestNotFound, got: %v", err)
	}
}

func TestCancel_NotPending(t *testing.T) {
	req := testutil.NewRequest(func(r *model.Request) {
		r.MakerID = "maker-1"
		r.Status = model.StatusApproved
	})
	requests := &testutil.MockRequestStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Request, error) {
			return req, nil
		},
	}
	svc := newTestRequestService(requests, &testutil.MockApprovalStore{}, &testutil.MockPolicyStore{}, &testutil.MockAuditStore{}, nil)

	_, err := svc.Cancel(context.Background(), req.ID, "maker-1")
	if !errors.Is(err, ErrRequestNotPending) {
		t.Fatalf("expected ErrRequestNotPending, got: %v", err)
	}
}

func TestCancel_WrongMaker(t *testing.T) {
	req := testutil.NewRequest(func(r *model.Request) { r.MakerID = "maker-1" })
	requests := &testutil.MockRequestStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Request, error) {
			return req, nil
		},
	}
	svc := newTestRequestService(requests, &testutil.MockApprovalStore{}, &testutil.MockPolicyStore{}, &testutil.MockAuditStore{}, nil)

	_, err := svc.Cancel(context.Background(), req.ID, "other-user")
	if err == nil {
		t.Fatal("expected error for wrong maker")
	}
}

func TestCancel_WebhookDispatch_Called(t *testing.T) {
	req := testutil.NewRequest(func(r *model.Request) { r.MakerID = "maker-1" })
	requests := &testutil.MockRequestStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Request, error) {
			return req, nil
		},
		UpdateStatusFunc: func(ctx context.Context, id uuid.UUID, status model.RequestStatus) error {
			return nil
		},
	}
	approvalStore := &testutil.MockApprovalStore{
		ListByRequestIDFunc: func(ctx context.Context, id uuid.UUID) ([]model.Approval, error) {
			return nil, nil
		},
	}
	audits := &testutil.MockAuditStore{
		CreateFunc: func(ctx context.Context, log *model.AuditLog) error { return nil },
	}
	svc := newTestRequestService(requests, approvalStore, &testutil.MockPolicyStore{}, audits, nil)

	enqueueCalled := false
	signalCalled := false
	svc.SetWebhookDispatch(
		func(ctx context.Context, fn func(tx *store.Stores) error) error {
			txStores := &store.Stores{
				Requests: requests,
				Outbox:   &testutil.MockOutboxStore{},
				Webhooks: &testutil.MockWebhookStore{},
			}
			return fn(txStores)
		},
		func(ctx context.Context, outbox store.OutboxStore, webhooks store.WebhookStore, r *model.Request, approvals []model.Approval) error {
			enqueueCalled = true
			return nil
		},
		func() { signalCalled = true },
	)

	_, err := svc.Cancel(context.Background(), req.ID, "maker-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !enqueueCalled {
		t.Error("expected enqueueWebhooks to be called on cancel")
	}
	if !signalCalled {
		t.Error("expected signalWebhooks to be called on cancel")
	}
}

// --- validateCheckerRoles Tests ---

func TestValidateCheckerRoles_NilRoles(t *testing.T) {
	stage := &model.ApprovalStage{Index: 0, RequiredApprovals: 1, RejectionPolicy: model.RejectionPolicyAny}
	err := validateCheckerRoles(stage, []string{"anything"})
	if err != nil {
		t.Fatalf("expected nil error for nil AllowedCheckerRoles, got: %v", err)
	}
}

func TestValidateCheckerRoles_EmptyArray(t *testing.T) {
	stage := &model.ApprovalStage{
		Index: 0, RequiredApprovals: 1, RejectionPolicy: model.RejectionPolicyAny,
		AllowedCheckerRoles: json.RawMessage(`[]`),
	}
	err := validateCheckerRoles(stage, []string{"anything"})
	if err != nil {
		t.Fatalf("expected nil error for empty allowed roles, got: %v", err)
	}
}

func TestValidateCheckerRoles_Match(t *testing.T) {
	stage := &model.ApprovalStage{
		Index: 0, RequiredApprovals: 1, RejectionPolicy: model.RejectionPolicyAny,
		AllowedCheckerRoles: json.RawMessage(`["admin","manager"]`),
	}
	err := validateCheckerRoles(stage, []string{"manager"})
	if err != nil {
		t.Fatalf("expected nil error for matching role, got: %v", err)
	}
}

func TestValidateCheckerRoles_NoMatch(t *testing.T) {
	stage := &model.ApprovalStage{
		Index: 0, RequiredApprovals: 1, RejectionPolicy: model.RejectionPolicyAny,
		AllowedCheckerRoles: json.RawMessage(`["admin","manager"]`),
	}
	err := validateCheckerRoles(stage, []string{"viewer"})
	if !errors.Is(err, ErrInvalidCheckerRole) {
		t.Fatalf("expected ErrInvalidCheckerRole, got: %v", err)
	}
}

// --- Multi-Stage Tests ---

func TestApprove_MultiStage_AdvancesToNextStage(t *testing.T) {
	req := testutil.NewRequest(func(r *model.Request) {
		r.CurrentStage = 0
	})
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.Stages = []model.ApprovalStage{
			{Index: 0, RequiredApprovals: 1, RejectionPolicy: model.RejectionPolicyAny},
			{Index: 1, RequiredApprovals: 1, RejectionPolicy: model.RejectionPolicyAny},
		}
	})
	requestStore, approvalStore, policyStore, auditStore := setupApproveTest(req, policy)

	var advancedStage int
	requestStore.UpdateStageAndStatusFunc = func(ctx context.Context, id uuid.UUID, stage int, status model.RequestStatus) error {
		advancedStage = stage
		return nil
	}

	svc := newTestRequestService(requestStore, approvalStore, policyStore, auditStore, nil)

	result, err := svc.Approve(context.Background(), req.ID, "checker-1", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should still be pending but at stage 1
	if result.Status != model.StatusPending {
		t.Errorf("status = %v, want pending (advanced to stage 1)", result.Status)
	}
	if advancedStage != 1 {
		t.Errorf("advanced stage = %d, want 1", advancedStage)
	}
}

func TestApprove_MultiStage_FinalStageApproved(t *testing.T) {
	req := testutil.NewRequest(func(r *model.Request) {
		r.CurrentStage = 1 // Already at last stage
	})
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.Stages = []model.ApprovalStage{
			{Index: 0, RequiredApprovals: 1, RejectionPolicy: model.RejectionPolicyAny},
			{Index: 1, RequiredApprovals: 1, RejectionPolicy: model.RejectionPolicyAny},
		}
	})
	requestStore, approvalStore, policyStore, auditStore := setupApproveTest(req, policy)

	svc := newTestRequestService(requestStore, approvalStore, policyStore, auditStore, nil)

	result, err := svc.Approve(context.Background(), req.ID, "checker-1", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != model.StatusApproved {
		t.Errorf("status = %v, want approved (final stage completed)", result.Status)
	}
}

func TestReject_MultiStage_StageRejection_RejectsEntireRequest(t *testing.T) {
	req := testutil.NewRequest(func(r *model.Request) {
		r.CurrentStage = 1
	})
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.Stages = []model.ApprovalStage{
			{Index: 0, RequiredApprovals: 1, RejectionPolicy: model.RejectionPolicyAny},
			{Index: 1, RequiredApprovals: 2, RejectionPolicy: model.RejectionPolicyAny},
		}
	})
	requestStore, approvalStore, policyStore, auditStore := setupApproveTest(req, policy)

	// 0 approvals, 1 rejection at stage 1
	approvalStore.CountByDecisionAndStageFunc = func(ctx context.Context, reqID uuid.UUID, decision model.Decision, stageIndex int) (int, error) {
		if decision == model.DecisionRejected {
			return 1, nil
		}
		return 0, nil
	}

	svc := newTestRequestService(requestStore, approvalStore, policyStore, auditStore, nil)

	result, err := svc.Reject(context.Background(), req.ID, "checker-1", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != model.StatusRejected {
		t.Errorf("status = %v, want rejected", result.Status)
	}
}

func TestApprove_ConcurrentTerminal_ReturnsNotPending(t *testing.T) {
	req := testutil.NewRequest()
	policy := testutil.NewPolicy() // 1 stage, 1 required
	requestStore, approvalStore, policyStore, auditStore := setupApproveTest(req, policy)

	// Simulate CAS failure: UpdateStatus returns ErrStatusConflict
	// (another checker already transitioned the request)
	requestStore.UpdateStatusFunc = func(ctx context.Context, id uuid.UUID, status model.RequestStatus) error {
		return store.ErrStatusConflict
	}

	svc := newTestRequestService(requestStore, approvalStore, policyStore, auditStore, nil)
	svc.SetWebhookDispatch(
		func(ctx context.Context, fn func(tx *store.Stores) error) error {
			// Inside the tx, count reflects the just-inserted vote.
			txApprovals := &testutil.MockApprovalStore{
				CreateFunc:                  approvalStore.CreateFunc,
				ListByRequestIDFunc:         approvalStore.ListByRequestIDFunc,
				ExistsByCheckerAndStageFunc: approvalStore.ExistsByCheckerAndStageFunc,
				CountByDecisionAndStageFunc: func(ctx context.Context, reqID uuid.UUID, decision model.Decision, stageIndex int) (int, error) {
					if decision == model.DecisionApproved {
						return 1, nil // 1 approved (just inserted) → triggers terminal path
					}
					return 0, nil
				},
			}
			return fn(&store.Stores{
				Requests:  requestStore,
				Approvals: txApprovals,
				Outbox:    &testutil.MockOutboxStore{},
				Webhooks:  &testutil.MockWebhookStore{},
			})
		},
		func(ctx context.Context, outbox store.OutboxStore, webhooks store.WebhookStore, r *model.Request, approvals []model.Approval) error {
			return nil
		},
		func() {},
	)

	_, err := svc.Approve(context.Background(), req.ID, "checker-1", nil, nil)
	if !errors.Is(err, ErrRequestNotPending) {
		t.Fatalf("expected ErrRequestNotPending on CAS conflict, got: %v", err)
	}
}

func TestApprove_StageAdvance_Atomic(t *testing.T) {
	req := testutil.NewRequest(func(r *model.Request) {
		r.CurrentStage = 0
	})
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.Stages = []model.ApprovalStage{
			{Index: 0, RequiredApprovals: 1, RejectionPolicy: model.RejectionPolicyAny},
			{Index: 1, RequiredApprovals: 1, RejectionPolicy: model.RejectionPolicyAny},
		}
	})
	_, approvalStore, policyStore, auditStore := setupApproveTest(req, policy)

	svc := newTestRequestService(&testutil.MockRequestStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Request, error) {
			return req, nil
		},
	}, approvalStore, policyStore, auditStore, nil)

	var txApprovalCreated, txStageUpdated bool
	svc.SetWebhookDispatch(
		func(ctx context.Context, fn func(tx *store.Stores) error) error {
			txStores := &store.Stores{
				Requests: &testutil.MockRequestStore{
					GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Request, error) {
						return req, nil
					},
					UpdateStageAndStatusFunc: func(ctx context.Context, id uuid.UUID, stage int, status model.RequestStatus) error {
						txStageUpdated = true
						return nil
					},
				},
				Approvals: &testutil.MockApprovalStore{
					CreateFunc: func(ctx context.Context, approval *model.Approval) error {
						txApprovalCreated = true
						return nil
					},
					CountByDecisionAndStageFunc: func(ctx context.Context, reqID uuid.UUID, decision model.Decision, stageIndex int) (int, error) {
						if decision == model.DecisionApproved {
							return 1, nil // 1 approved (just inserted) → meets stage 0 threshold
						}
						return 0, nil
					},
				},
			}
			return fn(txStores)
		},
		func(ctx context.Context, outbox store.OutboxStore, webhooks store.WebhookStore, r *model.Request, approvals []model.Approval) error {
			return nil
		},
		func() {},
	)

	result, err := svc.Approve(context.Background(), req.ID, "checker-1", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != model.StatusPending {
		t.Errorf("status = %v, want pending", result.Status)
	}
	if !txApprovalCreated {
		t.Error("expected approval to be created inside transaction")
	}
	if !txStageUpdated {
		t.Error("expected stage to be updated inside transaction")
	}
}

func TestApprove_MultiStage_PerStageRoleValidation(t *testing.T) {
	req := testutil.NewRequest(func(r *model.Request) {
		r.CurrentStage = 1
	})
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.Stages = []model.ApprovalStage{
			{Index: 0, RequiredApprovals: 1, RejectionPolicy: model.RejectionPolicyAny, AllowedCheckerRoles: json.RawMessage(`["finance"]`)},
			{Index: 1, RequiredApprovals: 1, RejectionPolicy: model.RejectionPolicyAny, AllowedCheckerRoles: json.RawMessage(`["manager"]`)},
		}
	})
	requestStore, approvalStore, policyStore, auditStore := setupApproveTest(req, policy)

	svc := newTestRequestService(requestStore, approvalStore, policyStore, auditStore, nil)

	// Stage 1 requires "manager" role, but checker has "finance"
	_, err := svc.Approve(context.Background(), req.ID, "checker-1", []string{"finance"}, nil)
	if !errors.Is(err, ErrInvalidCheckerRole) {
		t.Fatalf("expected ErrInvalidCheckerRole for stage 1, got: %v", err)
	}

	// Now with correct role
	_, err = svc.Approve(context.Background(), req.ID, "checker-1", []string{"manager"}, nil)
	if err != nil {
		t.Fatalf("unexpected error with correct role: %v", err)
	}
}

// --- CanViewerAct Tests ---

func TestCanViewerAct_ValidChecker(t *testing.T) {
	req := testutil.NewRequest(func(r *model.Request) {
		r.Approvals = nil
	})
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.RequestType = req.Type
	})
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) {
			return policy, nil
		},
	}
	svc := newTestRequestService(&testutil.MockRequestStore{}, &testutil.MockApprovalStore{}, policies, &testutil.MockAuditStore{}, nil)

	if !svc.CanViewerAct(context.Background(), req, "checker-1", nil, nil) {
		t.Error("expected true for valid checker with no role restrictions")
	}
}

func TestCanViewerAct_EmptyViewerID(t *testing.T) {
	req := testutil.NewRequest()
	svc := newTestRequestService(&testutil.MockRequestStore{}, &testutil.MockApprovalStore{}, &testutil.MockPolicyStore{}, &testutil.MockAuditStore{}, nil)

	if svc.CanViewerAct(context.Background(), req, "", nil, nil) {
		t.Error("expected false for empty viewer ID")
	}
}

func TestCanViewerAct_NotPending(t *testing.T) {
	req := testutil.NewRequest(func(r *model.Request) {
		r.Status = model.StatusApproved
	})
	svc := newTestRequestService(&testutil.MockRequestStore{}, &testutil.MockApprovalStore{}, &testutil.MockPolicyStore{}, &testutil.MockAuditStore{}, nil)

	if svc.CanViewerAct(context.Background(), req, "checker-1", nil, nil) {
		t.Error("expected false for non-pending request")
	}
}

func TestCanViewerAct_IsMaker(t *testing.T) {
	req := testutil.NewRequest()
	svc := newTestRequestService(&testutil.MockRequestStore{}, &testutil.MockApprovalStore{}, &testutil.MockPolicyStore{}, &testutil.MockAuditStore{}, nil)

	if svc.CanViewerAct(context.Background(), req, req.MakerID, nil, nil) {
		t.Error("expected false when viewer is the maker")
	}
}

func TestCanViewerAct_AlreadyVoted(t *testing.T) {
	req := testutil.NewRequest(func(r *model.Request) {
		r.CurrentStage = 0
		r.Approvals = []model.Approval{
			{CheckerID: "checker-1", StageIndex: 0, Decision: model.DecisionApproved},
		}
	})
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.RequestType = req.Type
	})
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) {
			return policy, nil
		},
	}
	svc := newTestRequestService(&testutil.MockRequestStore{}, &testutil.MockApprovalStore{}, policies, &testutil.MockAuditStore{}, nil)

	if svc.CanViewerAct(context.Background(), req, "checker-1", nil, nil) {
		t.Error("expected false when viewer already voted on current stage")
	}
}

func TestCanViewerAct_VotedOnDifferentStage(t *testing.T) {
	req := testutil.NewRequest(func(r *model.Request) {
		r.CurrentStage = 1
		r.Approvals = []model.Approval{
			{CheckerID: "checker-1", StageIndex: 0, Decision: model.DecisionApproved},
		}
	})
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.RequestType = req.Type
		p.Stages = []model.ApprovalStage{
			{Index: 0, RequiredApprovals: 1, RejectionPolicy: model.RejectionPolicyAny},
			{Index: 1, RequiredApprovals: 1, RejectionPolicy: model.RejectionPolicyAny},
		}
	})
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) {
			return policy, nil
		},
	}
	svc := newTestRequestService(&testutil.MockRequestStore{}, &testutil.MockApprovalStore{}, policies, &testutil.MockAuditStore{}, nil)

	if !svc.CanViewerAct(context.Background(), req, "checker-1", nil, nil) {
		t.Error("expected true when viewer voted on a different stage")
	}
}

func TestCanViewerAct_WrongRole(t *testing.T) {
	req := testutil.NewRequest()
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.RequestType = req.Type
		p.Stages = []model.ApprovalStage{
			{
				Index:               0,
				RequiredApprovals:   1,
				AllowedCheckerRoles: json.RawMessage(`["manager"]`),
				RejectionPolicy:     model.RejectionPolicyAny,
			},
		}
	})
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) {
			return policy, nil
		},
	}
	svc := newTestRequestService(&testutil.MockRequestStore{}, &testutil.MockApprovalStore{}, policies, &testutil.MockAuditStore{}, nil)

	if svc.CanViewerAct(context.Background(), req, "checker-1", []string{"employee"}, nil) {
		t.Error("expected false when viewer lacks required role")
	}
}

func TestCanViewerAct_CorrectRole(t *testing.T) {
	req := testutil.NewRequest()
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.RequestType = req.Type
		p.Stages = []model.ApprovalStage{
			{
				Index:               0,
				RequiredApprovals:   1,
				AllowedCheckerRoles: json.RawMessage(`["manager"]`),
				RejectionPolicy:     model.RejectionPolicyAny,
			},
		}
	})
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) {
			return policy, nil
		},
	}
	svc := newTestRequestService(&testutil.MockRequestStore{}, &testutil.MockApprovalStore{}, policies, &testutil.MockAuditStore{}, nil)

	if !svc.CanViewerAct(context.Background(), req, "checker-1", []string{"manager"}, nil) {
		t.Error("expected true when viewer has required role")
	}
}

func TestCanViewerAct_NotInEligibleReviewers(t *testing.T) {
	req := testutil.NewRequest(func(r *model.Request) {
		r.EligibleReviewers = []string{"reviewer-a", "reviewer-b"}
	})
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.RequestType = req.Type
	})
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) {
			return policy, nil
		},
	}
	svc := newTestRequestService(&testutil.MockRequestStore{}, &testutil.MockApprovalStore{}, policies, &testutil.MockAuditStore{}, nil)

	if svc.CanViewerAct(context.Background(), req, "checker-1", nil, nil) {
		t.Error("expected false when viewer is not in eligible reviewers list")
	}
}

func TestCanViewerAct_InEligibleReviewers(t *testing.T) {
	req := testutil.NewRequest(func(r *model.Request) {
		r.EligibleReviewers = []string{"reviewer-a", "checker-1"}
	})
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.RequestType = req.Type
	})
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) {
			return policy, nil
		},
	}
	svc := newTestRequestService(&testutil.MockRequestStore{}, &testutil.MockApprovalStore{}, policies, &testutil.MockAuditStore{}, nil)

	if !svc.CanViewerAct(context.Background(), req, "checker-1", nil, nil) {
		t.Error("expected true when viewer is in eligible reviewers list")
	}
}

func TestCanViewerAct_PolicyNotFound(t *testing.T) {
	req := testutil.NewRequest()
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) {
			return nil, nil
		},
	}
	svc := newTestRequestService(&testutil.MockRequestStore{}, &testutil.MockApprovalStore{}, policies, &testutil.MockAuditStore{}, nil)

	if svc.CanViewerAct(context.Background(), req, "checker-1", nil, nil) {
		t.Error("expected false when policy not found")
	}
}

// --- Permission-based Authorization Tests ---

func TestCanViewerAct_CorrectPermission(t *testing.T) {
	req := testutil.NewRequest()
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.RequestType = req.Type
		p.Stages = []model.ApprovalStage{
			{
				Index:              0,
				RequiredApprovals:  1,
				AllowedPermissions: json.RawMessage(`["approve_transfer"]`),
				RejectionPolicy:    model.RejectionPolicyAny,
			},
		}
	})
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) {
			return policy, nil
		},
	}
	svc := newTestRequestService(&testutil.MockRequestStore{}, &testutil.MockApprovalStore{}, policies, &testutil.MockAuditStore{}, nil)

	if !svc.CanViewerAct(context.Background(), req, "checker-1", nil, []string{"approve_transfer"}) {
		t.Error("expected true when viewer has required permission")
	}
}

func TestCanViewerAct_WrongPermission(t *testing.T) {
	req := testutil.NewRequest()
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.RequestType = req.Type
		p.Stages = []model.ApprovalStage{
			{
				Index:              0,
				RequiredApprovals:  1,
				AllowedPermissions: json.RawMessage(`["approve_transfer"]`),
				RejectionPolicy:    model.RejectionPolicyAny,
			},
		}
	})
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) {
			return policy, nil
		},
	}
	svc := newTestRequestService(&testutil.MockRequestStore{}, &testutil.MockApprovalStore{}, policies, &testutil.MockAuditStore{}, nil)

	if svc.CanViewerAct(context.Background(), req, "checker-1", nil, []string{"read_only"}) {
		t.Error("expected false when viewer lacks required permission")
	}
}

func TestCanViewerAct_AuthorizationModeAny(t *testing.T) {
	req := testutil.NewRequest()
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.RequestType = req.Type
		p.Stages = []model.ApprovalStage{
			{
				Index:               0,
				RequiredApprovals:   1,
				AllowedCheckerRoles: json.RawMessage(`["manager"]`),
				AllowedPermissions:  json.RawMessage(`["approve_transfer"]`),
				AuthorizationMode:   model.AuthModeAny,
				RejectionPolicy:     model.RejectionPolicyAny,
			},
		}
	})
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) {
			return policy, nil
		},
	}
	svc := newTestRequestService(&testutil.MockRequestStore{}, &testutil.MockApprovalStore{}, policies, &testutil.MockAuditStore{}, nil)

	// Has permission but not role — should pass with mode "any"
	if !svc.CanViewerAct(context.Background(), req, "checker-1", []string{"employee"}, []string{"approve_transfer"}) {
		t.Error("expected true: viewer has matching permission (any mode)")
	}
	// Has role but not permission — should pass with mode "any"
	if !svc.CanViewerAct(context.Background(), req, "checker-1", []string{"manager"}, []string{"read_only"}) {
		t.Error("expected true: viewer has matching role (any mode)")
	}
	// Has neither — should fail
	if svc.CanViewerAct(context.Background(), req, "checker-1", []string{"employee"}, []string{"read_only"}) {
		t.Error("expected false: viewer has neither matching role nor permission")
	}
}

func TestCanViewerAct_AuthorizationModeAll(t *testing.T) {
	req := testutil.NewRequest()
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.RequestType = req.Type
		p.Stages = []model.ApprovalStage{
			{
				Index:               0,
				RequiredApprovals:   1,
				AllowedCheckerRoles: json.RawMessage(`["manager"]`),
				AllowedPermissions:  json.RawMessage(`["approve_transfer"]`),
				AuthorizationMode:   model.AuthModeAll,
				RejectionPolicy:     model.RejectionPolicyAny,
			},
		}
	})
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) {
			return policy, nil
		},
	}
	svc := newTestRequestService(&testutil.MockRequestStore{}, &testutil.MockApprovalStore{}, policies, &testutil.MockAuditStore{}, nil)

	// Has both — should pass
	if !svc.CanViewerAct(context.Background(), req, "checker-1", []string{"manager"}, []string{"approve_transfer"}) {
		t.Error("expected true: viewer has both matching role and permission (all mode)")
	}
	// Has only role — should fail
	if svc.CanViewerAct(context.Background(), req, "checker-1", []string{"manager"}, []string{"read_only"}) {
		t.Error("expected false: viewer only has role but not permission (all mode)")
	}
	// Has only permission — should fail
	if svc.CanViewerAct(context.Background(), req, "checker-1", []string{"employee"}, []string{"approve_transfer"}) {
		t.Error("expected false: viewer only has permission but not role (all mode)")
	}
}
