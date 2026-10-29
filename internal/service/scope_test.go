package service

import (
	"context"
	"errors"
	"testing"

	"github.com/lawale/quorum/internal/auth"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/testutil"
)

func ctxWithAttributes(attrs map[string]string) context.Context {
	return auth.WithIdentity(context.Background(), &auth.Identity{
		UserID:     "checker-1",
		TenantID:   "t1",
		Attributes: attrs,
	})
}

func scopedPolicy(rules ...model.ScopeRule) *model.Policy {
	return testutil.NewPolicy(func(p *model.Policy) {
		p.Stages[0].Scope = rules
	})
}

func TestValidateScope_NoRules_Passes(t *testing.T) {
	stage := &model.ApprovalStage{}
	if err := validateScope(stage, testutil.NewRequest(), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateScope_MakerMatch(t *testing.T) {
	stage := &model.ApprovalStage{Scope: []model.ScopeRule{{Attribute: "branch_id"}}}
	req := testutil.NewRequest(func(r *model.Request) {
		r.MakerAttributes = map[string]string{"branch_id": "LAG-01"}
	})

	if err := validateScope(stage, req, map[string]string{"branch_id": "LAG-01"}); err != nil {
		t.Errorf("same branch should pass: %v", err)
	}
	if err := validateScope(stage, req, map[string]string{"branch_id": "ABJ-02"}); !errors.Is(err, ErrOutOfScope) {
		t.Errorf("different branch: err = %v, want ErrOutOfScope", err)
	}
}

func TestValidateScope_CheckerMissingAttribute(t *testing.T) {
	req := testutil.NewRequest(func(r *model.Request) {
		r.MakerAttributes = map[string]string{"branch_id": "LAG-01"}
	})

	strict := &model.ApprovalStage{Scope: []model.ScopeRule{{Attribute: "branch_id"}}}
	if err := validateScope(strict, req, nil); !errors.Is(err, ErrOutOfScope) {
		t.Errorf("strict rule with no checker attribute: err = %v, want ErrOutOfScope", err)
	}

	lenient := &model.ApprovalStage{Scope: []model.ScopeRule{{Attribute: "branch_id", AllowUnscopedCheckers: true}}}
	if err := validateScope(lenient, req, nil); err != nil {
		t.Errorf("allow_unscoped_checkers should pass a checker without the attribute: %v", err)
	}
	// A checker WITH the attribute is still held to it under the lenient rule.
	if err := validateScope(lenient, req, map[string]string{"branch_id": "ABJ-02"}); !errors.Is(err, ErrOutOfScope) {
		t.Errorf("lenient rule, wrong branch: err = %v, want ErrOutOfScope", err)
	}
}

func TestValidateScope_MakerMissingAttribute_FailsClosed(t *testing.T) {
	stage := &model.ApprovalStage{Scope: []model.ScopeRule{{Attribute: "branch_id"}}}
	req := testutil.NewRequest() // no maker attributes

	if err := validateScope(stage, req, map[string]string{"branch_id": "LAG-01"}); !errors.Is(err, ErrOutOfScope) {
		t.Errorf("err = %v, want ErrOutOfScope", err)
	}
}

func TestValidateScope_PayloadPath(t *testing.T) {
	stage := &model.ApprovalStage{Scope: []model.ScopeRule{{Attribute: "branch_id", PayloadPath: "current.branch_id"}}}
	req := testutil.NewRequest(func(r *model.Request) {
		r.Payload = []byte(`{"current": {"branch_id": "LAG-01", "code": 7}, "proposed": {"branch_id": "LAG-01"}}`)
		r.MakerAttributes = map[string]string{"branch_id": "HQ"} // ignored when payload_path is set
	})

	if err := validateScope(stage, req, map[string]string{"branch_id": "LAG-01"}); err != nil {
		t.Errorf("matching payload value should pass: %v", err)
	}
	if err := validateScope(stage, req, map[string]string{"branch_id": "HQ"}); !errors.Is(err, ErrOutOfScope) {
		t.Errorf("maker attribute must not be used when payload_path is set: err = %v", err)
	}

	numeric := &model.ApprovalStage{Scope: []model.ScopeRule{{Attribute: "code", PayloadPath: "current.code"}}}
	if err := validateScope(numeric, req, map[string]string{"code": "7"}); err != nil {
		t.Errorf("numeric payload value should compare as string: %v", err)
	}

	missing := &model.ApprovalStage{Scope: []model.ScopeRule{{Attribute: "branch_id", PayloadPath: "nope.branch_id"}}}
	if err := validateScope(missing, req, map[string]string{"branch_id": "LAG-01"}); !errors.Is(err, ErrOutOfScope) {
		t.Errorf("missing payload path must fail closed: err = %v", err)
	}

	object := &model.ApprovalStage{Scope: []model.ScopeRule{{Attribute: "branch_id", PayloadPath: "current"}}}
	if err := validateScope(object, req, map[string]string{"branch_id": "LAG-01"}); !errors.Is(err, ErrOutOfScope) {
		t.Errorf("object at payload path is not a scalar: err = %v", err)
	}
}

func TestValidateScope_AllRulesMustPass(t *testing.T) {
	stage := &model.ApprovalStage{Scope: []model.ScopeRule{{Attribute: "branch_id"}, {Attribute: "department"}}}
	req := testutil.NewRequest(func(r *model.Request) {
		r.MakerAttributes = map[string]string{"branch_id": "LAG-01", "department": "ops"}
	})

	if err := validateScope(stage, req, map[string]string{"branch_id": "LAG-01", "department": "ops"}); err != nil {
		t.Errorf("both match: %v", err)
	}
	if err := validateScope(stage, req, map[string]string{"branch_id": "LAG-01", "department": "risk"}); !errors.Is(err, ErrOutOfScope) {
		t.Errorf("second rule fails: err = %v", err)
	}
}

func TestApprove_OutOfScope_Rejected(t *testing.T) {
	req := testutil.NewRequest(func(r *model.Request) {
		r.MakerAttributes = map[string]string{"branch_id": "LAG-01"}
	})
	policy := scopedPolicy(model.ScopeRule{Attribute: "branch_id"})
	requests, approvals, policies, audits := setupApproveTest(req, policy)
	svc := newTestRequestService(requests, approvals, policies, audits, nil)

	_, err := svc.Approve(ctxWithAttributes(map[string]string{"branch_id": "ABJ-02"}), req.ID, "checker-1", nil, nil)
	if !errors.Is(err, ErrOutOfScope) {
		t.Fatalf("err = %v, want ErrOutOfScope", err)
	}
}

func TestApprove_InScope_Succeeds(t *testing.T) {
	req := testutil.NewRequest(func(r *model.Request) {
		r.MakerAttributes = map[string]string{"branch_id": "LAG-01"}
	})
	policy := scopedPolicy(model.ScopeRule{Attribute: "branch_id"})
	requests, approvals, policies, audits := setupApproveTest(req, policy)
	svc := newTestRequestService(requests, approvals, policies, audits, nil)

	result, err := svc.Approve(ctxWithAttributes(map[string]string{"branch_id": "LAG-01"}), req.ID, "checker-1", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != model.StatusApproved {
		t.Errorf("status = %v, want approved", result.Status)
	}
}

func TestCanViewerAct_RespectsScope(t *testing.T) {
	req := testutil.NewRequest(func(r *model.Request) {
		r.MakerAttributes = map[string]string{"branch_id": "LAG-01"}
	})
	policy := scopedPolicy(model.ScopeRule{Attribute: "branch_id"})
	policy.RequestType = req.Type
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) { return policy, nil },
	}
	svc := newTestRequestService(&testutil.MockRequestStore{}, &testutil.MockApprovalStore{}, policies, &testutil.MockAuditStore{}, nil)

	if !svc.CanViewerAct(ctxWithAttributes(map[string]string{"branch_id": "LAG-01"}), req, "checker-1", nil, nil) {
		t.Error("same branch should be able to act")
	}
	if svc.CanViewerAct(ctxWithAttributes(map[string]string{"branch_id": "ABJ-02"}), req, "checker-1", nil, nil) {
		t.Error("different branch should not be able to act")
	}
}

func TestCreate_CapturesMakerAttributes(t *testing.T) {
	var stored *model.Request
	requests := &testutil.MockRequestStore{
		CreateFunc: func(ctx context.Context, r *model.Request) error { stored = r; return nil },
	}
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) { return testutil.NewPolicy(), nil },
	}
	audits := &testutil.MockAuditStore{CreateFunc: func(ctx context.Context, l *model.AuditLog) error { return nil }}
	svc := newTestRequestService(requests, &testutil.MockApprovalStore{}, policies, audits, nil)

	ctx := auth.WithIdentity(context.Background(), &auth.Identity{
		UserID: "maker", TenantID: "t1", Attributes: map[string]string{"branch_id": "LAG-01"},
	})
	if _, err := svc.Create(ctx, &model.Request{Type: "transfer", Payload: []byte(`{"a":1}`), MakerID: "maker"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stored == nil || stored.MakerAttributes["branch_id"] != "LAG-01" {
		t.Errorf("maker attributes not captured: %+v", stored)
	}
}

func TestPolicyCreate_ScopeRule_MissingAttribute_Error(t *testing.T) {
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) { return nil, nil },
	}
	svc := NewPolicyService(policies)

	policy := scopedPolicy(model.ScopeRule{Attribute: "  "})
	if err := svc.Create(context.Background(), policy); !errors.Is(err, ErrInvalidScopeRule) {
		t.Fatalf("expected ErrInvalidScopeRule, got: %v", err)
	}
}
