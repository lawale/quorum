package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lawale/quorum/clients/go"
	"github.com/lawale/quorum/internal/apitest"
	"github.com/lawale/quorum/internal/model"
)

const tenant = "banking"

var (
	alice = client.Identity{UserID: "alice", Roles: []string{"teller"}, Attributes: map[string]string{"branch_id": "LAG-01"}}
	bob   = client.Identity{UserID: "bob", Roles: []string{"finance"}, Attributes: map[string]string{"branch_id": "LAG-01"}}
	carol = client.Identity{UserID: "carol", Roles: []string{"manager"}, Attributes: map[string]string{"branch_id": "ABJ-02"}}
	hq    = client.Identity{UserID: "hq", Roles: []string{"manager"}}
)

func newClient(t *testing.T) *client.Client {
	t.Helper()
	srv := apitest.New(t)
	c, err := client.New(srv.URL,
		client.WithTenant(tenant),
		client.WithTrustHeaderNames(client.HeaderNames{Attributes: map[string]string{"branch_id": apitest.BranchHeader}}),
	)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func wirePolicy(t *testing.T, c *client.Client, stages ...client.ApprovalStage) *client.Policy {
	t.Helper()
	if len(stages) == 0 {
		stages = []client.ApprovalStage{
			{Index: 0, Name: "Finance", RequiredApprovals: 1, AllowedCheckerRoles: []string{"finance"}},
			{Index: 1, Name: "Management", RequiredApprovals: 1, AllowedCheckerRoles: []string{"manager"}},
		}
	}
	p, err := c.As(hq).CreatePolicy(context.Background(), client.PolicyInput{
		Name:               "Wire transfers",
		RequestType:        "wire_transfer",
		Stages:             stages,
		IdentityFields:     []string{"transfer_id"},
		AutoExpireDuration: client.NewDuration(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create policy: %v", err)
	}
	return p
}

type transfer struct {
	TransferID string `json:"transfer_id"`
	Amount     int    `json:"amount"`
}

func TestWireTransferGoesThroughBothStages(t *testing.T) {
	ctx := context.Background()
	c := newClient(t)
	wirePolicy(t, c)

	created, err := c.As(alice).CreateRequest(ctx, client.CreateRequest{
		Type:           "wire_transfer",
		Payload:        transfer{TransferID: "T-1", Amount: 5000},
		IdempotencyKey: "T-1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Status != client.StatusPending || created.MakerID != "alice" || created.TenantID != tenant {
		t.Fatalf("created = %+v", created)
	}
	if created.MakerAttributes["branch_id"] != "LAG-01" {
		t.Errorf("maker attributes not captured: %v", created.MakerAttributes)
	}

	replay, err := c.As(alice).CreateRequest(ctx, client.CreateRequest{Type: "wire_transfer", Payload: transfer{TransferID: "T-1", Amount: 5000}, IdempotencyKey: "T-1"})
	if err != nil || replay.ID != created.ID {
		t.Fatalf("idempotent replay: id=%v err=%v, want %v", replay, err, created.ID)
	}

	_, err = c.As(alice).CreateRequest(ctx, client.CreateRequest{Type: "wire_transfer", Payload: transfer{TransferID: "T-1", Amount: 1}})
	if !client.IsConflict(err) {
		t.Fatalf("duplicate pending request: got %v, want conflict", err)
	}

	asBob, err := c.As(bob).GetRequest(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if asBob.ViewerCanAct == nil || !*asBob.ViewerCanAct {
		t.Errorf("bob (finance) should be able to act at stage 0")
	}
	if asBob.TotalStages == nil || *asBob.TotalStages != 2 {
		t.Errorf("total_stages = %v, want 2", asBob.TotalStages)
	}

	if _, err := c.As(alice).Approve(ctx, created.ID, ""); !client.IsForbidden(err) {
		t.Errorf("self-approval: got %v, want forbidden", err)
	}
	if _, err := c.As(carol).Approve(ctx, created.ID, ""); !client.IsForbidden(err) {
		t.Errorf("manager at finance stage: got %v, want forbidden", err)
	}

	afterBob, err := c.As(bob).Approve(ctx, created.ID, "looks right")
	if err != nil {
		t.Fatalf("bob approve: %v", err)
	}
	if afterBob.Status != client.StatusPending || afterBob.CurrentStage != 1 {
		t.Fatalf("after stage 0: status=%s stage=%d, want pending/1", afterBob.Status, afterBob.CurrentStage)
	}
	if _, err := c.As(bob).Approve(ctx, created.ID, ""); !client.IsForbidden(err) {
		t.Errorf("bob again at management stage: got %v, want forbidden", err)
	}

	final, err := c.As(carol).Approve(ctx, created.ID, "")
	if err != nil {
		t.Fatalf("carol approve: %v", err)
	}
	if final.Status != client.StatusApproved {
		t.Fatalf("final status = %s, want approved", final.Status)
	}
	if len(final.Approvals) != 2 || final.Approvals[0].Comment == nil || *final.Approvals[0].Comment != "looks right" {
		t.Errorf("approvals = %+v", final.Approvals)
	}

	if _, err := c.As(hq).Approve(ctx, created.ID, ""); !client.IsConflict(err) {
		t.Errorf("approve after terminal: got %v, want conflict", err)
	}

	trail, err := c.As(alice).Audit(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	var actions []string
	for _, l := range trail {
		actions = append(actions, l.Action)
	}
	want := "created,approved,stage_advanced,approved"
	if got := strings.Join(actions, ","); got != want {
		t.Errorf("audit actions = %s, want %s", got, want)
	}

	approved, err := c.As(hq).ListRequests(ctx, client.ListRequests{Status: client.StatusApproved, Type: "wire_transfer"})
	if err != nil {
		t.Fatal(err)
	}
	if approved.Total != 1 || len(approved.Data) != 1 || approved.Data[0].ID != created.ID {
		t.Errorf("approved list = %+v", approved)
	}
	pending, err := c.As(hq).ListRequests(ctx, client.ListRequests{Status: client.StatusPending})
	if err != nil {
		t.Fatal(err)
	}
	if pending.Total != 0 || len(pending.Data) != 0 {
		t.Errorf("pending list = %+v, want empty", pending)
	}

	if _, err := c.As(alice).GetRequest(ctx, uuid.New()); !client.IsNotFound(err) {
		t.Errorf("unknown request: got %v, want not found", err)
	}
}

func TestRejectAndCancel(t *testing.T) {
	ctx := context.Background()
	c := newClient(t)
	wirePolicy(t, c)

	rejected, err := c.As(alice).CreateRequest(ctx, client.CreateRequest{Type: "wire_transfer", Payload: transfer{TransferID: "T-2", Amount: 10}})
	if err != nil {
		t.Fatal(err)
	}
	res, err := c.As(bob).Reject(ctx, rejected.ID, "wrong account")
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if res.Status != client.StatusRejected {
		t.Errorf("status = %s, want rejected", res.Status)
	}

	cancelled, err := c.As(alice).CreateRequest(ctx, client.CreateRequest{Type: "wire_transfer", Payload: transfer{TransferID: "T-3", Amount: 10}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.As(bob).Cancel(ctx, cancelled.ID); !client.IsForbidden(err) {
		t.Errorf("cancel by non-maker: got %v, want forbidden", err)
	}
	res, err = c.As(alice).Cancel(ctx, cancelled.ID)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if res.Status != client.StatusCancelled {
		t.Errorf("status = %s, want cancelled", res.Status)
	}
	if _, err := c.As(alice).Cancel(ctx, cancelled.ID); !client.IsConflict(err) {
		t.Errorf("cancel twice: got %v, want conflict", err)
	}
}

func TestBranchScopeFollowsTheMaker(t *testing.T) {
	ctx := context.Background()
	c := newClient(t)
	wirePolicy(t, c, client.ApprovalStage{
		Index:               0,
		RequiredApprovals:   1,
		AllowedCheckerRoles: []string{"finance", "manager"},
		Scope:               []client.ScopeRule{{Attribute: "branch_id", MatchMaker: true, AllowUnscopedCheckers: true}},
	})

	req, err := c.As(alice).CreateRequest(ctx, client.CreateRequest{Type: "wire_transfer", Payload: transfer{TransferID: "T-4", Amount: 10}})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := c.As(carol).Approve(ctx, req.ID, ""); !client.IsForbidden(err) {
		t.Errorf("manager from another branch: got %v, want forbidden", err)
	}
	seen, err := c.As(carol).GetRequest(ctx, req.ID)
	if err != nil {
		t.Fatal(err)
	}
	if seen.ViewerCanAct != nil && *seen.ViewerCanAct {
		t.Errorf("carol should not be told she can act")
	}

	// Head office carries no branch at all, which the rule allows.
	res, err := c.As(hq).Approve(ctx, req.ID, "")
	if err != nil {
		t.Fatalf("head office approve: %v", err)
	}
	if res.Status != client.StatusApproved {
		t.Errorf("status = %s, want approved", res.Status)
	}
}

func TestWaitForDecisionFollowsTheStream(t *testing.T) {
	ctx := context.Background()
	c := newClient(t)
	wirePolicy(t, c, client.ApprovalStage{Index: 0, RequiredApprovals: 1, AllowedCheckerRoles: []string{"finance"}})

	req, err := c.As(alice).CreateRequest(ctx, client.CreateRequest{Type: "wire_transfer", Payload: transfer{TransferID: "T-5", Amount: 10}})
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan *client.Request, 1)
	errs := make(chan error, 1)
	go func() {
		waitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		r, err := c.As(alice).WaitForDecision(waitCtx, req.ID, client.WaitOptions{PollInterval: 50 * time.Millisecond})
		if err != nil {
			errs <- err
			return
		}
		done <- r
	}()

	time.Sleep(100 * time.Millisecond)
	if _, err := c.As(bob).Approve(ctx, req.ID, ""); err != nil {
		t.Fatal(err)
	}

	select {
	case r := <-done:
		if r.Status != client.StatusApproved {
			t.Errorf("waited status = %s, want approved", r.Status)
		}
	case err := <-errs:
		t.Fatalf("wait: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("WaitForDecision did not return after approval")
	}

	r, err := c.As(alice).WaitForDecision(ctx, req.ID)
	if err != nil || r.Status != client.StatusApproved {
		t.Fatalf("wait on decided request: %v %v", r, err)
	}

	pending, err := c.As(alice).CreateRequest(ctx, client.CreateRequest{Type: "wire_transfer", Payload: transfer{TransferID: "T-6", Amount: 10}})
	if err != nil {
		t.Fatal(err)
	}
	shortCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	if _, err := c.As(alice).WaitForDecision(shortCtx, pending.ID); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("wait past deadline: got %v, want deadline exceeded", err)
	}

	var events []client.Event
	for ev, err := range c.As(alice).Events(ctx, req.ID) {
		if err != nil {
			t.Fatal(err)
		}
		events = append(events, ev)
	}
	if len(events) != 1 || !events[0].Terminal() || events[0].Status != client.StatusApproved {
		t.Errorf("events for decided request = %+v", events)
	}
}

func TestPolicyLifecycle(t *testing.T) {
	ctx := context.Background()
	c := newClient(t).As(hq)

	p, err := c.CreatePolicy(ctx, client.PolicyInput{
		Name:        "Expenses",
		RequestType: "expense",
		Stages:      []client.ApprovalStage{{Index: 0, RequiredApprovals: 1, AllowedCheckerRoles: []string{"manager"}}},
		BypassRules: []client.BypassRule{{
			Name:    "petty cash",
			Payload: []client.BypassCondition{{Path: "amount", Op: "lt", Value: json.RawMessage(`50`)}},
			Limits:  []client.BypassLimit{{Window: "1d", MaxCount: intPtr(3)}},
		}},
		AutoExpireDuration: client.NewDuration(72 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.AutoExpireDuration == nil || p.AutoExpireDuration.Std() != 72*time.Hour {
		t.Errorf("auto expire = %v, want 72h", p.AutoExpireDuration)
	}

	if _, err := c.CreatePolicy(ctx, client.PolicyInput{Name: "Again", RequestType: "expense", Stages: p.Stages}); !client.IsConflict(err) {
		t.Errorf("second policy for same type: got %v, want conflict", err)
	}

	var apiErr *client.Error
	_, err = c.CreatePolicy(ctx, client.PolicyInput{Name: "Bad", RequestType: "bad", Stages: []client.ApprovalStage{{Index: 0, RequiredApprovals: 1, RejectionPolicy: "threshold"}}})
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusBadRequest || !strings.Contains(apiErr.Message, "max_checkers") {
		t.Errorf("invalid policy: got %v, want 400 mentioning max_checkers", err)
	}

	small, err := c.As(alice).CreateRequest(ctx, client.CreateRequest{Type: "expense", Payload: map[string]any{"amount": 20}})
	if err != nil {
		t.Fatal(err)
	}
	if small.Status != client.StatusApproved {
		t.Errorf("small expense status = %s, want approved by bypass", small.Status)
	}

	got, err := c.GetPolicy(ctx, p.ID)
	if err != nil || got.Name != "Expenses" || len(got.BypassRules) != 1 {
		t.Fatalf("get: %+v %v", got, err)
	}

	list, err := c.ListPolicies(ctx, client.Paging{PerPage: 10})
	if err != nil || list.Total != 1 || list.Data[0].ID != p.ID {
		t.Fatalf("list: %+v %v", list, err)
	}

	updated, err := c.UpdatePolicy(ctx, p.ID, client.PolicyUpdate{Name: "Expenses v2", BypassRules: []client.BypassRule{}})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "Expenses v2" || len(updated.BypassRules) != 0 || len(updated.Stages) != 1 {
		t.Errorf("updated = %+v", updated)
	}

	again, err := c.As(alice).CreateRequest(ctx, client.CreateRequest{Type: "expense", Payload: map[string]any{"amount": 20}})
	if err != nil {
		t.Fatal(err)
	}
	if again.Status != client.StatusPending {
		t.Errorf("after clearing bypass, status = %s, want pending", again.Status)
	}

	if err := c.DeletePolicy(ctx, p.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := c.GetPolicy(ctx, p.ID); !client.IsNotFound(err) {
		t.Errorf("get after delete: got %v, want not found", err)
	}
	if err := c.DeletePolicy(ctx, p.ID); !client.IsNotFound(err) {
		t.Errorf("delete twice: got %v, want not found", err)
	}
}

func TestWebhookLifecycle(t *testing.T) {
	ctx := context.Background()
	c := newClient(t).As(hq)

	_, err := c.CreateWebhook(ctx, client.WebhookInput{URL: "https://hooks.example.com/quorum", Secret: "s3cret"})
	if !client.IsBadRequest(err) {
		t.Errorf("webhook with no events: got %v, want bad request", err)
	}

	wh, err := c.CreateWebhook(ctx, client.WebhookInput{
		URL:         "https://hooks.example.com/quorum",
		Events:      []string{client.EventApproved, client.EventRejected, client.EventReviewRequested},
		Secret:      "s3cret",
		RequestType: strPtr("wire_transfer"),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !wh.Active || wh.Secret != "" {
		t.Errorf("created webhook = %+v: want active with secret withheld", wh)
	}

	list, err := c.ListWebhooks(ctx, client.Paging{})
	if err != nil || list.Total != 1 || list.Data[0].ID != wh.ID {
		t.Fatalf("list: %+v %v", list, err)
	}

	if err := c.DeleteWebhook(ctx, wh.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := c.DeleteWebhook(ctx, wh.ID); !client.IsNotFound(err) {
		t.Errorf("delete twice: got %v, want not found", err)
	}
}

func TestParseWebhookAcceptsOnlyGenuineDeliveries(t *testing.T) {
	const secret = "s3cret"
	payload := model.WebhookPayload{
		Event: client.EventReviewRequested,
		Request: client.Request{
			ID: uuid.New(), TenantID: tenant, Type: "wire_transfer", Status: client.StatusPending,
			Payload: json.RawMessage(`{"transfer_id":"T-9"}`), MakerID: "alice",
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		},
		Review: &client.ReviewRequest{
			Reason:            client.ReviewReasonCreated,
			RequiredApprovals: 1,
			AllowedRoles:      []string{"finance"},
			Scope:             []client.ResolvedScope{{Attribute: "branch_id", Values: []string{"LAG-01"}}},
			ExcludedUserIDs:   []string{"alice"},
		},
		Timestamp: time.Now(),
	}
	body, _ := json.Marshal(payload)

	deliver := func(body []byte, sig string) (*client.WebhookPayload, error) {
		r := httptest.NewRequest(http.MethodPost, "/webhooks/quorum", strings.NewReader(string(body)))
		r.Header.Set(client.SignatureHeader, sig)
		return client.ParseWebhook(r, secret)
	}

	got, err := deliver(body, client.Sign(body, secret))
	if err != nil {
		t.Fatalf("genuine delivery rejected: %v", err)
	}
	if got.Event != client.EventReviewRequested || got.Request.ID != payload.Request.ID || got.Review == nil || got.Review.Scope[0].Values[0] != "LAG-01" {
		t.Errorf("decoded = %+v", got)
	}

	tampered := []byte(strings.Replace(string(body), `"alice"`, `"mallory"`, 1))
	if _, err := deliver(tampered, client.Sign(body, secret)); !errors.Is(err, client.ErrInvalidSignature) {
		t.Errorf("tampered body: got %v, want ErrInvalidSignature", err)
	}
	if _, err := deliver(body, client.Sign(body, "other")); !errors.Is(err, client.ErrInvalidSignature) {
		t.Errorf("wrong secret: got %v, want ErrInvalidSignature", err)
	}
	if _, err := deliver(body, ""); !errors.Is(err, client.ErrInvalidSignature) {
		t.Errorf("missing signature: got %v, want ErrInvalidSignature", err)
	}
}

func TestIdentityIsRequired(t *testing.T) {
	ctx := context.Background()
	c := newClient(t)

	if _, err := c.ListRequests(ctx, client.ListRequests{}); !client.IsUnauthorized(err) {
		t.Errorf("no credentials: got %v, want unauthorized", err)
	}

	h, err := c.Health(ctx)
	if err != nil || h.Status != "healthy" || h.Version != apitest.Version {
		t.Errorf("health needs no identity and reports the build version: %+v %v", h, err)
	}
}

func intPtr(i int) *int       { return &i }
func strPtr(s string) *string { return &s }
