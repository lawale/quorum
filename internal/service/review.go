package service

import (
	"encoding/json"

	"github.com/lawale/quorum/internal/model"
)

// buildReviewRequest describes the reviewers a stage is waiting on, resolving
// everything Quorum knows: the per-request allowlist, the stage's role and
// permission requirements, scope values computed for this request, and the
// user IDs that must not be notified because they cannot act. A stage is
// announced only when it starts, so nobody has voted on it yet and the only
// exclusion is the maker. EligibleReviewers and Scope always serialise as
// lists, never null, so consumers can iterate them without a nil check.
func buildReviewRequest(policy *model.Policy, req *model.Request, stageIndex int, reason string) *model.ReviewRequest {
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
		Scope:             []model.ResolvedScope{},
		ExcludedUserIDs:   []string{req.MakerID},
		ExpiresAt:         req.ExpiresAt,
	}
	if rr.EligibleReviewers == nil {
		rr.EligibleReviewers = []string{}
	}

	rr.AllowedRoles = stage.AllowedCheckerRoles
	rr.AllowedPermissions = stage.AllowedPermissions

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

	return rr
}

// emitReviewRequested announces that the request's current stage is waiting
// on reviewers. A stage the policy no longer defines is not announced.
func emitReviewRequested(emit emitFunc, policy *model.Policy, req *model.Request, reason string) error {
	review := buildReviewRequest(policy, req, req.CurrentStage, reason)
	if review == nil {
		return nil
	}
	return emit(model.ReviewRequested(req, review))
}
