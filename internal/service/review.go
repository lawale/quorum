package service

import (
	"context"
	"encoding/json"

	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/store"
)

// OutboxEnqueuer writes outbox entries for an event on a request. Status
// changes pass the status as the event; review_requested also carries a
// ReviewRequest. It is injected from main so the service never imports the
// webhook package.
type OutboxEnqueuer func(ctx context.Context, outbox store.OutboxStore, webhooks store.WebhookStore, event string, req *model.Request, approvals []model.Approval, review *model.ReviewRequest) error

// buildReviewRequest describes the reviewers a stage is waiting on, resolving
// everything Quorum knows: the per-request allowlist, the stage's role and
// permission requirements, scope values computed for this request, and the
// user IDs that must not be notified because they cannot act (the maker and
// anyone who already voted on this stage).
func buildReviewRequest(policy *model.Policy, req *model.Request, stageIndex int, approvals []model.Approval, reason string) *model.ReviewRequest {
	stage := policy.StageAt(stageIndex)
	if stage == nil {
		return nil
	}

	rr := &model.ReviewRequest{
		Reason:            reason,
		StageIndex:        stageIndex,
		StageName:         stage.Name,
		TotalStages:       policy.TotalStages(),
		RequiredApprovals: stage.RequiredApprovals,
		EligibleReviewers: req.EligibleReviewers,
		AuthorizationMode: stage.AuthorizationMode,
		ExpiresAt:         req.ExpiresAt,
	}

	if len(stage.AllowedCheckerRoles) > 0 {
		_ = json.Unmarshal(stage.AllowedCheckerRoles, &rr.AllowedRoles)
	}
	if len(stage.AllowedPermissions) > 0 {
		_ = json.Unmarshal(stage.AllowedPermissions, &rr.AllowedPermissions)
	}

	if len(stage.Scope) > 0 {
		var payload map[string]any
		_ = json.Unmarshal(req.Payload, &payload)
		for _, rule := range stage.Scope {
			if rule.Attribute == "" {
				continue
			}
			values := scopeExpectedValues(rule, req, payload)
			if values == nil {
				values = []string{}
			}
			rr.Scope = append(rr.Scope, model.ResolvedScope{
				Attribute:             rule.Attribute,
				Values:                values,
				AllowUnscopedCheckers: rule.AllowUnscopedCheckers,
			})
		}
	}

	excluded := []string{req.MakerID}
	for _, a := range approvals {
		if a.StageIndex == stageIndex && a.CheckerID != req.MakerID {
			excluded = append(excluded, a.CheckerID)
		}
	}
	rr.ExcludedUserIDs = excluded

	return rr
}

// enqueueReviewRequested writes a review_requested event for the request's
// current stage using the given (possibly transactional) stores. It is a
// no-op when no event enqueuer is installed.
func (s *RequestService) enqueueReviewRequested(ctx context.Context, outbox store.OutboxStore, webhooks store.WebhookStore, policy *model.Policy, req *model.Request, approvals []model.Approval, reason string) error {
	if s.enqueue == nil {
		return nil
	}
	review := buildReviewRequest(policy, req, req.CurrentStage, approvals, reason)
	if review == nil {
		return nil
	}
	return s.enqueue(ctx, outbox, webhooks, model.EventReviewRequested, req, approvals, review)
}
