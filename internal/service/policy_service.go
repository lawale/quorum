package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/auth"
	"github.com/lawale/quorum/internal/display"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/store"
)

var (
	ErrPolicyNotFound           = errors.New("policy not found")
	ErrPolicyTypeConflict       = errors.New("a policy for this request type already exists")
	ErrNoStages                 = errors.New("policy must have at least one approval stage")
	ErrInvalidStageIndex        = errors.New("stage indices must be sequential starting from 0")
	ErrInvalidDisplayTemplate   = errors.New("invalid display template")
	ErrThresholdNoMaxCheckers   = errors.New("max_checkers is required when rejection_policy is 'threshold'")
	ErrInvalidAuthorizationMode = errors.New("authorization_mode must be 'any' or 'all' when both allowed_checker_roles and allowed_permissions are set")
)

type PolicyService struct {
	policies store.PolicyStore
}

func NewPolicyService(policies store.PolicyStore) *PolicyService {
	return &PolicyService{policies: policies}
}

func (s *PolicyService) Create(ctx context.Context, policy *model.Policy) error {
	if policy.TenantID == "" {
		policy.TenantID = auth.TenantIDFromContext(ctx)
	}

	existing, err := s.policies.GetByRequestType(ctx, policy.RequestType)
	if err != nil {
		return fmt.Errorf("checking existing policy: %w", err)
	}
	if existing != nil {
		return ErrPolicyTypeConflict
	}

	if err := validateAndDefaultStages(policy.Stages); err != nil {
		return err
	}

	if err := display.ValidateTemplate(policy.DisplayTemplate); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidDisplayTemplate, err.Error())
	}

	return s.policies.Create(ctx, policy)
}

// validateAndDefaultStages validates stage ordering and applies defaults
// for required_approvals and rejection_policy. It is called on both
// Create and Update to prevent policies from entering an invalid state.
func validateAndDefaultStages(stages []model.ApprovalStage) error {
	if len(stages) == 0 {
		return ErrNoStages
	}
	for i := range stages {
		if stages[i].Index != i {
			return fmt.Errorf("%w: expected %d, got %d", ErrInvalidStageIndex, i, stages[i].Index)
		}
		if stages[i].RequiredApprovals < 1 {
			stages[i].RequiredApprovals = 1
		}
		if stages[i].RejectionPolicy == "" {
			stages[i].RejectionPolicy = model.RejectionPolicyAny
		}
		if stages[i].RejectionPolicy == model.RejectionPolicyThreshold && stages[i].MaxCheckers == nil {
			return fmt.Errorf("%w (stage %d)", ErrThresholdNoMaxCheckers, i)
		}

		hasRoles := isNonEmptyJSONArray(stages[i].AllowedCheckerRoles)
		hasPermissions := isNonEmptyJSONArray(stages[i].AllowedPermissions)
		mode := stages[i].AuthorizationMode

		if hasRoles && hasPermissions {
			if mode != model.AuthModeAny && mode != model.AuthModeAll {
				return fmt.Errorf("%w (stage %d)", ErrInvalidAuthorizationMode, i)
			}
		} else if hasRoles && !hasPermissions && mode == model.AuthModePermission {
			return fmt.Errorf("%w (stage %d)", ErrInvalidAuthorizationMode, i)
		} else if hasPermissions && !hasRoles && mode == model.AuthModeRole {
			return fmt.Errorf("%w (stage %d)", ErrInvalidAuthorizationMode, i)
		}
	}
	return nil
}

// isNonEmptyJSONArray returns true if raw is a JSON array with at least one element.
func isNonEmptyJSONArray(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return false
	}
	return len(arr) > 0
}

func (s *PolicyService) GetByID(ctx context.Context, id uuid.UUID) (*model.Policy, error) {
	policy, err := s.policies.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if policy == nil {
		return nil, ErrPolicyNotFound
	}
	return policy, nil
}

func (s *PolicyService) GetByRequestType(ctx context.Context, requestType string) (*model.Policy, error) {
	policy, err := s.policies.GetByRequestType(ctx, requestType)
	if err != nil {
		return nil, err
	}
	if policy == nil {
		return nil, ErrPolicyNotFound
	}
	return policy, nil
}

func (s *PolicyService) List(ctx context.Context, filter store.PolicyFilter) ([]model.Policy, int, error) {
	return s.policies.List(ctx, filter)
}

func (s *PolicyService) Update(ctx context.Context, policy *model.Policy) error {
	existing, err := s.policies.GetByID(ctx, policy.ID)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrPolicyNotFound
	}

	if err := validateAndDefaultStages(policy.Stages); err != nil {
		return err
	}

	if err := display.ValidateTemplate(policy.DisplayTemplate); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidDisplayTemplate, err.Error())
	}

	return s.policies.Update(ctx, policy)
}

func (s *PolicyService) Delete(ctx context.Context, id uuid.UUID) error {
	existing, err := s.policies.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrPolicyNotFound
	}

	return s.policies.Delete(ctx, id)
}
