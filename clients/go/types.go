package client

import (
	"github.com/lawale/quorum/internal/health"
	"github.com/lawale/quorum/internal/model"
)

// The API's types, shared with the server.
type (
	Request       = model.Request
	RequestStatus = model.RequestStatus
	Approval      = model.Approval
	Decision      = model.Decision
	AuditLog      = model.AuditLog

	Policy          = model.Policy
	PolicyInput     = model.PolicyInput
	PolicyUpdate    = model.PolicyUpdate
	ApprovalStage   = model.ApprovalStage
	ScopeRule       = model.ScopeRule
	BypassRule      = model.BypassRule
	BypassCondition = model.BypassCondition
	BypassLimit     = model.BypassLimit
	BypassSum       = model.BypassSum
	Duration        = model.Duration

	Webhook        = model.Webhook
	WebhookInput   = model.WebhookInput
	WebhookPayload = model.WebhookPayload
	ReviewRequest  = model.ReviewRequest
	ResolvedScope  = model.ResolvedScope

	ComponentStatus = health.ComponentStatus
)

// Page is one page of a list endpoint.
type Page[T any] = model.Page[T]

// Request statuses.
const (
	StatusPending   = model.StatusPending
	StatusApproved  = model.StatusApproved
	StatusRejected  = model.StatusRejected
	StatusCancelled = model.StatusCancelled
	StatusExpired   = model.StatusExpired
)

// Webhook event names. The status events share their name with the status.
const (
	EventReviewRequested = model.EventReviewRequested
	EventApproved        = string(model.StatusApproved)
	EventRejected        = string(model.StatusRejected)
	EventCancelled       = string(model.StatusCancelled)
	EventExpired         = string(model.StatusExpired)
)

// Reasons a review_requested event is sent.
const (
	ReviewReasonCreated             = model.ReviewReasonCreated
	ReviewReasonStageAdvanced       = model.ReviewReasonStageAdvanced
	ReviewReasonBypassLimitExceeded = model.ReviewReasonBypassLimitExceeded
)

// NewDuration wraps a time.Duration for a policy's auto_expire_duration.
var NewDuration = model.NewDuration

// Health is the server's health report.
type Health struct {
	Status     string                     `json:"status"`
	Components map[string]ComponentStatus `json:"components"`
}
