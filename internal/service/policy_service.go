package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/wale/maker-checker/internal/model"
	"github.com/wale/maker-checker/internal/store"
)

var (
	ErrPolicyNotFound     = errors.New("policy not found")
	ErrPolicyTypeConflict = errors.New("a policy for this request type already exists")
)

type PolicyService struct {
	policies store.PolicyStore
}

func NewPolicyService(policies store.PolicyStore) *PolicyService {
	return &PolicyService{policies: policies}
}

func (s *PolicyService) Create(ctx context.Context, policy *model.Policy) error {
	existing, err := s.policies.GetByRequestType(ctx, policy.RequestType)
	if err != nil {
		return fmt.Errorf("checking existing policy: %w", err)
	}
	if existing != nil {
		return ErrPolicyTypeConflict
	}

	if policy.RequiredApprovals < 1 {
		policy.RequiredApprovals = 1
	}
	if policy.RejectionPolicy == "" {
		policy.RejectionPolicy = model.RejectionPolicyAny
	}

	return s.policies.Create(ctx, policy)
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

func (s *PolicyService) List(ctx context.Context) ([]model.Policy, error) {
	return s.policies.List(ctx)
}

func (s *PolicyService) Update(ctx context.Context, policy *model.Policy) error {
	existing, err := s.policies.GetByID(ctx, policy.ID)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrPolicyNotFound
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
