package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/model"
)

// ErrStatusConflict is returned by UpdateStatus / UpdateStageAndStatus when
// the request is no longer in 'pending' status. This acts as a compare-and-set
// guard, preventing double terminal transitions from concurrent approvers.
var ErrStatusConflict = errors.New("request status has already changed")

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
	UpdateStageAndStatus(ctx context.Context, id uuid.UUID, stage int, status model.RequestStatus) error
	ListExpired(ctx context.Context) ([]model.Request, error)
}

type ApprovalStore interface {
	Create(ctx context.Context, approval *model.Approval) error
	ListByRequestID(ctx context.Context, requestID uuid.UUID) ([]model.Approval, error)
	CountByDecisionAndStage(ctx context.Context, requestID uuid.UUID, decision model.Decision, stageIndex int) (int, error)
	ExistsByCheckerAndStage(ctx context.Context, requestID uuid.UUID, checkerID string, stageIndex int) (bool, error)
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

type OperatorStore interface {
	Create(ctx context.Context, operator *model.Operator) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Operator, error)
	GetByUsername(ctx context.Context, username string) (*model.Operator, error)
	List(ctx context.Context) ([]model.Operator, error)
	Update(ctx context.Context, operator *model.Operator) error
	Delete(ctx context.Context, id uuid.UUID) error
	Count(ctx context.Context) (int, error)
}

// OutboxStore manages durable webhook delivery entries.
type OutboxStore interface {
	CreateBatch(ctx context.Context, entries []model.OutboxEntry) error
	// ClaimBatch atomically selects and locks up to `limit` pending entries for
	// delivery, transitioning their status to 'processing'. This prevents
	// duplicate deliveries across multiple app instances.
	ClaimBatch(ctx context.Context, limit int) ([]model.OutboxEntry, error)
	MarkDelivered(ctx context.Context, id uuid.UUID) error
	MarkRetry(ctx context.Context, id uuid.UUID, attempts int, lastError string, nextRetryAt time.Time) error
	MarkFailed(ctx context.Context, id uuid.UUID, attempts int, lastError string) error
	DeleteDelivered(ctx context.Context, olderThan time.Time) (int64, error)
}

// Stores bundles every store interface and a Close function for the underlying connection.
type Stores struct {
	Requests  RequestStore
	Approvals ApprovalStore
	Policies  PolicyStore
	Webhooks  WebhookStore
	Audits    AuditStore
	Operators OperatorStore
	Outbox    OutboxStore
	Close     func()

	// RunInTx executes fn within a database transaction. The callback receives
	// a Stores instance whose stores are bound to the transaction. If fn returns
	// nil the transaction is committed; otherwise it is rolled back.
	RunInTx func(ctx context.Context, fn func(tx *Stores) error) error
}
