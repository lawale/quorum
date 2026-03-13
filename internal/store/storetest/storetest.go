// Package storetest provides shared integration tests that verify any store
// implementation correctly satisfies the store interfaces. Each driver's
// integration test calls into these functions with a real database connection.
package storetest

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/store"
)

// TestRequestStore exercises every method of store.RequestStore.
func TestRequestStore(t *testing.T, s store.RequestStore) {
	ctx := context.Background()

	t.Run("Create and GetByID", func(t *testing.T) {
		req := &model.Request{
			Type:    "transfer",
			Payload: json.RawMessage(`{"amount":100}`),
			Status:  model.StatusPending,
			MakerID: "user-1",
		}
		if err := s.Create(ctx, req); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if req.ID == uuid.Nil {
			t.Fatal("expected ID to be assigned")
		}

		got, err := s.GetByID(ctx, req.ID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got == nil {
			t.Fatal("expected non-nil result")
		}
		if got.Type != "transfer" {
			t.Errorf("Type = %q, want %q", got.Type, "transfer")
		}
		if got.MakerID != "user-1" {
			t.Errorf("MakerID = %q, want %q", got.MakerID, "user-1")
		}
		if string(got.Payload) != `{"amount":100}` {
			t.Errorf("Payload = %s, want %s", got.Payload, `{"amount":100}`)
		}
		if got.Status != model.StatusPending {
			t.Errorf("Status = %q, want %q", got.Status, model.StatusPending)
		}
	})

	t.Run("GetByID not found", func(t *testing.T) {
		got, err := s.GetByID(ctx, uuid.New())
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got != nil {
			t.Fatal("expected nil for non-existent ID")
		}
	})

	t.Run("IdempotencyKey", func(t *testing.T) {
		key := "idem-" + uuid.NewString()
		req := &model.Request{
			Type:           "transfer",
			Payload:        json.RawMessage(`{}`),
			MakerID:        "user-1",
			IdempotencyKey: &key,
		}
		if err := s.Create(ctx, req); err != nil {
			t.Fatalf("Create: %v", err)
		}

		got, err := s.GetByIdempotencyKey(ctx, key)
		if err != nil {
			t.Fatalf("GetByIdempotencyKey: %v", err)
		}
		if got == nil || got.ID != req.ID {
			t.Fatal("expected to find request by idempotency key")
		}
	})

	t.Run("GetByIdempotencyKey not found", func(t *testing.T) {
		got, err := s.GetByIdempotencyKey(ctx, "nonexistent-key")
		if err != nil {
			t.Fatalf("GetByIdempotencyKey: %v", err)
		}
		if got != nil {
			t.Fatal("expected nil for non-existent key")
		}
	})

	t.Run("FindPendingByFingerprint", func(t *testing.T) {
		fp := "fp-" + uuid.NewString()
		req := &model.Request{
			Type:        "transfer",
			Payload:     json.RawMessage(`{}`),
			MakerID:     "user-1",
			Fingerprint: &fp,
		}
		if err := s.Create(ctx, req); err != nil {
			t.Fatalf("Create: %v", err)
		}

		got, err := s.FindPendingByFingerprint(ctx, "transfer", fp)
		if err != nil {
			t.Fatalf("FindPendingByFingerprint: %v", err)
		}
		if got == nil || got.ID != req.ID {
			t.Fatal("expected to find pending request by fingerprint")
		}
	})

	t.Run("EligibleReviewers roundtrip", func(t *testing.T) {
		req := &model.Request{
			Type:              "transfer",
			Payload:           json.RawMessage(`{}`),
			MakerID:           "user-1",
			EligibleReviewers: []string{"reviewer-a", "reviewer-b"},
		}
		if err := s.Create(ctx, req); err != nil {
			t.Fatalf("Create: %v", err)
		}

		got, err := s.GetByID(ctx, req.ID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if len(got.EligibleReviewers) != 2 {
			t.Fatalf("EligibleReviewers length = %d, want 2", len(got.EligibleReviewers))
		}
		if got.EligibleReviewers[0] != "reviewer-a" || got.EligibleReviewers[1] != "reviewer-b" {
			t.Errorf("EligibleReviewers = %v", got.EligibleReviewers)
		}
	})

	t.Run("Metadata roundtrip", func(t *testing.T) {
		meta := json.RawMessage(`{"env":"prod"}`)
		req := &model.Request{
			Type:     "transfer",
			Payload:  json.RawMessage(`{}`),
			MakerID:  "user-1",
			Metadata: meta,
		}
		if err := s.Create(ctx, req); err != nil {
			t.Fatalf("Create: %v", err)
		}

		got, err := s.GetByID(ctx, req.ID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if string(got.Metadata) != `{"env":"prod"}` {
			t.Errorf("Metadata = %s, want %s", got.Metadata, `{"env":"prod"}`)
		}
	})

	t.Run("UpdateStatus", func(t *testing.T) {
		req := &model.Request{
			Type:    "transfer",
			Payload: json.RawMessage(`{}`),
			MakerID: "user-1",
		}
		if err := s.Create(ctx, req); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if err := s.UpdateStatus(ctx, req.ID, model.StatusApproved); err != nil {
			t.Fatalf("UpdateStatus: %v", err)
		}

		got, err := s.GetByID(ctx, req.ID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got.Status != model.StatusApproved {
			t.Errorf("Status = %q, want %q", got.Status, model.StatusApproved)
		}
	})

	t.Run("List with filters", func(t *testing.T) {
		// Create requests with distinct type
		listType := "list-test-" + uuid.NewString()
		for i := 0; i < 3; i++ {
			req := &model.Request{
				Type:    listType,
				Payload: json.RawMessage(`{}`),
				MakerID: "user-1",
			}
			if err := s.Create(ctx, req); err != nil {
				t.Fatalf("Create: %v", err)
			}
		}

		typeFilter := listType
		results, total, err := s.List(ctx, store.RequestFilter{Type: &typeFilter, Page: 1, PerPage: 10})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if total != 3 {
			t.Errorf("total = %d, want 3", total)
		}
		if len(results) != 3 {
			t.Errorf("results length = %d, want 3", len(results))
		}
	})

	t.Run("ListExpired", func(t *testing.T) {
		past := time.Now().UTC().Add(-time.Hour)
		req := &model.Request{
			Type:      "transfer",
			Payload:   json.RawMessage(`{}`),
			MakerID:   "user-1",
			ExpiresAt: &past,
		}
		if err := s.Create(ctx, req); err != nil {
			t.Fatalf("Create: %v", err)
		}

		expired, err := s.ListExpired(ctx)
		if err != nil {
			t.Fatalf("ListExpired: %v", err)
		}
		found := false
		for _, r := range expired {
			if r.ID == req.ID {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected expired request in ListExpired results")
		}
	})
}

// TestApprovalStore exercises every method of store.ApprovalStore.
func TestApprovalStore(t *testing.T, as store.ApprovalStore, rs store.RequestStore) {
	ctx := context.Background()

	// Helper to create a request for foreign key.
	createRequest := func(t *testing.T) uuid.UUID {
		t.Helper()
		req := &model.Request{
			Type:    "transfer",
			Payload: json.RawMessage(`{}`),
			MakerID: "user-1",
		}
		if err := rs.Create(ctx, req); err != nil {
			t.Fatalf("Create request: %v", err)
		}
		return req.ID
	}

	t.Run("Create and ListByRequestID", func(t *testing.T) {
		reqID := createRequest(t)

		approval := &model.Approval{
			RequestID: reqID,
			CheckerID: "checker-1",
			Decision:  model.DecisionApproved,
		}
		if err := as.Create(ctx, approval); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if approval.ID == uuid.Nil {
			t.Fatal("expected ID to be assigned")
		}

		list, err := as.ListByRequestID(ctx, reqID)
		if err != nil {
			t.Fatalf("ListByRequestID: %v", err)
		}
		if len(list) != 1 {
			t.Fatalf("length = %d, want 1", len(list))
		}
		if list[0].CheckerID != "checker-1" {
			t.Errorf("CheckerID = %q, want %q", list[0].CheckerID, "checker-1")
		}
	})

	t.Run("CountByDecision", func(t *testing.T) {
		reqID := createRequest(t)

		for _, cid := range []string{"c1", "c2", "c3"} {
			as.Create(ctx, &model.Approval{RequestID: reqID, CheckerID: cid, Decision: model.DecisionApproved})
		}
		as.Create(ctx, &model.Approval{RequestID: reqID, CheckerID: "c4", Decision: model.DecisionRejected})

		approved, err := as.CountByDecision(ctx, reqID, model.DecisionApproved)
		if err != nil {
			t.Fatalf("CountByDecision approved: %v", err)
		}
		if approved != 3 {
			t.Errorf("approved = %d, want 3", approved)
		}

		rejected, err := as.CountByDecision(ctx, reqID, model.DecisionRejected)
		if err != nil {
			t.Fatalf("CountByDecision rejected: %v", err)
		}
		if rejected != 1 {
			t.Errorf("rejected = %d, want 1", rejected)
		}
	})

	t.Run("ExistsByChecker", func(t *testing.T) {
		reqID := createRequest(t)
		as.Create(ctx, &model.Approval{RequestID: reqID, CheckerID: "checker-x", Decision: model.DecisionApproved})

		exists, err := as.ExistsByChecker(ctx, reqID, "checker-x")
		if err != nil {
			t.Fatalf("ExistsByChecker: %v", err)
		}
		if !exists {
			t.Error("expected true")
		}

		exists, err = as.ExistsByChecker(ctx, reqID, "checker-y")
		if err != nil {
			t.Fatalf("ExistsByChecker: %v", err)
		}
		if exists {
			t.Error("expected false")
		}
	})
}

// TestPolicyStore exercises every method of store.PolicyStore.
func TestPolicyStore(t *testing.T, s store.PolicyStore) {
	ctx := context.Background()

	t.Run("Create and GetByID", func(t *testing.T) {
		dur := 2 * time.Hour
		policy := &model.Policy{
			Name:                "Test Policy",
			RequestType:         "pol-test-" + uuid.NewString(),
			RequiredApprovals:   2,
			RejectionPolicy:     model.RejectionPolicyThreshold,
			AllowedCheckerRoles: json.RawMessage(`["admin","manager"]`),
			IdentityFields:      []string{"account_id"},
			AutoExpireDuration:  &dur,
		}
		if err := s.Create(ctx, policy); err != nil {
			t.Fatalf("Create: %v", err)
		}

		got, err := s.GetByID(ctx, policy.ID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got == nil {
			t.Fatal("expected non-nil")
		}
		if got.Name != "Test Policy" {
			t.Errorf("Name = %q, want %q", got.Name, "Test Policy")
		}
		if got.RequiredApprovals != 2 {
			t.Errorf("RequiredApprovals = %d, want 2", got.RequiredApprovals)
		}
		if got.RejectionPolicy != model.RejectionPolicyThreshold {
			t.Errorf("RejectionPolicy = %q, want %q", got.RejectionPolicy, model.RejectionPolicyThreshold)
		}
		if string(got.AllowedCheckerRoles) != `["admin","manager"]` {
			t.Errorf("AllowedCheckerRoles = %s", got.AllowedCheckerRoles)
		}
		if len(got.IdentityFields) != 1 || got.IdentityFields[0] != "account_id" {
			t.Errorf("IdentityFields = %v", got.IdentityFields)
		}
		if got.AutoExpireDuration == nil || *got.AutoExpireDuration != 2*time.Hour {
			t.Errorf("AutoExpireDuration = %v, want 2h", got.AutoExpireDuration)
		}
	})

	t.Run("GetByID not found", func(t *testing.T) {
		got, err := s.GetByID(ctx, uuid.New())
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got != nil {
			t.Fatal("expected nil")
		}
	})

	t.Run("GetByRequestType", func(t *testing.T) {
		rt := "rt-" + uuid.NewString()
		s.Create(ctx, &model.Policy{Name: "P", RequestType: rt, RequiredApprovals: 1, RejectionPolicy: model.RejectionPolicyAny})

		got, err := s.GetByRequestType(ctx, rt)
		if err != nil {
			t.Fatalf("GetByRequestType: %v", err)
		}
		if got == nil || got.RequestType != rt {
			t.Fatal("expected to find policy by request type")
		}
	})

	t.Run("List", func(t *testing.T) {
		list, err := s.List(ctx)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(list) == 0 {
			t.Error("expected at least one policy")
		}
	})

	t.Run("Update", func(t *testing.T) {
		rt := "upd-" + uuid.NewString()
		policy := &model.Policy{Name: "Original", RequestType: rt, RequiredApprovals: 1, RejectionPolicy: model.RejectionPolicyAny}
		s.Create(ctx, policy)

		policy.Name = "Updated"
		policy.RequiredApprovals = 5
		if err := s.Update(ctx, policy); err != nil {
			t.Fatalf("Update: %v", err)
		}

		got, _ := s.GetByID(ctx, policy.ID)
		if got.Name != "Updated" {
			t.Errorf("Name = %q, want %q", got.Name, "Updated")
		}
		if got.RequiredApprovals != 5 {
			t.Errorf("RequiredApprovals = %d, want 5", got.RequiredApprovals)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		rt := "del-" + uuid.NewString()
		policy := &model.Policy{Name: "ToDelete", RequestType: rt, RequiredApprovals: 1, RejectionPolicy: model.RejectionPolicyAny}
		s.Create(ctx, policy)

		if err := s.Delete(ctx, policy.ID); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		got, _ := s.GetByID(ctx, policy.ID)
		if got != nil {
			t.Error("expected nil after delete")
		}
	})
}

// TestWebhookStore exercises every method of store.WebhookStore.
func TestWebhookStore(t *testing.T, s store.WebhookStore) {
	ctx := context.Background()

	t.Run("Create and GetByID", func(t *testing.T) {
		wh := &model.Webhook{
			URL:    "https://example.com/hook",
			Events: []string{"approved", "rejected"},
			Secret: "s3cret",
		}
		if err := s.Create(ctx, wh); err != nil {
			t.Fatalf("Create: %v", err)
		}

		got, err := s.GetByID(ctx, wh.ID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got == nil {
			t.Fatal("expected non-nil")
		}
		if got.URL != "https://example.com/hook" {
			t.Errorf("URL = %q", got.URL)
		}
		if len(got.Events) != 2 {
			t.Fatalf("Events length = %d, want 2", len(got.Events))
		}
		if !got.Active {
			t.Error("expected Active=true")
		}
	})

	t.Run("GetByID not found", func(t *testing.T) {
		got, err := s.GetByID(ctx, uuid.New())
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got != nil {
			t.Fatal("expected nil")
		}
	})

	t.Run("ListByEventAndType", func(t *testing.T) {
		rt := "wh-type-" + uuid.NewString()
		s.Create(ctx, &model.Webhook{URL: "https://a.com", Events: []string{"approved"}, Secret: "s", RequestType: &rt})
		s.Create(ctx, &model.Webhook{URL: "https://b.com", Events: []string{"rejected"}, Secret: "s", RequestType: &rt})
		// Global webhook (no request type)
		s.Create(ctx, &model.Webhook{URL: "https://c.com", Events: []string{"approved"}, Secret: "s"})

		results, err := s.ListByEventAndType(ctx, "approved", rt)
		if err != nil {
			t.Fatalf("ListByEventAndType: %v", err)
		}
		// Should match the type-specific one and the global one
		if len(results) < 2 {
			t.Errorf("expected at least 2 matching webhooks, got %d", len(results))
		}
	})

	t.Run("List", func(t *testing.T) {
		list, err := s.List(ctx)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(list) == 0 {
			t.Error("expected at least one webhook")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		wh := &model.Webhook{URL: "https://del.com", Events: []string{"approved"}, Secret: "s"}
		s.Create(ctx, wh)

		if err := s.Delete(ctx, wh.ID); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		got, _ := s.GetByID(ctx, wh.ID)
		if got != nil {
			t.Error("expected nil after delete")
		}
	})
}

// TestAuditStore exercises every method of store.AuditStore.
func TestAuditStore(t *testing.T, as store.AuditStore, rs store.RequestStore) {
	ctx := context.Background()

	t.Run("Create and ListByRequestID", func(t *testing.T) {
		req := &model.Request{Type: "transfer", Payload: json.RawMessage(`{}`), MakerID: "user-1"}
		rs.Create(ctx, req)

		log := &model.AuditLog{
			RequestID: req.ID,
			Action:    "created",
			ActorID:   "user-1",
			Details:   json.RawMessage(`{"note":"test"}`),
		}
		if err := as.Create(ctx, log); err != nil {
			t.Fatalf("Create: %v", err)
		}

		logs, err := as.ListByRequestID(ctx, req.ID)
		if err != nil {
			t.Fatalf("ListByRequestID: %v", err)
		}
		if len(logs) != 1 {
			t.Fatalf("length = %d, want 1", len(logs))
		}
		if logs[0].Action != "created" {
			t.Errorf("Action = %q, want %q", logs[0].Action, "created")
		}
		if string(logs[0].Details) != `{"note":"test"}` {
			t.Errorf("Details = %s", logs[0].Details)
		}
	})
}
