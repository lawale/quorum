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

// newTestRequestService creates a RequestService with mock stores and optional PermissionChecker.
func newTestRequestService(
	requests *testutil.MockRequestStore,
	approvals *testutil.MockApprovalStore,
	policies *testutil.MockPolicyStore,
	audits *testutil.MockAuditStore,
	permChecker *auth.PermissionChecker,
) *RequestService {
	return NewRequestService(requests, approvals, policies, audits, permChecker)
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
	}
	approvals := &testutil.MockApprovalStore{
		ExistsByCheckerFunc: func(ctx context.Context, reqID uuid.UUID, checkerID string) (bool, error) {
			return false, nil
		},
		CreateFunc: func(ctx context.Context, approval *model.Approval) error {
			return nil
		},
		CountByDecisionFunc: func(ctx context.Context, reqID uuid.UUID, decision model.Decision) (int, error) {
			if decision == model.DecisionApproved {
				return 1, nil // This approval is the first
			}
			return 0, nil
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
	policy := testutil.NewPolicy(func(p *model.Policy) { p.RequiredApprovals = 1 })
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
	policy := testutil.NewPolicy(func(p *model.Policy) { p.RequiredApprovals = 3 })
	requestStore, approvalStore, policyStore, auditStore := setupApproveTest(req, policy)

	// Override: only 1 approval so far, need 3
	approvalStore.CountByDecisionFunc = func(ctx context.Context, reqID uuid.UUID, decision model.Decision) (int, error) {
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
	requests := &testutil.MockRequestStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Request, error) {
			return req, nil
		},
	}
	approvals := &testutil.MockApprovalStore{
		ExistsByCheckerFunc: func(ctx context.Context, reqID uuid.UUID, checkerID string) (bool, error) {
			return true, nil // Already acted
		},
	}
	svc := newTestRequestService(requests, approvals, &testutil.MockPolicyStore{}, &testutil.MockAuditStore{}, nil)

	_, err := svc.Approve(context.Background(), req.ID, "checker-1", nil, nil)
	if !errors.Is(err, ErrAlreadyActioned) {
		t.Fatalf("expected ErrAlreadyActioned, got: %v", err)
	}
}

func TestApprove_InvalidCheckerRole(t *testing.T) {
	req := testutil.NewRequest()
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.AllowedCheckerRoles = json.RawMessage(`["admin","manager"]`)
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
		p.AllowedCheckerRoles = json.RawMessage(`["admin","manager"]`)
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
	policy := testutil.NewPolicy() // No AllowedCheckerRoles
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

func TestApprove_PermissionCheck_Denied(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(model.PermissionCheckResponse{Allowed: false, Reason: "not authorized"})
	}))
	defer server.Close()

	req := testutil.NewRequest()
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.PermissionCheckURL = testutil.StringPtr(server.URL)
	})
	requestStore, approvalStore, policyStore, auditStore := setupApproveTest(req, policy)
	checker := auth.NewPermissionChecker(5 * time.Second)

	svc := newTestRequestService(requestStore, approvalStore, policyStore, auditStore, checker)

	_, err := svc.Approve(context.Background(), req.ID, "checker-1", nil, nil)
	if !errors.Is(err, auth.ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied, got: %v", err)
	}
}

