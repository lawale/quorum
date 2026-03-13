package testutil

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/auth"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/store"
)

// MockRequestStore implements store.RequestStore with configurable function fields.
type MockRequestStore struct {
	CreateFunc                   func(ctx context.Context, req *model.Request) error
	GetByIDFunc                  func(ctx context.Context, id uuid.UUID) (*model.Request, error)
	GetByIdempotencyKeyFunc      func(ctx context.Context, key string) (*model.Request, error)
	FindPendingByFingerprintFunc func(ctx context.Context, reqType string, fingerprint string) (*model.Request, error)
	ListFunc                     func(ctx context.Context, filter store.RequestFilter) ([]model.Request, int, error)
	UpdateStatusFunc             func(ctx context.Context, id uuid.UUID, status model.RequestStatus) error
	ListExpiredFunc              func(ctx context.Context) ([]model.Request, error)
}

func (m *MockRequestStore) Create(ctx context.Context, req *model.Request) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, req)
	}
	panic("MockRequestStore.Create not set up")
}

func (m *MockRequestStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Request, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	panic("MockRequestStore.GetByID not set up")
}

func (m *MockRequestStore) GetByIdempotencyKey(ctx context.Context, key string) (*model.Request, error) {
	if m.GetByIdempotencyKeyFunc != nil {
		return m.GetByIdempotencyKeyFunc(ctx, key)
	}
	panic("MockRequestStore.GetByIdempotencyKey not set up")
}

func (m *MockRequestStore) FindPendingByFingerprint(ctx context.Context, reqType string, fingerprint string) (*model.Request, error) {
	if m.FindPendingByFingerprintFunc != nil {
		return m.FindPendingByFingerprintFunc(ctx, reqType, fingerprint)
	}
	panic("MockRequestStore.FindPendingByFingerprint not set up")
}

func (m *MockRequestStore) List(ctx context.Context, filter store.RequestFilter) ([]model.Request, int, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx, filter)
	}
	panic("MockRequestStore.List not set up")
}

func (m *MockRequestStore) UpdateStatus(ctx context.Context, id uuid.UUID, status model.RequestStatus) error {
	if m.UpdateStatusFunc != nil {
		return m.UpdateStatusFunc(ctx, id, status)
	}
	panic("MockRequestStore.UpdateStatus not set up")
}

func (m *MockRequestStore) ListExpired(ctx context.Context) ([]model.Request, error) {
	if m.ListExpiredFunc != nil {
		return m.ListExpiredFunc(ctx)
	}
	panic("MockRequestStore.ListExpired not set up")
}

// MockApprovalStore implements store.ApprovalStore with configurable function fields.
type MockApprovalStore struct {
	CreateFunc          func(ctx context.Context, approval *model.Approval) error
	ListByRequestIDFunc func(ctx context.Context, requestID uuid.UUID) ([]model.Approval, error)
	CountByDecisionFunc func(ctx context.Context, requestID uuid.UUID, decision model.Decision) (int, error)
	ExistsByCheckerFunc func(ctx context.Context, requestID uuid.UUID, checkerID string) (bool, error)
}

func (m *MockApprovalStore) Create(ctx context.Context, approval *model.Approval) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, approval)
	}
	panic("MockApprovalStore.Create not set up")
}

func (m *MockApprovalStore) ListByRequestID(ctx context.Context, requestID uuid.UUID) ([]model.Approval, error) {
	if m.ListByRequestIDFunc != nil {
		return m.ListByRequestIDFunc(ctx, requestID)
	}
	panic("MockApprovalStore.ListByRequestID not set up")
}

func (m *MockApprovalStore) CountByDecision(ctx context.Context, requestID uuid.UUID, decision model.Decision) (int, error) {
	if m.CountByDecisionFunc != nil {
		return m.CountByDecisionFunc(ctx, requestID, decision)
	}
	panic("MockApprovalStore.CountByDecision not set up")
}

func (m *MockApprovalStore) ExistsByChecker(ctx context.Context, requestID uuid.UUID, checkerID string) (bool, error) {
	if m.ExistsByCheckerFunc != nil {
		return m.ExistsByCheckerFunc(ctx, requestID, checkerID)
	}
	panic("MockApprovalStore.ExistsByChecker not set up")
}

