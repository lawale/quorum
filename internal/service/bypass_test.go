package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/auth"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/store"
	"github.com/lawale/quorum/internal/testutil"
)

func raw(s string) json.RawMessage { return json.RawMessage(s) }

func bypassPolicy(rules ...model.BypassRule) *model.Policy {
	return testutil.NewPolicy(func(p *model.Policy) { p.BypassRules = rules })
}

func TestMatchBypass_MakerConditions(t *testing.T) {
	req := testutil.NewRequest()
	policy := bypassPolicy(model.BypassRule{
		Name:             "root",
		MakerRoles:       []string{"root_admin", "cto"},
		MakerPermissions: []string{"bypass_review"},
		MakerAttributes:  map[string][]string{"branch_id": {"HQ"}},
	})

	if r := matchBypass(policy, req, []string{"cto"}, []string{"bypass_review"}, map[string]string{"branch_id": "HQ"}); r == nil || r.Name != "root" {
		t.Error("all conditions met should match")
	}
	if matchBypass(policy, req, []string{"manager"}, []string{"bypass_review"}, map[string]string{"branch_id": "HQ"}) != nil {
		t.Error("wrong role must not match")
	}
	if matchBypass(policy, req, []string{"cto"}, nil, map[string]string{"branch_id": "HQ"}) != nil {
		t.Error("missing permission must not match")
	}
	if matchBypass(policy, req, []string{"cto"}, []string{"bypass_review"}, map[string]string{"branch_id": "LAG-01"}) != nil {
		t.Error("wrong attribute must not match")
	}
	if matchBypass(policy, req, []string{"cto"}, []string{"bypass_review"}, nil) != nil {
		t.Error("missing attribute must not match")
	}
}

