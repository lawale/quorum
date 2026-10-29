package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/auth"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/store"
	"github.com/lawale/quorum/internal/testutil"
)

type capturedEvent struct {
	event  string
	req    *model.Request
	review *model.ReviewRequest
}

// installEventCapture wires a transaction runner and event enqueuer that
// record every emitted event.
func installEventCapture(svc *RequestService, requests *testutil.MockRequestStore, approvals *testutil.MockApprovalStore) *[]capturedEvent {
	var events []capturedEvent
	svc.SetWebhookDispatch(
		func(ctx context.Context, fn func(tx *store.Stores) error) error {
			return fn(&store.Stores{Requests: requests, Approvals: approvals, Outbox: &testutil.MockOutboxStore{}, Webhooks: &testutil.MockWebhookStore{}})
		},
		func(ctx context.Context, outbox store.OutboxStore, webhooks store.WebhookStore, req *model.Request, approvals []model.Approval) error {
			return nil
		},
		func() {},
	)
	svc.SetEventEnqueue(func(ctx context.Context, outbox store.OutboxStore, webhooks store.WebhookStore, event string, req *model.Request, approvals []model.Approval, review *model.ReviewRequest) error {
		copied := *req
		events = append(events, capturedEvent{event: event, req: &copied, review: review})
		return nil
	})
	return &events
}

// countVote makes the approval mock report the vote just inserted, which is
// what the transactional decision path reads after recording it.
func countVote(approvals *testutil.MockApprovalStore) {
	approvals.CountByDecisionAndStageFunc = func(ctx context.Context, reqID uuid.UUID, decision model.Decision, stageIndex int) (int, error) {
		if decision == model.DecisionApproved {
			return 1, nil
		}
		return 0, nil
	}
}

func TestBuildReviewRequest_ResolvesStageAndScope(t *testing.T) {
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.Stages = []model.ApprovalStage{
			{Index: 0, Name: "Branch", RequiredApprovals: 1, RejectionPolicy: model.RejectionPolicyAny},
			{
				Index: 1, Name: "Receiving branch", RequiredApprovals: 2, RejectionPolicy: model.RejectionPolicyAny,
				AllowedCheckerRoles: []byte(`["branch_manager"]`),
				AllowedPermissions:  []byte(`["approve_transfer"]`),
				AuthorizationMode:   model.AuthModeAll,
				Scope: []model.ScopeRule{
					{Attribute: "branch_id", PayloadPath: "destination_branch_id", Values: []string{"HQ"}},
					{Attribute: "department"},
				},
			},
		}
	})
	req := testutil.NewRequest(func(r *model.Request) {
		r.Payload = []byte(`{"destination_branch_id": "ABJ-02"}`)
		r.MakerAttributes = map[string]string{"branch_id": "LAG-01", "department": "ops"}
		r.EligibleReviewers = []string{"bob", "carol"}
	})
	approvals := []model.Approval{
		{CheckerID: "zed", StageIndex: 0},
		{CheckerID: "yan", StageIndex: 1},
	}

	rr := buildReviewRequest(policy, req, 1, approvals, "stage_advanced")
	if rr == nil {
		t.Fatal("expected a review request")
	}
	if rr.Reason != "stage_advanced" || rr.StageIndex != 1 || rr.StageName != "Receiving branch" || rr.TotalStages != 2 || rr.RequiredApprovals != 2 {
		t.Errorf("stage fields = %+v", rr)
	}
	if len(rr.AllowedRoles) != 1 || rr.AllowedRoles[0] != "branch_manager" || len(rr.AllowedPermissions) != 1 || rr.AuthorizationMode != model.AuthModeAll {
		t.Errorf("authorization fields = %+v", rr)
	}
	if len(rr.EligibleReviewers) != 2 {
		t.Errorf("eligible reviewers = %v", rr.EligibleReviewers)
	}
	if len(rr.Scope) != 2 {
		t.Fatalf("scope = %+v", rr.Scope)
	}
	if rr.Scope[0].Attribute != "branch_id" || len(rr.Scope[0].Values) != 2 || rr.Scope[0].Values[0] != "ABJ-02" || rr.Scope[0].Values[1] != "HQ" {
		t.Errorf("scope[0] = %+v, want payload value then fixed value", rr.Scope[0])
	}
	if rr.Scope[1].Attribute != "department" || len(rr.Scope[1].Values) != 1 || rr.Scope[1].Values[0] != "ops" {
		t.Errorf("scope[1] = %+v, want maker's department", rr.Scope[1])
	}
	// Maker always excluded; only this stage's voters excluded.
	if len(rr.ExcludedUserIDs) != 2 || rr.ExcludedUserIDs[0] != req.MakerID || rr.ExcludedUserIDs[1] != "yan" {
		t.Errorf("excluded = %v", rr.ExcludedUserIDs)
	}
}