func TestApprove_PermissionCheck_Allowed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(model.PermissionCheckResponse{Allowed: true})
	}))
	defer server.Close()

	req := testutil.NewRequest()
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.PermissionCheckURL = testutil.StringPtr(server.URL)
	})
	requestStore, approvalStore, policyStore, auditStore := setupApproveTest(req, policy)
	checker := auth.NewPermissionChecker(5 * time.Second)

	svc := newTestRequestService(requestStore, approvalStore, policyStore, auditStore, checker)

	_, err := svc.Approve(context.Background(), req.ID, "checker-1", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApprove_PermissionCheck_NoURL_Skips(t *testing.T) {
	req := testutil.NewRequest()
	policy := testutil.NewPolicy() // No PermissionCheckURL
	requestStore, approvalStore, policyStore, auditStore := setupApproveTest(req, policy)
	checker := auth.NewPermissionChecker(5 * time.Second)

	svc := newTestRequestService(requestStore, approvalStore, policyStore, auditStore, checker)

	_, err := svc.Approve(context.Background(), req.ID, "checker-1", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApprove_OnResolve_CalledOnTerminal(t *testing.T) {
	req := testutil.NewRequest()
	policy := testutil.NewPolicy(func(p *model.Policy) { p.RequiredApprovals = 1 })
	requestStore, approvalStore, policyStore, auditStore := setupApproveTest(req, policy)

	svc := newTestRequestService(requestStore, approvalStore, policyStore, auditStore, nil)

	called := false
	svc.SetOnResolve(func(ctx context.Context, r *model.Request, approvals []model.Approval) {
		called = true
	})

	_, err := svc.Approve(context.Background(), req.ID, "checker-1", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected onResolve to be called")
	}
}

func TestApprove_OnResolve_NotCalledWhenPending(t *testing.T) {
	req := testutil.NewRequest()
	policy := testutil.NewPolicy(func(p *model.Policy) { p.RequiredApprovals = 3 })
	requestStore, approvalStore, policyStore, auditStore := setupApproveTest(req, policy)

	// Override: 1 approval, need 3
	approvalStore.CountByDecisionFunc = func(ctx context.Context, reqID uuid.UUID, decision model.Decision) (int, error) {
		if decision == model.DecisionApproved {
			return 1, nil
		}
		return 0, nil
	}

	svc := newTestRequestService(requestStore, approvalStore, policyStore, auditStore, nil)

	called := false
	svc.SetOnResolve(func(ctx context.Context, r *model.Request, approvals []model.Approval) {
		called = true
	})

	_, err := svc.Approve(context.Background(), req.ID, "checker-1", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("onResolve should not be called when status is still pending")
	}
}

// --- Reject Tests ---

func TestReject_RejectionPolicyAny_ImmediateReject(t *testing.T) {
	req := testutil.NewRequest()
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.RequiredApprovals = 2
		p.RejectionPolicy = model.RejectionPolicyAny
	})
	requestStore, approvalStore, policyStore, auditStore := setupApproveTest(req, policy)

	// Override counts for rejection: 0 approvals, 1 rejection
	approvalStore.CountByDecisionFunc = func(ctx context.Context, reqID uuid.UUID, decision model.Decision) (int, error) {
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
		p.RequiredApprovals = 2
		p.RejectionPolicy = model.RejectionPolicyThreshold
		p.MaxCheckers = testutil.IntPtr(3)
	})
	requestStore, approvalStore, policyStore, auditStore := setupApproveTest(req, policy)

	// 0 approvals, 1 rejection. Remaining = 3 - 0 - 1 = 2. 0 + 2 >= 2 → still possible
	approvalStore.CountByDecisionFunc = func(ctx context.Context, reqID uuid.UUID, decision model.Decision) (int, error) {
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
	if result.Status != model.StatusPending {
		t.Errorf("status = %v, want pending (still achievable)", result.Status)
	}
}

func TestReject_RejectionPolicyThreshold_Impossible(t *testing.T) {
	req := testutil.NewRequest()
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.RequiredApprovals = 3
		p.RejectionPolicy = model.RejectionPolicyThreshold
		p.MaxCheckers = testutil.IntPtr(3)
	})
	requestStore, approvalStore, policyStore, auditStore := setupApproveTest(req, policy)

	// 0 approvals, 1 rejection. Remaining = 3 - 0 - 1 = 2. 0 + 2 < 3 → impossible
	approvalStore.CountByDecisionFunc = func(ctx context.Context, reqID uuid.UUID, decision model.Decision) (int, error) {
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

func TestCancel_OnResolve_Called(t *testing.T) {
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

	called := false
	svc.SetOnResolve(func(ctx context.Context, r *model.Request, approvals []model.Approval) {
		called = true
	})

	_, err := svc.Cancel(context.Background(), req.ID, "maker-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected onResolve to be called on cancel")
	}
}

// --- validateCheckerRoles Tests ---

func TestValidateCheckerRoles_NilRoles(t *testing.T) {
	policy := testutil.NewPolicy() // AllowedCheckerRoles is nil
	err := validateCheckerRoles(policy, []string{"anything"})
	if err != nil {
		t.Fatalf("expected nil error for nil AllowedCheckerRoles, got: %v", err)
	}
}

func TestValidateCheckerRoles_EmptyArray(t *testing.T) {
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.AllowedCheckerRoles = json.RawMessage(`[]`)
	})
	err := validateCheckerRoles(policy, []string{"anything"})
	if err != nil {
		t.Fatalf("expected nil error for empty allowed roles, got: %v", err)
	}
}

func TestValidateCheckerRoles_Match(t *testing.T) {
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.AllowedCheckerRoles = json.RawMessage(`["admin","manager"]`)
	})
	err := validateCheckerRoles(policy, []string{"manager"})
	if err != nil {
		t.Fatalf("expected nil error for matching role, got: %v", err)
	}
}

func TestValidateCheckerRoles_NoMatch(t *testing.T) {
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.AllowedCheckerRoles = json.RawMessage(`["admin","manager"]`)
	})
	err := validateCheckerRoles(policy, []string{"viewer"})
	if !errors.Is(err, ErrInvalidCheckerRole) {
		t.Fatalf("expected ErrInvalidCheckerRole, got: %v", err)
	}
}
