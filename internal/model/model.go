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
	IdempotencyKey    *string         `json:"idempotency_key,omitempty"`
	Type              string          `json:"type"`
	Payload           json.RawMessage `json:"payload"`
	Status            RequestStatus   `json:"status"`
	MakerID           string          `json:"maker_id"`
	CallbackURL       *string         `json:"callback_url,omitempty"`
	EligibleReviewers []string        `json:"eligible_reviewers,omitempty"`
	Metadata          json.RawMessage `json:"metadata,omitempty"`
	Fingerprint       *string         `json:"fingerprint,omitempty"`
	ExpiresAt         *time.Time      `json:"expires_at,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
	Approvals         []Approval      `json:"approvals,omitempty"`
}

type Approval struct {
	ID        uuid.UUID `json:"id"`
	RequestID uuid.UUID `json:"request_id"`
	CheckerID string    `json:"checker_id"`
	Decision  Decision  `json:"decision"`
	Comment   *string   `json:"comment,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Policy struct {
	ID                  uuid.UUID       `json:"id"`
	Name                string          `json:"name"`
	RequestType         string          `json:"request_type"`
	RequiredApprovals   int             `json:"required_approvals"`
	AllowedCheckerRoles json.RawMessage `json:"allowed_checker_roles,omitempty"`
	RejectionPolicy     RejectionPolicy `json:"rejection_policy"`
	MaxCheckers         *int            `json:"max_checkers,omitempty"`
	IdentityFields      []string        `json:"identity_fields,omitempty"`
	PermissionCheckURL  *string         `json:"permission_check_url,omitempty"`
	AutoExpireDuration  *time.Duration  `json:"auto_expire_duration,omitempty"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}

// PermissionCheckRequest is the payload sent to the consuming system's permission check endpoint.
type PermissionCheckRequest struct {
	RequestID   uuid.UUID       `json:"request_id"`
	RequestType string          `json:"request_type"`
	CheckerID   string          `json:"checker_id"`
	CheckerRoles []string       `json:"checker_roles"`
	MakerID     string          `json:"maker_id"`
	Payload     json.RawMessage `json:"payload"`
}

// PermissionCheckResponse is the expected response from the permission check endpoint.
type PermissionCheckResponse struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

type Webhook struct {
	ID          uuid.UUID `json:"id"`
	URL         string    `json:"url"`
	Events      []string  `json:"events"`
	Secret      string    `json:"-"`
	RequestType *string   `json:"request_type,omitempty"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
}

type AuditLog struct {
	ID        uuid.UUID       `json:"id"`
	RequestID uuid.UUID       `json:"request_id"`
	Action    string          `json:"action"`
	ActorID   string          `json:"actor_id"`
	Details   json.RawMessage `json:"details,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

type WebhookPayload struct {
	Event     string    `json:"event"`
	Request   Request   `json:"request"`
	Approvals []Approval `json:"approvals,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}
