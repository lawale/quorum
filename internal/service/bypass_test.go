package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/auth"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/store"
	"github.com/lawale/quorum/internal/testutil"
)

func raw(s string) json.RawMessage { return json.RawMessage(s) }

// matchBypassConds is the condition-only view used by the matching tests.
func matchBypassConds(policy *model.Policy, req *model.Request, roles, perms []string, attrs map[string]string) *model.BypassRule {
	rules := matchingBypassRules(policy, req, roles, perms, attrs)
	if len(rules) == 0 {
		return nil
	}
	return rules[0]
}

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

	if r := matchBypassConds(policy, req, []string{"cto"}, []string{"bypass_review"}, map[string]string{"branch_id": "HQ"}); r == nil || r.Name != "root" {
		t.Error("all conditions met should match")
	}
	if matchBypassConds(policy, req, []string{"manager"}, []string{"bypass_review"}, map[string]string{"branch_id": "HQ"}) != nil {
		t.Error("wrong role must not match")
	}
	if matchBypassConds(policy, req, []string{"cto"}, nil, map[string]string{"branch_id": "HQ"}) != nil {
		t.Error("missing permission must not match")
	}
	if matchBypassConds(policy, req, []string{"cto"}, []string{"bypass_review"}, map[string]string{"branch_id": "LAG-01"}) != nil {
		t.Error("wrong attribute must not match")
	}
	if matchBypassConds(policy, req, []string{"cto"}, []string{"bypass_review"}, nil) != nil {
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
			got := matchBypassConds(policy, req, nil, nil, nil) != nil
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
	if r := matchBypassConds(policy, req, []string{"teller"}, nil, nil); r == nil || r.Name != "small" {
		t.Errorf("expected second rule to match, got %+v", r)
	}
	if r := matchBypassConds(policy, req, []string{"admin"}, nil, nil); r == nil || r.Name != "small-and-admin" {
		t.Errorf("expected first rule to match, got %+v", r)
	}
}

