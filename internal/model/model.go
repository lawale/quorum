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
	ID             uuid.UUID       `json:"id"`
	TenantID       string          `json:"tenant_id"`
	IdempotencyKey *string         `json:"idempotency_key,omitempty"`
	Type           string          `json:"type"`
	Payload        json.RawMessage `json:"payload"`
	Status         RequestStatus   `json:"status"`
	MakerID        string          `json:"maker_id"`
	// MakerAttributes are the maker's identity attributes captured at creation
	// (for example branch_id). Stage scope rules compare checkers against them.
	MakerAttributes   map[string]string `json:"maker_attributes,omitempty"`
	EligibleReviewers []string          `json:"eligible_reviewers,omitempty"`
	Metadata          json.RawMessage   `json:"metadata,omitempty"`
	Fingerprint       *string           `json:"fingerprint,omitempty"`
	CurrentStage      int               `json:"current_stage"`
	ExpiresAt         *time.Time        `json:"expires_at,omitempty"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
	Approvals         []Approval        `json:"approvals,omitempty"`
	ViewerCanAct      *bool             `json:"viewer_can_act,omitempty"`
	TotalStages       *int              `json:"total_stages,omitempty"`
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

type AuthorizationMode string

const (
	AuthModeRole       AuthorizationMode = "role"
	AuthModePermission AuthorizationMode = "permission"
	AuthModeAny        AuthorizationMode = "any"
	AuthModeAll        AuthorizationMode = "all"
)

// ScopeRule restricts a stage to checkers whose identity attribute equals one
// of the values the rule's sources produce. A rule may draw from the maker's
// attribute of the same name (captured at creation), from a dot-notation path
// into the payload, and from fixed values; it passes when the checker matches
// any of them. When no source is set the rule matches the maker. Multiple
// rules on a stage must all pass.
type ScopeRule struct {
	Attribute   string   `json:"attribute"`
	MatchMaker  bool     `json:"match_maker,omitempty"`
	PayloadPath string   `json:"payload_path,omitempty"`
	Values      []string `json:"values,omitempty"`
	// AllowUnscopedCheckers lets a checker who has no value for the attribute
	// (for example a head-office user with no branch) act on any request.
	AllowUnscopedCheckers bool `json:"allow_unscoped_checkers,omitempty"`
}

// HasSource reports whether the rule names at least one source of expected values.
func (r ScopeRule) HasSource() bool {
	return r.MatchMaker || r.PayloadPath != "" || len(r.Values) > 0
}

type ApprovalStage struct {
	Index               int               `json:"index"`
	Name                string            `json:"name,omitempty"`
	RequiredApprovals   int               `json:"required_approvals"`
	AllowedCheckerRoles json.RawMessage   `json:"allowed_checker_roles,omitempty"`
	AllowedPermissions  json.RawMessage   `json:"allowed_permissions,omitempty"`
	AuthorizationMode   AuthorizationMode `json:"authorization_mode,omitempty"`
	RejectionPolicy     RejectionPolicy   `json:"rejection_policy"`
	MaxCheckers         *int              `json:"max_checkers,omitempty"`
	Scope               []ScopeRule       `json:"scope,omitempty"`
}

// BypassCondition is a comparison against a value in the request payload.
// Op is one of eq, ne, lt, lte, gt, gte, in. Ordering operators compare
// numbers only; eq, ne and in compare numbers, strings and booleans.
type BypassCondition struct {
	Path  string          `json:"path"`
	Op    string          `json:"op"`
	Value json.RawMessage `json:"value"`
}

// BypassSum caps the total of a numeric payload field across the window.
type BypassSum struct {
	Path  string  `json:"path"`
	Value float64 `json:"value"`
}

// BypassLimit is a velocity ceiling on a bypass rule. It counts requests of
// the same type created within Window and, by default, by the same maker.
// GroupBy widens the key: "maker" (default) and "payload:<path>" entries must
// all match for a prior request to count. When MaxCount or MaxSum would be
// exceeded by the new request, the rule does not match and the request goes
// through normal review. Window accepts Go durations plus a "d" suffix.
type BypassLimit struct {
	Window   string     `json:"window"`
	MaxCount *int       `json:"max_count,omitempty"`
	MaxSum   *BypassSum `json:"max_sum,omitempty"`
	GroupBy  []string   `json:"group_by,omitempty"`
}

// BypassRule describes requests that need no review. A rule matches when every
// condition it specifies holds for the maker and the payload: the maker holds
// any listed role, holds any listed permission, has each listed attribute set
// to one of the allowed values, and every payload condition is true. A policy's
// rules are tried in order and the first match auto-approves the request.
type BypassRule struct {
	Name             string              `json:"name,omitempty"`
	MakerRoles       []string            `json:"maker_roles,omitempty"`
	MakerPermissions []string            `json:"maker_permissions,omitempty"`
	MakerAttributes  map[string][]string `json:"maker_attributes,omitempty"`
	Payload          []BypassCondition   `json:"payload,omitempty"`
	Limits           []BypassLimit       `json:"limits,omitempty"`
}

// BypassLimitExceeded is written to request metadata when a bypass rule
// matched but one of its limits stopped it, so reviewers see why a request
// that would normally skip review is in front of them.
type BypassLimitExceeded struct {
	Rule     string   `json:"rule"`
	Window   string   `json:"window"`
	Measure  string   `json:"measure"` // "count" or "sum"
	Path     string   `json:"path,omitempty"`
	Observed float64  `json:"observed"` // including this request
	Limit    float64  `json:"limit"`
	GroupBy  []string `json:"group_by,omitempty"`
}

// HasCondition reports whether the rule constrains anything at all. A rule
// with no conditions would bypass every request and is rejected.
func (r BypassRule) HasCondition() bool {
	return len(r.MakerRoles) > 0 || len(r.MakerPermissions) > 0 || len(r.MakerAttributes) > 0 || len(r.Payload) > 0
}

type Policy struct {
	ID                         uuid.UUID       `json:"id"`
	TenantID                   string          `json:"tenant_id"`
	Name                       string          `json:"name"`
	RequestType                string          `json:"request_type"`
	Stages                     []ApprovalStage `json:"stages"`
	IdentityFields             []string        `json:"identity_fields,omitempty"`
	DynamicAuthorizationURL    *string         `json:"dynamic_authorization_url,omitempty"`
	DynamicAuthorizationSecret *string         `json:"-"`
	AutoExpireDuration         *time.Duration  `json:"auto_expire_duration,omitempty"`
	DisplayTemplate            json.RawMessage `json:"display_template,omitempty"`
	BypassRules                []BypassRule    `json:"bypass_rules,omitempty"`
	CreatedAt                  time.Time       `json:"created_at"`
	UpdatedAt                  time.Time       `json:"updated_at"`
	// Warnings are advisory notes produced when a policy is saved, such as a
	// scope rule naming an identity attribute the auth mode does not provide.
	// They are returned to the caller and never stored.
	Warnings []string `json:"warnings,omitempty"`
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

type AuthorizationHookRequest struct {
	RequestID         uuid.UUID         `json:"request_id"`
	RequestType       string            `json:"request_type"`
	CheckerID         string            `json:"checker_id"`
	CheckerAttributes map[string]string `json:"checker_attributes,omitempty"`
	MakerID           string            `json:"maker_id"`
	MakerAttributes   map[string]string `json:"maker_attributes,omitempty"`
	Payload           json.RawMessage   `json:"payload"`
}

type AuthorizationHookResponse struct {
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
	EventType     string          `json:"event_type"`
	Status        string          `json:"status"` // pending, delivered, failed
	Attempts      int             `json:"attempts"`
	MaxRetries    int             `json:"max_retries"`
	LastError     *string         `json:"last_error,omitempty"`
	NextRetryAt   time.Time       `json:"next_retry_at"`
	CreatedAt     time.Time       `json:"created_at"`
	DeliveredAt   *time.Time      `json:"delivered_at,omitempty"`
}
