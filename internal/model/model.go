package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type RequestStatus string

const (
	StatusPending   RequestStatus = "pending"
	StatusApproved  RequestStatus = "approved"
	StatusRejected  RequestStatus = "rejected"
	StatusCancelled RequestStatus = "cancelled"
	StatusExpired   RequestStatus = "expired"
)

func (s RequestStatus) IsTerminal() bool {
	return s == StatusApproved || s == StatusRejected || s == StatusCancelled || s == StatusExpired
}

type Decision string

const (
	DecisionApproved Decision = "approved"
	DecisionRejected Decision = "rejected"
)

type RejectionPolicy string

const (
	RejectionPolicyAny       RejectionPolicy = "any"
	RejectionPolicyThreshold RejectionPolicy = "threshold"
)

type Request struct {
	ID                uuid.UUID       `json:"id"`
	TenantID          string          `json:"tenant_id"`
	IdempotencyKey    *string         `json:"idempotency_key,omitempty"`
	Type              string          `json:"type"`
	Payload           json.RawMessage `json:"payload"`
	Status            RequestStatus   `json:"status"`
	MakerID           string          `json:"maker_id"`
	CallbackURL       *string         `json:"callback_url,omitempty"`
	EligibleReviewers []string        `json:"eligible_reviewers,omitempty"`
	Metadata          json.RawMessage `json:"metadata,omitempty"`
	Fingerprint       *string         `json:"fingerprint,omitempty"`
	CurrentStage      int             `json:"current_stage"`
	ExpiresAt         *time.Time      `json:"expires_at,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
	Approvals         []Approval      `json:"approvals,omitempty"`
}

type Approval struct {
	ID         uuid.UUID `json:"id"`
	RequestID  uuid.UUID `json:"request_id"`
	CheckerID  string    `json:"checker_id"`
	Decision   Decision  `json:"decision"`
	StageIndex int       `json:"stage_index"`
	Comment    *string   `json:"comment,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type ApprovalStage struct {
	Index               int             `json:"index"`
	Name                string          `json:"name,omitempty"`
	RequiredApprovals   int             `json:"required_approvals"`
	AllowedCheckerRoles json.RawMessage `json:"allowed_checker_roles,omitempty"`
	RejectionPolicy     RejectionPolicy `json:"rejection_policy"`
	MaxCheckers         *int            `json:"max_checkers,omitempty"`
}

type Policy struct {
	ID                 uuid.UUID       `json:"id"`
	TenantID           string          `json:"tenant_id"`
	Name               string          `json:"name"`
	RequestType        string          `json:"request_type"`
	Stages             []ApprovalStage `json:"stages"`
	IdentityFields     []string        `json:"identity_fields,omitempty"`
	PermissionCheckURL *string         `json:"permission_check_url,omitempty"`
	AutoExpireDuration *time.Duration  `json:"auto_expire_duration,omitempty"`
	DisplayTemplate    json.RawMessage `json:"display_template,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// StageAt returns the approval stage at the given index, or nil if out of range.
func (p *Policy) StageAt(index int) *ApprovalStage {
	if index < 0 || index >= len(p.Stages) {
		return nil
	}
	return &p.Stages[index]
}

// TotalStages returns the number of stages in this policy.
func (p *Policy) TotalStages() int {
	return len(p.Stages)
}

// PermissionCheckRequest is the payload sent to the consuming system's permission check endpoint.
type PermissionCheckRequest struct {
	RequestID    uuid.UUID       `json:"request_id"`
	RequestType  string          `json:"request_type"`
	CheckerID    string          `json:"checker_id"`
	CheckerRoles []string        `json:"checker_roles"`
	MakerID      string          `json:"maker_id"`
	Payload      json.RawMessage `json:"payload"`
}

// PermissionCheckResponse is the expected response from the permission check endpoint.
type PermissionCheckResponse struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

type Webhook struct {
	ID          uuid.UUID `json:"id"`
	TenantID    string    `json:"tenant_id"`
	URL         string    `json:"url"`
	Events      []string  `json:"events"`
	Secret      string    `json:"-"`
	RequestType *string   `json:"request_type,omitempty"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
}

type AuditLog struct {
	ID        uuid.UUID       `json:"id"`
	TenantID  string          `json:"tenant_id"`
	RequestID uuid.UUID       `json:"request_id"`
	Action    string          `json:"action"`
	ActorID   string          `json:"actor_id"`
	Details   json.RawMessage `json:"details,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

type WebhookPayload struct {
	Event     string     `json:"event"`
	Request   Request    `json:"request"`
	Approvals []Approval `json:"approvals,omitempty"`
	Timestamp time.Time  `json:"timestamp"`
}

// Tenant represents a registered tenant (application) in the system.
type Tenant struct {
	ID        uuid.UUID `json:"id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// OutboxEntry represents a durable webhook delivery entry in the outbox table.
type OutboxEntry struct {
	ID            uuid.UUID       `json:"id"`
	RequestID     uuid.UUID       `json:"request_id"`
	WebhookURL    string          `json:"webhook_url"`
	WebhookSecret string          `json:"-"`
	Payload       json.RawMessage `json:"payload"`
	Status        string          `json:"status"` // pending, delivered, failed
	Attempts      int             `json:"attempts"`
	MaxRetries    int             `json:"max_retries"`
	LastError     *string         `json:"last_error,omitempty"`
	NextRetryAt   time.Time       `json:"next_retry_at"`
	CreatedAt     time.Time       `json:"created_at"`
	DeliveredAt   *time.Time      `json:"delivered_at,omitempty"`
}