func TestBuildReviewRequest_UnresolvedScopeIsEmptyList(t *testing.T) {
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.Stages[0].Scope = []model.ScopeRule{{Attribute: "branch_id"}}
	})
	rr := buildReviewRequest(policy, testutil.NewRequest(), 0, nil, "created")
	b, _ := json.Marshal(rr.Scope)
	if string(b) != `[{"attribute":"branch_id","values":[]}]` {
		t.Errorf("scope json = %s", b)
	}
}

func TestBuildReviewRequest_OutOfRangeStage(t *testing.T) {
	if rr := buildReviewRequest(testutil.NewPolicy(), testutil.NewRequest(), 5, nil, "created"); rr != nil {
		t.Errorf("expected nil for missing stage, got %+v", rr)
	}
}

func TestCreate_EmitsReviewRequested(t *testing.T) {
	policy := testutil.NewPolicy()
	requests := &testutil.MockRequestStore{
		CreateFunc: func(ctx context.Context, r *model.Request) error {
			r.ID = uuid.New()
			r.Status = model.StatusPending
			return nil
		},
	}
	policies := &testutil.MockPolicyStore{GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) { return policy, nil }}
	audits := &testutil.MockAuditStore{CreateFunc: func(ctx context.Context, l *model.AuditLog) error { return nil }}
	svc := newTestRequestService(requests, &testutil.MockApprovalStore{}, policies, audits, nil)
	events := installEventCapture(svc, requests, &testutil.MockApprovalStore{})

	if _, err := svc.Create(context.Background(), &model.Request{Type: "transfer", Payload: []byte(`{"a":1}`), MakerID: "alice"}); err != nil {
		t.Fatal(err)
	}
	if len(*events) != 1 {
		t.Fatalf("events = %+v, want one", *events)
	}
	e := (*events)[0]
	if e.event != model.EventReviewRequested || e.review == nil || e.review.Reason != "created" || e.review.StageIndex != 0 {
		t.Errorf("event = %s review = %+v", e.event, e.review)
	}
	if e.req.ID == uuid.Nil {
		t.Error("event must carry the persisted request ID")
	}
}

func TestCreate_Bypassed_DoesNotEmitReviewRequested(t *testing.T) {
	policy := bypassPolicy(model.BypassRule{MakerRoles: []string{"root_admin"}})
	f := newBypassCreateFixture(policy)
	svc := newTestRequestService(f.requests, &testutil.MockApprovalStore{}, f.policies, f.audits, nil)
	events := installEventCapture(svc, f.requests, &testutil.MockApprovalStore{})

	ctx := auth.WithIdentity(context.Background(), &auth.Identity{UserID: "alice", TenantID: "t1", Roles: []string{"root_admin"}})
	if _, err := svc.Create(ctx, &model.Request{Type: "transfer", Payload: []byte(`{}`), MakerID: "alice"}); err != nil {
		t.Fatal(err)
	}
	for _, e := range *events {
		if e.event == model.EventReviewRequested {
			t.Errorf("bypassed request must not ask for review: %+v", e)
		}
	}
}