func TestMatchBypass_PayloadConditions(t *testing.T) {
	req := testutil.NewRequest(func(r *model.Request) {
		r.Payload = raw(`{"amount": 500, "currency": "NGN", "meta": {"internal": true}}`)
	})
	cases := []struct {
		name string
		cond model.BypassCondition
		want bool
	}{
		{"lt true", model.BypassCondition{Path: "amount", Op: "lt", Value: raw(`1000`)}, true},
		{"lt false", model.BypassCondition{Path: "amount", Op: "lt", Value: raw(`100`)}, false},
		{"lte equal", model.BypassCondition{Path: "amount", Op: "lte", Value: raw(`500`)}, true},
		{"gt false", model.BypassCondition{Path: "amount", Op: "gt", Value: raw(`500`)}, false},
		{"gte equal", model.BypassCondition{Path: "amount", Op: "gte", Value: raw(`500`)}, true},
		{"eq string", model.BypassCondition{Path: "currency", Op: "eq", Value: raw(`"NGN"`)}, true},
		{"eq type mismatch", model.BypassCondition{Path: "amount", Op: "eq", Value: raw(`"500"`)}, false},
		{"ne", model.BypassCondition{Path: "currency", Op: "ne", Value: raw(`"USD"`)}, true},
		{"ne on missing path is false", model.BypassCondition{Path: "nope", Op: "ne", Value: raw(`"USD"`)}, false},
		{"in", model.BypassCondition{Path: "currency", Op: "in", Value: raw(`["NGN","GHS"]`)}, true},
		{"in miss", model.BypassCondition{Path: "currency", Op: "in", Value: raw(`["USD"]`)}, false},
		{"nested bool", model.BypassCondition{Path: "meta.internal", Op: "eq", Value: raw(`true`)}, true},
		{"ordering on string is false", model.BypassCondition{Path: "currency", Op: "lt", Value: raw(`5`)}, false},
		{"object never equal", model.BypassCondition{Path: "meta", Op: "eq", Value: raw(`{"internal":true}`)}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			policy := bypassPolicy(model.BypassRule{Payload: []model.BypassCondition{tc.cond}})
			got := matchBypass(policy, req, nil, nil, nil) != nil
			if got != tc.want {
				t.Errorf("match = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMatchBypass_FirstMatchingRuleWins_AndAllConditionsAND(t *testing.T) {
	req := testutil.NewRequest(func(r *model.Request) { r.Payload = raw(`{"amount": 50}`) })
	policy := bypassPolicy(
		model.BypassRule{Name: "small-and-admin", MakerRoles: []string{"admin"}, Payload: []model.BypassCondition{{Path: "amount", Op: "lt", Value: raw(`100`)}}},
		model.BypassRule{Name: "small", Payload: []model.BypassCondition{{Path: "amount", Op: "lt", Value: raw(`100`)}}},
	)
	if r := matchBypass(policy, req, []string{"teller"}, nil, nil); r == nil || r.Name != "small" {
		t.Errorf("expected second rule to match, got %+v", r)
	}
	if r := matchBypass(policy, req, []string{"admin"}, nil, nil); r == nil || r.Name != "small-and-admin" {
		t.Errorf("expected first rule to match, got %+v", r)
	}
}

func TestMatchBypass_UnconstrainedRuleNeverMatches(t *testing.T) {
	policy := bypassPolicy(model.BypassRule{Name: "oops"})
	if matchBypass(policy, testutil.NewRequest(), nil, nil, nil) != nil {
		t.Error("a rule with no conditions must never bypass")
	}
}

func TestValidateBypassRules(t *testing.T) {
	cases := []struct {
		name    string
		rules   []model.BypassRule
		wantErr bool
	}{
		{"none", nil, false},
		{"role rule", []model.BypassRule{{MakerRoles: []string{"admin"}}}, false},
		{"empty rule", []model.BypassRule{{Name: "x"}}, true},
		{"only blank roles", []model.BypassRule{{MakerRoles: []string{" ", ""}}}, true},
		{"attr without values", []model.BypassRule{{MakerAttributes: map[string][]string{"branch_id": {" "}}}}, true},
		{"cond missing path", []model.BypassRule{{Payload: []model.BypassCondition{{Op: "eq", Value: raw(`1`)}}}}, true},
		{"cond bad op", []model.BypassRule{{Payload: []model.BypassCondition{{Path: "a", Op: "like", Value: raw(`1`)}}}}, true},
		{"cond missing value", []model.BypassRule{{Payload: []model.BypassCondition{{Path: "a", Op: "eq"}}}}, true},
		{"cond invalid json", []model.BypassRule{{Payload: []model.BypassCondition{{Path: "a", Op: "eq", Value: raw(`{bad`)}}}}, true},
		{"lt needs number", []model.BypassRule{{Payload: []model.BypassCondition{{Path: "a", Op: "lt", Value: raw(`"5"`)}}}}, true},
		{"in needs array", []model.BypassRule{{Payload: []model.BypassCondition{{Path: "a", Op: "in", Value: raw(`"x"`)}}}}, true},
		{"valid conds", []model.BypassRule{{Payload: []model.BypassCondition{{Path: "a", Op: "LTE", Value: raw(`5`)}, {Path: "b", Op: "in", Value: raw(`[1,2]`)}}}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateBypassRules(tc.rules)
			if (err != nil) != tc.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if err != nil && !errors.Is(err, ErrInvalidBypassRule) {
				t.Errorf("err should wrap ErrInvalidBypassRule: %v", err)
			}
		})
	}
}

func TestValidateBypassRules_NormalisesOp(t *testing.T) {
	rules := []model.BypassRule{{Payload: []model.BypassCondition{{Path: " a ", Op: " GTE ", Value: raw(`1`)}}}}
	if err := validateBypassRules(rules); err != nil {
		t.Fatal(err)
	}
	if rules[0].Payload[0].Op != "gte" || rules[0].Payload[0].Path != "a" {
		t.Errorf("not normalised: %+v", rules[0].Payload[0])
	}
}

type bypassCreateFixture struct {
	requests *testutil.MockRequestStore
	audits   *testutil.MockAuditStore
	policies *testutil.MockPolicyStore
	statuses []model.RequestStatus
	actions  []string
}

func newBypassCreateFixture(policy *model.Policy) *bypassCreateFixture {
	f := &bypassCreateFixture{}
	f.requests = &testutil.MockRequestStore{
		CreateFunc: func(ctx context.Context, r *model.Request) error {
			if r.ID == uuid.Nil {
				r.ID = uuid.New()
			}
			if r.Status == "" {
				r.Status = model.StatusPending // mirrors the real stores
			}
			return nil
		},
		UpdateStatusFunc: func(ctx context.Context, id uuid.UUID, status model.RequestStatus) error {
			f.statuses = append(f.statuses, status)
			return nil
		},
	}
	f.policies = &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) { return policy, nil },
	}
	f.audits = &testutil.MockAuditStore{
		CreateFunc: func(ctx context.Context, l *model.AuditLog) error {
			f.actions = append(f.actions, l.Action)
			return nil
		},
	}
	return f
}

func TestCreate_Bypass_AutoApprovesWithoutTx(t *testing.T) {
	policy := bypassPolicy(model.BypassRule{Name: "root-admin", MakerRoles: []string{"root_admin"}})
	f := newBypassCreateFixture(policy)
	svc := newTestRequestService(f.requests, &testutil.MockApprovalStore{}, f.policies, f.audits, nil)

	ctx := auth.WithIdentity(context.Background(), &auth.Identity{UserID: "alice", TenantID: "t1", Roles: []string{"root_admin"}})
	result, err := svc.Create(ctx, &model.Request{Type: "transfer", Payload: raw(`{"a":1}`), MakerID: "alice"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != model.StatusApproved {
		t.Errorf("status = %v, want approved", result.Status)
	}
	if len(f.statuses) != 1 || f.statuses[0] != model.StatusApproved {
		t.Errorf("UpdateStatus calls = %v", f.statuses)
	}
	if len(f.actions) != 2 || f.actions[0] != "created" || f.actions[1] != "bypassed" {
		t.Errorf("audit actions = %v, want [created bypassed]", f.actions)
	}
}

func TestCreate_Bypass_UsesTransactionAndEnqueuesWebhooks(t *testing.T) {
	policy := bypassPolicy(model.BypassRule{Name: "small", Payload: []model.BypassCondition{{Path: "amount", Op: "lt", Value: raw(`100`)}}})
	f := newBypassCreateFixture(policy)
	svc := newTestRequestService(f.requests, &testutil.MockApprovalStore{}, f.policies, f.audits, nil)

	txRuns, enqueued, signalled := 0, 0, 0
	svc.SetWebhookDispatch(
		func(ctx context.Context, fn func(tx *store.Stores) error) error {
			txRuns++
			return fn(&store.Stores{Requests: f.requests, Outbox: &testutil.MockOutboxStore{}, Webhooks: &testutil.MockWebhookStore{}})
		},
		func(ctx context.Context, outbox store.OutboxStore, webhooks store.WebhookStore, req *model.Request, approvals []model.Approval) error {
			enqueued++
			if req.Status != model.StatusApproved {
				t.Errorf("webhook payload status = %v, want approved", req.Status)
			}
			return nil
		},
		func() { signalled++ },
	)

	result, err := svc.Create(context.Background(), &model.Request{Type: "transfer", Payload: raw(`{"amount": 50}`), MakerID: "alice"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != model.StatusApproved {
		t.Errorf("status = %v, want approved", result.Status)
	}
	if txRuns != 1 || enqueued != 1 || signalled != 1 {
		t.Errorf("txRuns=%d enqueued=%d signalled=%d, want 1 each", txRuns, enqueued, signalled)
	}
}

func TestCreate_NoBypassMatch_StaysPending(t *testing.T) {
	policy := bypassPolicy(model.BypassRule{Name: "small", Payload: []model.BypassCondition{{Path: "amount", Op: "lt", Value: raw(`100`)}}})
	f := newBypassCreateFixture(policy)
	svc := newTestRequestService(f.requests, &testutil.MockApprovalStore{}, f.policies, f.audits, nil)

	result, err := svc.Create(context.Background(), &model.Request{Type: "transfer", Payload: raw(`{"amount": 5000}`), MakerID: "alice"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != model.StatusPending {
		t.Errorf("status = %v, want pending", result.Status)
	}
	if len(f.statuses) != 0 {
		t.Errorf("UpdateStatus should not be called, got %v", f.statuses)
	}
	if len(f.actions) != 1 || f.actions[0] != "created" {
		t.Errorf("audit actions = %v, want [created]", f.actions)
	}
}

func TestPolicyCreate_InvalidBypassRule_Error(t *testing.T) {
	policies := &testutil.MockPolicyStore{
		GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) { return nil, nil },
	}
	svc := NewPolicyService(policies)
	if err := svc.Create(context.Background(), bypassPolicy(model.BypassRule{Name: "empty"})); !errors.Is(err, ErrInvalidBypassRule) {
		t.Fatalf("expected ErrInvalidBypassRule, got %v", err)
	}
}
