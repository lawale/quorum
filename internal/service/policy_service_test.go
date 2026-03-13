package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/testutil"
)

func TestPolicyCreate_Success(t *testing.T) {
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) {
			return nil, nil // No conflict
		},
		CreateFunc: func(ctx context.Context, policy *model.Policy) error {
			return nil
		},
	}
	svc := NewPolicyService(policies)

	policy := testutil.NewPolicy()
	err := svc.Create(context.Background(), policy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPolicyCreate_TypeConflict(t *testing.T) {
	existing := testutil.NewPolicy()
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) {
			return existing, nil
		},
	}
	svc := NewPolicyService(policies)

	policy := testutil.NewPolicy()
	err := svc.Create(context.Background(), policy)
	if !errors.Is(err, ErrPolicyTypeConflict) {
		t.Fatalf("expected ErrPolicyTypeConflict, got: %v", err)
	}
}

func TestPolicyCreate_DefaultsStageRequiredApprovals(t *testing.T) {
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) {
			return nil, nil
		},
		CreateFunc: func(ctx context.Context, policy *model.Policy) error {
			return nil
		},
	}
	svc := NewPolicyService(policies)

	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.Stages[0].RequiredApprovals = 0
	})
	err := svc.Create(context.Background(), policy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if policy.Stages[0].RequiredApprovals != 1 {
		t.Errorf("Stages[0].RequiredApprovals = %d, want 1 (default)", policy.Stages[0].RequiredApprovals)
	}
}

func TestPolicyCreate_DefaultsStageRejectionPolicy(t *testing.T) {
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) {
			return nil, nil
		},
		CreateFunc: func(ctx context.Context, policy *model.Policy) error {
			return nil
		},
	}
	svc := NewPolicyService(policies)

	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.Stages[0].RejectionPolicy = ""
	})
	err := svc.Create(context.Background(), policy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if policy.Stages[0].RejectionPolicy != model.RejectionPolicyAny {
		t.Errorf("Stages[0].RejectionPolicy = %q, want %q", policy.Stages[0].RejectionPolicy, model.RejectionPolicyAny)
	}
}

func TestPolicyCreate_NoStages_Error(t *testing.T) {
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) {
			return nil, nil
		},
	}
	svc := NewPolicyService(policies)

	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.Stages = nil
	})
	err := svc.Create(context.Background(), policy)
	if !errors.Is(err, ErrNoStages) {
		t.Fatalf("expected ErrNoStages, got: %v", err)
	}
}

func TestPolicyCreate_InvalidStageIndex_Error(t *testing.T) {
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) {
			return nil, nil
		},
	}
	svc := NewPolicyService(policies)

	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.Stages = []model.ApprovalStage{
			{Index: 0, RequiredApprovals: 1, RejectionPolicy: model.RejectionPolicyAny},
			{Index: 5, RequiredApprovals: 1, RejectionPolicy: model.RejectionPolicyAny}, // Wrong index
		}
	})
	err := svc.Create(context.Background(), policy)
	if !errors.Is(err, ErrInvalidStageIndex) {
		t.Fatalf("expected ErrInvalidStageIndex, got: %v", err)
	}
}

func TestPolicyCreate_MultiStage_Success(t *testing.T) {
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) {
			return nil, nil
		},
		CreateFunc: func(ctx context.Context, policy *model.Policy) error {
			return nil
		},
	}
	svc := NewPolicyService(policies)

	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.Stages = []model.ApprovalStage{
			{Index: 0, Name: "Finance Review", RequiredApprovals: 1, RejectionPolicy: model.RejectionPolicyAny},
			{Index: 1, Name: "Manager Approval", RequiredApprovals: 2, RejectionPolicy: model.RejectionPolicyThreshold, MaxCheckers: testutil.IntPtr(3)},
		}
	})
	err := svc.Create(context.Background(), policy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPolicyGetByID_Success(t *testing.T) {
	expected := testutil.NewPolicy()
	policies := &testutil.MockPolicyStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Policy, error) {
			return expected, nil
		},
	}
	svc := NewPolicyService(policies)

	result, err := svc.GetByID(context.Background(), expected.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != expected.ID {
		t.Errorf("ID = %v, want %v", result.ID, expected.ID)
	}
}

func TestPolicyGetByID_NotFound(t *testing.T) {
	policies := &testutil.MockPolicyStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Policy, error) {
			return nil, nil
		},
	}
	svc := NewPolicyService(policies)

	_, err := svc.GetByID(context.Background(), uuid.New())
	if !errors.Is(err, ErrPolicyNotFound) {
		t.Fatalf("expected ErrPolicyNotFound, got: %v", err)
	}
}

func TestPolicyGetByRequestType_Success(t *testing.T) {
	expected := testutil.NewPolicy()
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) {
			return expected, nil
		},
	}
	svc := NewPolicyService(policies)

	result, err := svc.GetByRequestType(context.Background(), "transfer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != expected.ID {
		t.Errorf("ID = %v, want %v", result.ID, expected.ID)
	}
}

func TestPolicyGetByRequestType_NotFound(t *testing.T) {
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) {
			return nil, nil
		},
	}
	svc := NewPolicyService(policies)

	_, err := svc.GetByRequestType(context.Background(), "nonexistent")
	if !errors.Is(err, ErrPolicyNotFound) {
		t.Fatalf("expected ErrPolicyNotFound, got: %v", err)
	}
}

func TestPolicyList_Success(t *testing.T) {
	expected := []model.Policy{*testutil.NewPolicy(), *testutil.NewPolicy()}
	policies := &testutil.MockPolicyStore{
		ListFunc: func(ctx context.Context) ([]model.Policy, error) {
			return expected, nil
		},
	}
	svc := NewPolicyService(policies)

	result, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 policies, got %d", len(result))
	}
}

func TestPolicyUpdate_Success(t *testing.T) {
	existing := testutil.NewPolicy()
	policies := &testutil.MockPolicyStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Policy, error) {
			return existing, nil
		},
		UpdateFunc: func(ctx context.Context, policy *model.Policy) error {
			return nil
		},
	}
	svc := NewPolicyService(policies)

	err := svc.Update(context.Background(), existing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPolicyUpdate_NotFound(t *testing.T) {
	policies := &testutil.MockPolicyStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Policy, error) {
			return nil, nil
		},
	}
	svc := NewPolicyService(policies)

	err := svc.Update(context.Background(), testutil.NewPolicy())
	if !errors.Is(err, ErrPolicyNotFound) {
		t.Fatalf("expected ErrPolicyNotFound, got: %v", err)
	}
}

func TestPolicyDelete_Success(t *testing.T) {
	existing := testutil.NewPolicy()
	policies := &testutil.MockPolicyStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Policy, error) {
			return existing, nil
		},
		DeleteFunc: func(ctx context.Context, id uuid.UUID) error {
			return nil
		},
	}
	svc := NewPolicyService(policies)

	err := svc.Delete(context.Background(), existing.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPolicyDelete_NotFound(t *testing.T) {
	policies := &testutil.MockPolicyStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Policy, error) {
			return nil, nil
		},
	}
	svc := NewPolicyService(policies)

	err := svc.Delete(context.Background(), uuid.New())
	if !errors.Is(err, ErrPolicyNotFound) {
		t.Fatalf("expected ErrPolicyNotFound, got: %v", err)
	}
}