func TestMatchBypass_UnconstrainedRuleNeverMatches(t *testing.T) {
	policy := bypassPolicy(model.BypassRule{Name: "oops"})
	if matchBypassConds(policy, testutil.NewRequest(), nil, nil, nil) != nil {
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

// --- Limits ---

func recentStore(recent []model.Request, err error) *testutil.MockRequestStore {
	return &testutil.MockRequestStore{
		ListRecentByTypeFunc: func(ctx context.Context, reqType string, since time.Time, makerID string) ([]model.Request, error) {
			return recent, err
		},
	}
}

func limitedRule(limits ...model.BypassLimit) *model.BypassRule {
	return &model.BypassRule{
		Name:    "small",
		Payload: []model.BypassCondition{{Path: "amount", Op: "lt", Value: raw(`1000`)}},
		Limits:  limits,
	}
}

func priorRequest(payload string) model.Request {
	return model.Request{ID: uuid.New(), Type: "transfer", MakerID: "alice", Payload: raw(payload)}
}

func TestLimitsExceeded_Count(t *testing.T) {
	three := 3
	rule := limitedRule(model.BypassLimit{Window: "24h", MaxCount: &three})
	req := &model.Request{ID: uuid.New(), Type: "transfer", MakerID: "alice", Payload: raw(`{"amount": 900}`)}

	hit, err := limitsExceeded(context.Background(), recentStore([]model.Request{priorRequest(`{"amount":1}`), priorRequest(`{"amount":1}`)}, nil), rule, req)
	if err != nil || hit != nil {
		t.Errorf("2 prior + 1 new = 3 should be within max_count 3: hit=%+v err=%v", hit, err)
	}

	hit, err = limitsExceeded(context.Background(), recentStore([]model.Request{priorRequest(`{}`), priorRequest(`{}`), priorRequest(`{}`)}, nil), rule, req)
	if err != nil || hit == nil || hit.Measure != "count" || hit.Observed != 4 || hit.Limit != 3 {
		t.Errorf("3 prior + 1 new should exceed: hit=%+v err=%v", hit, err)
	}
}

func TestLimitsExceeded_Sum(t *testing.T) {
	rule := limitedRule(model.BypassLimit{Window: "24h", MaxSum: &model.BypassSum{Path: "amount", Value: 2500}})
	req := &model.Request{ID: uuid.New(), Type: "transfer", MakerID: "alice", Payload: raw(`{"amount": 900}`)}

	hit, err := limitsExceeded(context.Background(), recentStore([]model.Request{priorRequest(`{"amount":900}`), priorRequest(`{"amount":"700"}`)}, nil), rule, req)
	if err != nil || hit != nil {
		t.Errorf("900+700+900=2500 is within 2500: hit=%+v err=%v", hit, err)
	}

	hit, err = limitsExceeded(context.Background(), recentStore([]model.Request{priorRequest(`{"amount":900}`), priorRequest(`{"amount":900}`)}, nil), rule, req)
	if err != nil || hit == nil || hit.Measure != "sum" || hit.Path != "amount" || hit.Observed != 2700 {
		t.Errorf("2700 exceeds 2500: hit=%+v err=%v", hit, err)
	}
}

func TestLimitsExceeded_GroupByPayload(t *testing.T) {
	two := 2
	rule := limitedRule(model.BypassLimit{Window: "7d", MaxCount: &two, GroupBy: []string{"payload:destination_account"}})
	req := &model.Request{ID: uuid.New(), Type: "transfer", MakerID: "alice", Payload: raw(`{"amount": 900, "destination_account": "ACC-9"}`)}

	var askedMaker *string
	requests := &testutil.MockRequestStore{
		ListRecentByTypeFunc: func(ctx context.Context, reqType string, since time.Time, makerID string) ([]model.Request, error) {
			askedMaker = &makerID
			return []model.Request{
				{ID: uuid.New(), MakerID: "bob", Payload: raw(`{"destination_account": "ACC-9"}`)},
				{ID: uuid.New(), MakerID: "carol", Payload: raw(`{"destination_account": "ACC-9"}`)},
				{ID: uuid.New(), MakerID: "dave", Payload: raw(`{"destination_account": "ACC-1"}`)},
			}, nil
		},
	}

	hit, err := limitsExceeded(context.Background(), requests, rule, req)
	if err != nil {
		t.Fatal(err)
	}
	if askedMaker == nil || *askedMaker != "" {
		t.Errorf("group_by without maker must query across makers, got maker filter %v", askedMaker)
	}
	if hit == nil || hit.Observed != 3 {
		t.Errorf("two priors to ACC-9 plus new = 3 > 2: hit=%+v", hit)
	}
}

func TestLimitsExceeded_ExcludesSelfAndUsesWindow(t *testing.T) {
	one := 1
	rule := limitedRule(model.BypassLimit{Window: "2d", MaxCount: &one})
	req := &model.Request{ID: uuid.New(), Type: "transfer", MakerID: "alice", Payload: raw(`{"amount": 1}`)}

	var since time.Time
	requests := &testutil.MockRequestStore{
		ListRecentByTypeFunc: func(ctx context.Context, reqType string, s time.Time, makerID string) ([]model.Request, error) {
			since = s
			return []model.Request{*req}, nil // only itself
		},
	}
	hit, err := limitsExceeded(context.Background(), requests, rule, req)
	if err != nil || hit != nil {
		t.Errorf("self must not count: hit=%+v err=%v", hit, err)
	}
	if d := time.Since(since); d < 47*time.Hour || d > 49*time.Hour {
		t.Errorf("window 2d should query ~48h back, got %v", d)
	}
}

func TestMatchBypass_LimitFailsSafeOnStoreError(t *testing.T) {
	one := 1
	policy := bypassPolicy(*limitedRule(model.BypassLimit{Window: "24h", MaxCount: &one}))
	req := &model.Request{ID: uuid.New(), Type: "transfer", MakerID: "alice", Payload: raw(`{"amount": 1}`)}

	rule, hit := matchBypass(context.Background(), recentStore(nil, errors.New("db down")), policy, req, nil, nil, nil)
	if rule != nil || hit != nil {
		t.Errorf("store error must route to review without a hit: rule=%+v hit=%+v", rule, hit)
	}
}

func TestMatchBypass_FallsThroughToNextRule(t *testing.T) {
	one := 1
	first := *limitedRule(model.BypassLimit{Window: "24h", MaxCount: &one})
	first.Name = "limited"
	second := model.BypassRule{Name: "admin", MakerRoles: []string{"admin"}}
	policy := bypassPolicy(first, second)
	req := &model.Request{ID: uuid.New(), Type: "transfer", MakerID: "alice", Payload: raw(`{"amount": 1}`)}
	requests := recentStore([]model.Request{priorRequest(`{}`), priorRequest(`{}`)}, nil)

	rule, hit := matchBypass(context.Background(), requests, policy, req, []string{"admin"}, nil, nil)
	if rule == nil || rule.Name != "admin" {
		t.Errorf("second rule should match after the first trips its limit, got %+v", rule)
	}
	if hit == nil || hit.Rule != "limited" {
		t.Errorf("the tripped limit should still be reported, got %+v", hit)
	}

	rule, hit = matchBypass(context.Background(), requests, policy, req, []string{"teller"}, nil, nil)
	if rule != nil || hit == nil {
		t.Errorf("no rule should match for a teller, but the hit must be reported: rule=%+v hit=%+v", rule, hit)
	}
}

func TestCreate_LimitExceeded_RoutesToReviewWithMetadataAndAudit(t *testing.T) {
	one := 1
	policy := bypassPolicy(*limitedRule(model.BypassLimit{Window: "24h", MaxCount: &one}))
	f := newBypassCreateFixture(policy)
	f.requests.ListRecentByTypeFunc = func(ctx context.Context, reqType string, since time.Time, makerID string) ([]model.Request, error) {
		return []model.Request{priorRequest(`{"amount": 1}`)}, nil
	}
	svc := newTestRequestService(f.requests, &testutil.MockApprovalStore{}, f.policies, f.audits, nil)

	result, err := svc.Create(context.Background(), &model.Request{Type: "transfer", Payload: raw(`{"amount": 5}`), MakerID: "alice"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != model.StatusPending {
		t.Errorf("status = %v, want pending", result.Status)
	}
	var meta map[string]model.BypassLimitExceeded
	if err := json.Unmarshal(result.Metadata, &meta); err != nil {
		t.Fatalf("metadata = %s: %v", result.Metadata, err)
	}
	if h, ok := meta["bypass_limit_exceeded"]; !ok || h.Rule != "small" || h.Measure != "count" || h.Observed != 2 {
		t.Errorf("metadata.bypass_limit_exceeded = %+v", meta)
	}
	if len(f.actions) != 2 || f.actions[0] != "created" || f.actions[1] != "bypass_limit_exceeded" {
		t.Errorf("audit actions = %v", f.actions)
	}
}

func TestParseWindow(t *testing.T) {
	for in, want := range map[string]time.Duration{"24h": 24 * time.Hour, "7d": 7 * 24 * time.Hour, "90m": 90 * time.Minute} {
		if got, err := parseWindow(in); err != nil || got != want {
			t.Errorf("parseWindow(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "0d", "-1d", "d", "soon"} {
		if _, err := parseWindow(bad); err == nil {
			t.Errorf("parseWindow(%q) should fail", bad)
		}
	}
}

func TestValidateBypassRules_Limits(t *testing.T) {
	one := 1
	zero := 0
	cases := []struct {
		name    string
		limit   model.BypassLimit
		wantErr bool
	}{
		{"count ok", model.BypassLimit{Window: "24h", MaxCount: &one}, false},
		{"sum ok with days", model.BypassLimit{Window: "7d", MaxSum: &model.BypassSum{Path: "amount", Value: 10}}, false},
		{"group by ok", model.BypassLimit{Window: "1h", MaxCount: &one, GroupBy: []string{"maker", "payload:dest"}}, false},
		{"bad window", model.BypassLimit{Window: "soon", MaxCount: &one}, true},
		{"no measure", model.BypassLimit{Window: "24h"}, true},
		{"count zero", model.BypassLimit{Window: "24h", MaxCount: &zero}, true},
		{"sum no path", model.BypassLimit{Window: "24h", MaxSum: &model.BypassSum{Value: 10}}, true},
		{"bad group", model.BypassLimit{Window: "24h", MaxCount: &one, GroupBy: []string{"checker"}}, true},
		{"empty payload group", model.BypassLimit{Window: "24h", MaxCount: &one, GroupBy: []string{"payload:"}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rules := []model.BypassRule{{MakerRoles: []string{"x"}, Limits: []model.BypassLimit{tc.limit}}}
			err := validateBypassRules(rules)
			if (err != nil) != tc.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
