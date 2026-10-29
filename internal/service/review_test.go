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

// installEventCapture wires a pass-through transaction runner over the given
// stores and a sink that records every emitted event.
func installEventCapture(svc *RequestService, requests *testutil.MockRequestStore, approvals *testutil.MockApprovalStore) *fakeSink {
	sink := &fakeSink{}
	svc.SetWebhookDispatch(passThrough(&store.Stores{Requests: requests, Approvals: approvals}), sink)
	return sink
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

	rr := buildReviewRequest(policy, req, 1, model.ReviewReasonStageAdvanced)
	if rr == nil {
		t.Fatal("expected a review request")
	}
	if rr.Reason != model.ReviewReasonStageAdvanced || rr.StageIndex != 1 || rr.StageName != "Receiving branch" || rr.TotalStages != 2 || rr.RequiredApprovals != 2 {
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
	if len(rr.ExcludedUserIDs) != 1 || rr.ExcludedUserIDs[0] != req.MakerID {
		t.Errorf("excluded = %v, want only the maker", rr.ExcludedUserIDs)
	}
}

func TestBuildReviewRequest_UnresolvedScopeIsEmptyList(t *testing.T) {
	policy := testutil.NewPolicy(func(p *model.Policy) {
		p.Stages[0].Scope = []model.ScopeRule{{Attribute: "branch_id"}}
	})
	rr := buildReviewRequest(policy, testutil.NewRequest(), 0, model.ReviewReasonCreated)
	b, _ := json.Marshal(rr.Scope)
	if string(b) != `[{"attribute":"branch_id","values":[]}]` {
		t.Errorf("scope json = %s", b)
	}
}

func TestBuildReviewRequest_OutOfRangeStage(t *testing.T) {
	if rr := buildReviewRequest(testutil.NewPolicy(), testutil.NewRequest(), 5, model.ReviewReasonCreated); rr != nil {
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
	sink := installEventCapture(svc, requests, &testutil.MockApprovalStore{})

	if _, err := svc.Create(context.Background(), &model.Request{Type: "transfer", Payload: []byte(`{"a":1}`), MakerID: "alice"}); err != nil {
		t.Fatal(err)
	}
	if len(sink.events) != 1 {
		t.Fatalf("events = %+v, want one", sink.events)
	}
	e := sink.events[0]
	if e.Name != model.EventReviewRequested || e.Review == nil || e.Review.Reason != model.ReviewReasonCreated || e.Review.StageIndex != 0 {
		t.Errorf("event = %s review = %+v", e.Name, e.Review)
	}
	if e.Request.ID == uuid.Nil {
		t.Error("event must carry the persisted request ID")
	}
}

func TestCreate_Bypassed_DoesNotEmitReviewRequested(t *testing.T) {
	policy := bypassPolicy(model.BypassRule{MakerRoles: []string{"root_admin"}})
	f := newBypassCreateFixture(policy)
	svc := newTestRequestService(f.requests, &testutil.MockApprovalStore{}, f.policies, f.audits, nil)
	sink := installEventCapture(svc, f.requests, &testutil.MockApprovalStore{})

	ctx := auth.WithIdentity(context.Background(), &auth.Identity{UserID: "alice", TenantID: "t1", Roles: []string{"root_admin"}})
	if _, err := svc.Create(ctx, &model.Request{Type: "transfer", Payload: []byte(`{}`), MakerID: "alice"}); err != nil {
		t.Fatal(err)
	}
	for _, e := range sink.events {
		if e.Name == model.EventReviewRequested {
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
	sink := installEventCapture(svc, f.requests, &testutil.MockApprovalStore{})

	if _, err := svc.Create(context.Background(), &model.Request{Type: "transfer", Payload: []byte(`{"amount": 5}`), MakerID: "alice"}); err != nil {
		t.Fatal(err)
	}
	if len(sink.events) != 1 || sink.events[0].Review == nil || sink.events[0].Review.Reason != model.ReviewReasonBypassLimitExceeded {
		t.Errorf("events = %+v", sink.events)
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
	svc := newTestRequestService(requests, approvals, policies, audits, nil)
	sink := installEventCapture(svc, requests, approvals)

	if _, err := svc.Approve(context.Background(), req.ID, "checker-1", nil, nil); err != nil {
		t.Fatal(err)
	}
	if len(sink.events) != 1 {
		t.Fatalf("events = %+v, want one review_requested", sink.events)
	}
	e := sink.events[0]
	if e.Name != model.EventReviewRequested || e.Review.Reason != model.ReviewReasonStageAdvanced || e.Review.StageIndex != 1 || e.Review.StageName != "Second" {
		t.Errorf("event = %+v review = %+v", e.Name, e.Review)
	}
	if e.Request.CurrentStage != 1 {
		t.Errorf("event request current_stage = %d, want 1", e.Request.CurrentStage)
	}
	if len(e.Review.AllowedRoles) != 1 || e.Review.AllowedRoles[0] != "compliance" {
		t.Errorf("allowed roles = %v", e.Review.AllowedRoles)
	}
}

func TestApprove_Terminal_DoesNotEmitReviewRequested(t *testing.T) {
	req := testutil.NewRequest()
	requests, approvals, policies, audits := setupApproveTest(req, testutil.NewPolicy())
	svc := newTestRequestService(requests, approvals, policies, audits, nil)
	sink := installEventCapture(svc, requests, approvals)

	if _, err := svc.Approve(context.Background(), req.ID, "checker-1", nil, nil); err != nil {
		t.Fatal(err)
	}
	if len(sink.events) != 1 || sink.events[0].Name != "approved" || sink.events[0].Review != nil {
		t.Errorf("terminal approval should emit exactly one approved event and no review request: %+v", sink.events)
	}
}

func TestCreate_NoDispatch_StillCreates(t *testing.T) {
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
