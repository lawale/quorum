package testutil

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/auth"
	"github.com/lawale/quorum/internal/model"
)

// NewRequest creates a model.Request with sensible defaults and optional overrides.
func NewRequest(overrides ...func(*model.Request)) *model.Request {
	now := time.Now().UTC()
	req := &model.Request{
		ID:        uuid.New(),
		Type:      "transfer",
		Payload:   json.RawMessage(`{"amount":100,"account_id":"acc-123"}`),
		Status:    model.StatusPending,
		MakerID:   "user-maker",
		CreatedAt: now,
		UpdatedAt: now,
	}
	for _, o := range overrides {
		o(req)
	}
	return req
}

// NewPolicy creates a model.Policy with sensible defaults and optional overrides.
func NewPolicy(overrides ...func(*model.Policy)) *model.Policy {
	now := time.Now().UTC()
	policy := &model.Policy{
		ID:          uuid.New(),
		Name:        "Transfer Policy",
		RequestType: "transfer",
		Stages: []model.ApprovalStage{
			{
				Index:             0,
				RequiredApprovals: 1,
				RejectionPolicy:   model.RejectionPolicyAny,
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	for _, o := range overrides {
		o(policy)
	}
	return policy
}

// NewApproval creates a model.Approval with sensible defaults and optional overrides.
func NewApproval(overrides ...func(*model.Approval)) *model.Approval {
	approval := &model.Approval{
		ID:        uuid.New(),
		RequestID: uuid.New(),
		CheckerID: "user-checker",
		Decision:  model.DecisionApproved,
		CreatedAt: time.Now().UTC(),
	}
	for _, o := range overrides {
		o(approval)
	}
	return approval
}

// NewWebhook creates a model.Webhook with sensible defaults and optional overrides.
func NewWebhook(overrides ...func(*model.Webhook)) *model.Webhook {
	webhook := &model.Webhook{
		ID:        uuid.New(),
		URL:       "https://example.com/webhook",
		Events:    []string{"approved", "rejected"},
		Secret:    "test-secret",
		Active:    true,
		CreatedAt: time.Now().UTC(),
	}
	for _, o := range overrides {
		o(webhook)
	}
	return webhook
}

// NewAuditLog creates a model.AuditLog with sensible defaults and optional overrides.
func NewAuditLog(overrides ...func(*model.AuditLog)) *model.AuditLog {
	log := &model.AuditLog{
		ID:        uuid.New(),
		RequestID: uuid.New(),
		Action:    "created",
		ActorID:   "user-maker",
		CreatedAt: time.Now().UTC(),
	}
	for _, o := range overrides {
		o(log)
	}
	return log
}

// ContextWithIdentity creates a context with an auth.Identity set.
func ContextWithIdentity(userID string, roles []string) context.Context {
	return auth.WithIdentity(context.Background(), &auth.Identity{
		UserID: userID,
		Roles:  roles,
	})
}

// StringPtr returns a pointer to the given string.
func StringPtr(s string) *string {
	return &s
}

// IntPtr returns a pointer to the given int.
func IntPtr(i int) *int {
	return &i
}

// DurationPtr returns a pointer to the given time.Duration.
func DurationPtr(d time.Duration) *time.Duration {
	return &d
}
