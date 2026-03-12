package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/model"
)

type RequestFilter struct {
	Status  *model.RequestStatus
	Type    *string
	MakerID *string
	Page    int
	PerPage int
}

type RequestStore interface {
	Create(ctx context.Context, req *model.Request) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Request, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*model.Request, error)
	FindPendingByFingerprint(ctx context.Context, reqType string, fingerprint string) (*model.Request, error)
	List(ctx context.Context, filter RequestFilter) ([]model.Request, int, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.RequestStatus) error
	ListExpired(ctx context.Context) ([]model.Request, error)
}

type ApprovalStore interface {
	Create(ctx context.Context, approval *model.Approval) error
	ListByRequestID(ctx context.Context, requestID uuid.UUID) ([]model.Approval, error)
	CountByDecision(ctx context.Context, requestID uuid.UUID, decision model.Decision) (int, error)
	ExistsByChecker(ctx context.Context, requestID uuid.UUID, checkerID string) (bool, error)
}

type PolicyStore interface {
	Create(ctx context.Context, policy *model.Policy) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Policy, error)
	GetByRequestType(ctx context.Context, requestType string) (*model.Policy, error)
	List(ctx context.Context) ([]model.Policy, error)
	Update(ctx context.Context, policy *model.Policy) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type WebhookStore interface {
	Create(ctx context.Context, webhook *model.Webhook) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Webhook, error)
	List(ctx context.Context) ([]model.Webhook, error)
	ListByEventAndType(ctx context.Context, event string, requestType string) ([]model.Webhook, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type AuditStore interface {
	Create(ctx context.Context, log *model.AuditLog) error
	ListByRequestID(ctx context.Context, requestID uuid.UUID) ([]model.AuditLog, error)
}