// MockPolicyStore implements store.PolicyStore with configurable function fields.
type MockPolicyStore struct {
	CreateFunc           func(ctx context.Context, policy *model.Policy) error
	GetByIDFunc          func(ctx context.Context, id uuid.UUID) (*model.Policy, error)
	GetByRequestTypeFunc func(ctx context.Context, requestType string) (*model.Policy, error)
	ListFunc             func(ctx context.Context) ([]model.Policy, error)
	UpdateFunc           func(ctx context.Context, policy *model.Policy) error
	DeleteFunc           func(ctx context.Context, id uuid.UUID) error
}

func (m *MockPolicyStore) Create(ctx context.Context, policy *model.Policy) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, policy)
	}
	panic("MockPolicyStore.Create not set up")
}

func (m *MockPolicyStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Policy, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	panic("MockPolicyStore.GetByID not set up")
}

func (m *MockPolicyStore) GetByRequestType(ctx context.Context, requestType string) (*model.Policy, error) {
	if m.GetByRequestTypeFunc != nil {
		return m.GetByRequestTypeFunc(ctx, requestType)
	}
	panic("MockPolicyStore.GetByRequestType not set up")
}

func (m *MockPolicyStore) List(ctx context.Context) ([]model.Policy, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx)
	}
	panic("MockPolicyStore.List not set up")
}

func (m *MockPolicyStore) Update(ctx context.Context, policy *model.Policy) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, policy)
	}
	panic("MockPolicyStore.Update not set up")
}

func (m *MockPolicyStore) Delete(ctx context.Context, id uuid.UUID) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, id)
	}
	panic("MockPolicyStore.Delete not set up")
}

// MockWebhookStore implements store.WebhookStore with configurable function fields.
type MockWebhookStore struct {
	CreateFunc             func(ctx context.Context, webhook *model.Webhook) error
	GetByIDFunc            func(ctx context.Context, id uuid.UUID) (*model.Webhook, error)
	ListFunc               func(ctx context.Context) ([]model.Webhook, error)
	ListByEventAndTypeFunc func(ctx context.Context, event string, requestType string) ([]model.Webhook, error)
	DeleteFunc             func(ctx context.Context, id uuid.UUID) error
}

func (m *MockWebhookStore) Create(ctx context.Context, webhook *model.Webhook) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, webhook)
	}
	panic("MockWebhookStore.Create not set up")
}

func (m *MockWebhookStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Webhook, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	panic("MockWebhookStore.GetByID not set up")
}

func (m *MockWebhookStore) List(ctx context.Context) ([]model.Webhook, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx)
	}
	panic("MockWebhookStore.List not set up")
}

func (m *MockWebhookStore) ListByEventAndType(ctx context.Context, event string, requestType string) ([]model.Webhook, error) {
	if m.ListByEventAndTypeFunc != nil {
		return m.ListByEventAndTypeFunc(ctx, event, requestType)
	}
	panic("MockWebhookStore.ListByEventAndType not set up")
}

func (m *MockWebhookStore) Delete(ctx context.Context, id uuid.UUID) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, id)
	}
	panic("MockWebhookStore.Delete not set up")
}

// MockAuditStore implements store.AuditStore with configurable function fields.
type MockAuditStore struct {
	CreateFunc          func(ctx context.Context, log *model.AuditLog) error
	ListByRequestIDFunc func(ctx context.Context, requestID uuid.UUID) ([]model.AuditLog, error)
}

func (m *MockAuditStore) Create(ctx context.Context, log *model.AuditLog) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, log)
	}
	panic("MockAuditStore.Create not set up")
}

func (m *MockAuditStore) ListByRequestID(ctx context.Context, requestID uuid.UUID) ([]model.AuditLog, error) {
	if m.ListByRequestIDFunc != nil {
		return m.ListByRequestIDFunc(ctx, requestID)
	}
	panic("MockAuditStore.ListByRequestID not set up")
}

// MockAuthProvider implements auth.Provider with configurable function fields.
type MockAuthProvider struct {
	AuthenticateFunc func(r *http.Request) (*auth.Identity, error)
}

func (m *MockAuthProvider) Authenticate(r *http.Request) (*auth.Identity, error) {
	if m.AuthenticateFunc != nil {
		return m.AuthenticateFunc(r)
	}
	panic("MockAuthProvider.Authenticate not set up")
}