func TestCreate_LimitHit_EmitsReviewRequestedWithReason(t *testing.T) {
	one := 1
	policy := bypassPolicy(*limitedRule(model.BypassLimit{Window: "24h", MaxCount: &one}))
	f := newBypassCreateFixture(policy)
	f.requests.ListRecentByTypeFunc = func(ctx context.Context, reqType string, since time.Time, makerID string) ([]model.Request, error) {
		return []model.Request{priorRequest(`{"amount": 1}`)}, nil
	}
	svc := newTestRequestService(f.requests, &testutil.MockApprovalStore{}, f.policies, f.audits, nil)
	events := installEventCapture(svc, f.requests, &testutil.MockApprovalStore{})

	if _, err := svc.Create(context.Background(), &model.Request{Type: "transfer", Payload: []byte(`{"amount": 5}`), MakerID: "alice"}); err != nil {
		t.Fatal(err)
	}
	if len(*events) != 1 || (*events)[0].review == nil || (*events)[0].review.Reason != "bypass_limit_exceeded" {
		t.Errorf("events = %+v", *events)
	}
}

func TestApprove_StageAdvance_EmitsReviewRequestedForNextStage(t *testing.T) {
	req := testutil.NewRequest()
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.Stages = []model.ApprovalStage{
			{Index: 0, Name: "First", RequiredApprovals: 1, RejectionPolicy: model.RejectionPolicyAny},
			{Index: 1, Name: "Second", RequiredApprovals: 1, RejectionPolicy: model.RejectionPolicyAny, AllowedCheckerRoles: []byte(`["compliance"]`)},
		}
	})
	requests, approvals, policies, audits := setupApproveTest(req, policy)
	countVote(approvals)
	svc := newTestRequestService(requests, approvals, policies, audits, nil)
	events := installEventCapture(svc, requests, approvals)

	if _, err := svc.Approve(context.Background(), req.ID, "checker-1", nil, nil); err != nil {
		t.Fatal(err)
	}
	if len(*events) != 1 {
		t.Fatalf("events = %+v, want one review_requested", *events)
	}
	e := (*events)[0]
	if e.event != model.EventReviewRequested || e.review.Reason != "stage_advanced" || e.review.StageIndex != 1 || e.review.StageName != "Second" {
		t.Errorf("event = %+v review = %+v", e.event, e.review)
	}
	if e.req.CurrentStage != 1 {
		t.Errorf("event request current_stage = %d, want 1", e.req.CurrentStage)
	}
	if len(e.review.AllowedRoles) != 1 || e.review.AllowedRoles[0] != "compliance" {
		t.Errorf("allowed roles = %v", e.review.AllowedRoles)
	}
}

func TestApprove_Terminal_DoesNotEmitReviewRequested(t *testing.T) {
	req := testutil.NewRequest()
	requests, approvals, policies, audits := setupApproveTest(req, testutil.NewPolicy())
	countVote(approvals)
	svc := newTestRequestService(requests, approvals, policies, audits, nil)
	events := installEventCapture(svc, requests, approvals)

	if _, err := svc.Approve(context.Background(), req.ID, "checker-1", nil, nil); err != nil {
		t.Fatal(err)
	}
	if len(*events) != 0 {
		t.Errorf("terminal approval must not ask for review: %+v", *events)
	}
}

func TestCreate_NoEventEnqueuer_StillCreates(t *testing.T) {
	policy := testutil.NewPolicy()
	created := false
	requests := &testutil.MockRequestStore{CreateFunc: func(ctx context.Context, r *model.Request) error { created = true; return nil }}
	policies := &testutil.MockPolicyStore{GetByRequestTypeFunc: func(ctx context.Context, rt string) (*model.Policy, error) { return policy, nil }}
	audits := &testutil.MockAuditStore{CreateFunc: func(ctx context.Context, l *model.AuditLog) error { return nil }}
	svc := newTestRequestService(requests, &testutil.MockApprovalStore{}, policies, audits, nil)

	if _, err := svc.Create(context.Background(), testutil.NewRequest()); err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
}
