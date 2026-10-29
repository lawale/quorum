package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"

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
	ErrInvalidScopeRule         = errors.New("scope rules require a non-empty attribute")
	ErrInvalidBypassRule        = errors.New("invalid bypass rule")
)

type PolicyService struct {
	policies store.PolicyStore

	authMode             string
	knownAttributes      map[string]bool
	attributesConfigured bool
}

func NewPolicyService(policies store.PolicyStore) *PolicyService {
	return &PolicyService{policies: policies}
}

// SetKnownAttributes tells the service which identity attribute names the
// configured auth mode can supply, so policies whose scope rules reference an
// attribute no checker will ever carry can be flagged when they are saved.
func (s *PolicyService) SetKnownAttributes(authMode string, names []string) {
	s.authMode = authMode
	s.knownAttributes = make(map[string]bool, len(names))
	for _, n := range names {
		s.knownAttributes[n] = true
	}
	s.attributesConfigured = true
}

// scopeWarnings returns advisory messages for scope rules that reference
// identity attributes the auth mode does not provide. Such rules are valid but
// unsatisfiable: the stage would never accept a checker.
func (s *PolicyService) scopeWarnings(policy *model.Policy) []string {
	if !s.attributesConfigured {
		return nil
	}

	seen := make(map[string]bool)
	var used []string
	for _, stage := range policy.Stages {
		for _, rule := range stage.Scope {
			if rule.Attribute != "" && !seen[rule.Attribute] {
				seen[rule.Attribute] = true
				used = append(used, rule.Attribute)
			}
		}
	}
	if len(used) == 0 {
		return nil
	}

	section := "auth." + s.authMode + ".attribute_headers"
	if s.authMode == "verify" {
		section = "auth.verify.claims.attributes"
	}

	if len(s.knownAttributes) == 0 {
		return []string{fmt.Sprintf(
			"scope rules reference identity attribute(s) %s but %s is empty; no checker will carry them and these stages can never be approved",
			strings.Join(used, ", "), section)}
	}

	known := make([]string, 0, len(s.knownAttributes))
	for n := range s.knownAttributes {
		known = append(known, n)
	}
	sort.Strings(known)

	var warnings []string
	for _, attr := range used {
		if !s.knownAttributes[attr] {
			warnings = append(warnings, fmt.Sprintf(
				"scope attribute %q is not configured in %s (configured: %s); no checker will carry it and that stage can never be approved",
				attr, section, strings.Join(known, ", ")))
		}
	}
	return warnings
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

	if err := validateBypassRules(policy.BypassRules); err != nil {
		return err
	}

	if err := s.policies.Create(ctx, policy); err != nil {
		return err
	}
	s.attachWarnings(policy)
	return nil
}

func (s *PolicyService) attachWarnings(policy *model.Policy) {
	policy.Warnings = s.scopeWarnings(policy)
	for _, w := range policy.Warnings {
		slog.Warn("policy saved with unsatisfiable scope rule", "policy_id", policy.ID, "request_type", policy.RequestType, "warning", w)
	}
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

		switch {
		case hasRoles && hasPermissions:
			if mode != model.AuthModeAny && mode != model.AuthModeAll {
				return fmt.Errorf("%w (stage %d)", ErrInvalidAuthorizationMode, i)
			}
		case hasRoles && !hasPermissions && mode == model.AuthModePermission:
			return fmt.Errorf("%w (stage %d)", ErrInvalidAuthorizationMode, i)
		case hasPermissions && !hasRoles && mode == model.AuthModeRole:
			return fmt.Errorf("%w (stage %d)", ErrInvalidAuthorizationMode, i)
		}

		for j := range stages[i].Scope {
			rule := &stages[i].Scope[j]
			rule.Attribute = strings.TrimSpace(rule.Attribute)
			rule.PayloadPath = strings.TrimSpace(rule.PayloadPath)
			if rule.Attribute == "" {
				return fmt.Errorf("%w (stage %d, rule %d)", ErrInvalidScopeRule, i, j)
			}
			values := rule.Values[:0]
			for _, v := range rule.Values {
				if v = strings.TrimSpace(v); v != "" {
					values = append(values, v)
				}
			}
			rule.Values = values
			if len(rule.Values) == 0 {
				rule.Values = nil
			}
			// A rule that names no source matches the maker; make that explicit
			// in the stored policy so readers do not have to know the default.
			if !rule.HasSource() {
				rule.MatchMaker = true
			}
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

	if err := validateBypassRules(policy.BypassRules); err != nil {
		return err
	}

	if err := s.policies.Update(ctx, policy); err != nil {
		return err
	}
	s.attachWarnings(policy)
	return nil
}

func (s *PolicyService) DistinctRequestTypes(ctx context.Context) ([]string, error) {
	return s.policies.DistinctRequestTypes(ctx)
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
