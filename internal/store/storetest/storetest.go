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
		if got.CurrentStage != 0 {
			t.Errorf("CurrentStage = %d, want 0", got.CurrentStage)
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

	t.Run("UpdateStageAndStatus", func(t *testing.T) {
		req := &model.Request{
			Type:    "transfer",
			Payload: json.RawMessage(`{}`),
			MakerID: "user-1",
		}
		if err := s.Create(ctx, req); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if err := s.UpdateStageAndStatus(ctx, req.ID, 2, model.StatusPending); err != nil {
			t.Fatalf("UpdateStageAndStatus: %v", err)
		}

		got, err := s.GetByID(ctx, req.ID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got.CurrentStage != 2 {
			t.Errorf("CurrentStage = %d, want 2", got.CurrentStage)
		}
		if got.Status != model.StatusPending {
			t.Errorf("Status = %q, want %q", got.Status, model.StatusPending)
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
		if list[0].StageIndex != 0 {
			t.Errorf("StageIndex = %d, want 0", list[0].StageIndex)
		}
	})

	t.Run("StageIndex roundtrip", func(t *testing.T) {
		reqID := createRequest(t)

		approval := &model.Approval{
			RequestID:  reqID,
			CheckerID:  "checker-stage-2",
			Decision:   model.DecisionApproved,
			StageIndex: 2,
		}
		if err := as.Create(ctx, approval); err != nil {
			t.Fatalf("Create: %v", err)
		}

		list, err := as.ListByRequestID(ctx, reqID)
		if err != nil {
			t.Fatalf("ListByRequestID: %v", err)
		}
		found := false
		for _, a := range list {
			if a.CheckerID == "checker-stage-2" && a.StageIndex == 2 {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find approval with StageIndex=2")
		}
	})

	t.Run("CountByDecisionAndStage", func(t *testing.T) {
		reqID := createRequest(t)

		// Stage 0: 3 approvals, 1 rejection
		for _, cid := range []string{"c1", "c2", "c3"} {
			as.Create(ctx, &model.Approval{RequestID: reqID, CheckerID: cid, Decision: model.DecisionApproved, StageIndex: 0})
		}
		as.Create(ctx, &model.Approval{RequestID: reqID, CheckerID: "c4", Decision: model.DecisionRejected, StageIndex: 0})

		// Stage 1: 1 approval
		as.Create(ctx, &model.Approval{RequestID: reqID, CheckerID: "c5", Decision: model.DecisionApproved, StageIndex: 1})

		approved, err := as.CountByDecisionAndStage(ctx, reqID, model.DecisionApproved, 0)
		if err != nil {
			t.Fatalf("CountByDecisionAndStage approved stage 0: %v", err)
		}
		if approved != 3 {
			t.Errorf("approved stage 0 = %d, want 3", approved)
		}

		rejected, err := as.CountByDecisionAndStage(ctx, reqID, model.DecisionRejected, 0)
		if err != nil {
			t.Fatalf("CountByDecisionAndStage rejected stage 0: %v", err)
		}
		if rejected != 1 {
			t.Errorf("rejected stage 0 = %d, want 1", rejected)
		}

		approvedS1, err := as.CountByDecisionAndStage(ctx, reqID, model.DecisionApproved, 1)
		if err != nil {
			t.Fatalf("CountByDecisionAndStage approved stage 1: %v", err)
		}
		if approvedS1 != 1 {
			t.Errorf("approved stage 1 = %d, want 1", approvedS1)
		}
	})

	t.Run("ExistsByCheckerAndStage", func(t *testing.T) {
		reqID := createRequest(t)
		as.Create(ctx, &model.Approval{RequestID: reqID, CheckerID: "checker-x", Decision: model.DecisionApproved, StageIndex: 0})

		exists, err := as.ExistsByCheckerAndStage(ctx, reqID, "checker-x", 0)
		if err != nil {
			t.Fatalf("ExistsByCheckerAndStage: %v", err)
		}
		if !exists {
			t.Error("expected true for checker-x at stage 0")
		}

		// Same checker, different stage — should not exist
		exists, err = as.ExistsByCheckerAndStage(ctx, reqID, "checker-x", 1)
		if err != nil {
			t.Fatalf("ExistsByCheckerAndStage: %v", err)
		}
		if exists {
			t.Error("expected false for checker-x at stage 1")
		}

		exists, err = as.ExistsByCheckerAndStage(ctx, reqID, "checker-y", 0)
		if err != nil {
			t.Fatalf("ExistsByCheckerAndStage: %v", err)
		}
		if exists {
			t.Error("expected false for checker-y")
		}
	})

	t.Run("Same checker different stages", func(t *testing.T) {
		reqID := createRequest(t)

		// A checker can act in multiple stages
		if err := as.Create(ctx, &model.Approval{RequestID: reqID, CheckerID: "multi-checker", Decision: model.DecisionApproved, StageIndex: 0}); err != nil {
			t.Fatalf("Create stage 0: %v", err)
		}
		if err := as.Create(ctx, &model.Approval{RequestID: reqID, CheckerID: "multi-checker", Decision: model.DecisionApproved, StageIndex: 1}); err != nil {
			t.Fatalf("Create stage 1: %v", err)
		}

		list, err := as.ListByRequestID(ctx, reqID)
		if err != nil {
			t.Fatalf("ListByRequestID: %v", err)
		}
		count := 0
		for _, a := range list {
			if a.CheckerID == "multi-checker" {
				count++
			}
		}
		if count != 2 {
			t.Errorf("expected 2 approvals from multi-checker, got %d", count)
		}
	})
}

// TestPolicyStore exercises every method of store.PolicyStore.
func TestPolicyStore(t *testing.T, s store.PolicyStore) {
	ctx := context.Background()

	t.Run("Create and GetByID", func(t *testing.T) {
		dur := 2 * time.Hour
		policy := &model.Policy{
			Name:        "Test Policy",
			RequestType: "pol-test-" + uuid.NewString(),
			Stages: []model.ApprovalStage{
				{
					Index:               0,
					Name:                "Finance Review",
					RequiredApprovals:   2,
					RejectionPolicy:     model.RejectionPolicyThreshold,
					AllowedCheckerRoles: json.RawMessage(`["admin","manager"]`),
				},
			},
			IdentityFields:     []string{"account_id"},
			AutoExpireDuration: &dur,
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
		if len(got.Stages) != 1 {
			t.Fatalf("Stages length = %d, want 1", len(got.Stages))
		}
		if got.Stages[0].RequiredApprovals != 2 {
			t.Errorf("Stages[0].RequiredApprovals = %d, want 2", got.Stages[0].RequiredApprovals)
		}
		if got.Stages[0].RejectionPolicy != model.RejectionPolicyThreshold {
			t.Errorf("Stages[0].RejectionPolicy = %q, want %q", got.Stages[0].RejectionPolicy, model.RejectionPolicyThreshold)
		}
		if string(got.Stages[0].AllowedCheckerRoles) != `["admin","manager"]` {
			t.Errorf("Stages[0].AllowedCheckerRoles = %s", got.Stages[0].AllowedCheckerRoles)
		}
		if got.Stages[0].Name != "Finance Review" {
			t.Errorf("Stages[0].Name = %q, want %q", got.Stages[0].Name, "Finance Review")
		}
		if len(got.IdentityFields) != 1 || got.IdentityFields[0] != "account_id" {
			t.Errorf("IdentityFields = %v", got.IdentityFields)
		}
		if got.AutoExpireDuration == nil || *got.AutoExpireDuration != 2*time.Hour {
			t.Errorf("AutoExpireDuration = %v, want 2h", got.AutoExpireDuration)
		}
	})

	t.Run("Multi-stage roundtrip", func(t *testing.T) {
		maxCheckers := 5
		policy := &model.Policy{
			Name:        "Multi-Stage Policy",
			RequestType: "multi-stage-" + uuid.NewString(),
			Stages: []model.ApprovalStage{
				{Index: 0, Name: "Stage 1", RequiredApprovals: 1, RejectionPolicy: model.RejectionPolicyAny},
				{Index: 1, Name: "Stage 2", RequiredApprovals: 2, RejectionPolicy: model.RejectionPolicyThreshold, MaxCheckers: &maxCheckers},
			},
		}
		if err := s.Create(ctx, policy); err != nil {
			t.Fatalf("Create: %v", err)
		}

		got, err := s.GetByID(ctx, policy.ID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if len(got.Stages) != 2 {
			t.Fatalf("Stages length = %d, want 2", len(got.Stages))
		}
		if got.Stages[1].Name != "Stage 2" {
			t.Errorf("Stages[1].Name = %q, want %q", got.Stages[1].Name, "Stage 2")
		}
		if got.Stages[1].RequiredApprovals != 2 {
			t.Errorf("Stages[1].RequiredApprovals = %d, want 2", got.Stages[1].RequiredApprovals)
		}
		if got.Stages[1].MaxCheckers == nil || *got.Stages[1].MaxCheckers != 5 {
			t.Errorf("Stages[1].MaxCheckers = %v, want 5", got.Stages[1].MaxCheckers)
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
		s.Create(ctx, &model.Policy{
			Name:        "P",
			RequestType: rt,
			Stages: []model.ApprovalStage{
				{Index: 0, RequiredApprovals: 1, RejectionPolicy: model.RejectionPolicyAny},
			},
		})

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
		policy := &model.Policy{
			Name:        "Original",
			RequestType: rt,
			Stages: []model.ApprovalStage{
				{Index: 0, RequiredApprovals: 1, RejectionPolicy: model.RejectionPolicyAny},
			},
		}
		s.Create(ctx, policy)

		policy.Name = "Updated"
		policy.Stages[0].RequiredApprovals = 5
		if err := s.Update(ctx, policy); err != nil {
			t.Fatalf("Update: %v", err)
		}

		got, _ := s.GetByID(ctx, policy.ID)
		if got.Name != "Updated" {
			t.Errorf("Name = %q, want %q", got.Name, "Updated")
		}
		if got.Stages[0].RequiredApprovals != 5 {
			t.Errorf("Stages[0].RequiredApprovals = %d, want 5", got.Stages[0].RequiredApprovals)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		rt := "del-" + uuid.NewString()
		policy := &model.Policy{
			Name:        "ToDelete",
			RequestType: rt,
			Stages: []model.ApprovalStage{
				{Index: 0, RequiredApprovals: 1, RejectionPolicy: model.RejectionPolicyAny},
			},
		}
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

// TestOperatorStore exercises every method of store.OperatorStore.
func TestOperatorStore(t *testing.T, s store.OperatorStore) {
	ctx := context.Background()

	t.Run("Create and GetByID", func(t *testing.T) {
		op := &model.Operator{
			Username:           "op-" + uuid.NewString()[:8],
			PasswordHash:       "$2a$10$fakehash",
			DisplayName:        "Test Operator",
			MustChangePassword: false,
		}
		if err := s.Create(ctx, op); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if op.ID == uuid.Nil {
			t.Fatal("expected ID to be assigned")
		}

		got, err := s.GetByID(ctx, op.ID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got == nil {
			t.Fatal("expected non-nil result")
		}
		if got.Username != op.Username {
			t.Errorf("Username = %q, want %q", got.Username, op.Username)
		}
		if got.DisplayName != "Test Operator" {
			t.Errorf("DisplayName = %q, want %q", got.DisplayName, "Test Operator")
		}
		if got.PasswordHash != "$2a$10$fakehash" {
			t.Errorf("PasswordHash = %q, want %q", got.PasswordHash, "$2a$10$fakehash")
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

	t.Run("GetByUsername", func(t *testing.T) {
		username := "user-" + uuid.NewString()[:8]
		op := &model.Operator{
			Username:     username,
			PasswordHash: "$2a$10$fakehash",
			DisplayName:  "By Username",
		}
		s.Create(ctx, op)

		got, err := s.GetByUsername(ctx, username)
		if err != nil {
			t.Fatalf("GetByUsername: %v", err)
		}
		if got == nil || got.Username != username {
			t.Fatal("expected to find operator by username")
		}
	})

	t.Run("GetByUsername not found", func(t *testing.T) {
		got, err := s.GetByUsername(ctx, "nonexistent-user")
		if err != nil {
			t.Fatalf("GetByUsername: %v", err)
		}
		if got != nil {
			t.Fatal("expected nil for non-existent username")
		}
	})

	t.Run("List", func(t *testing.T) {
		list, err := s.List(ctx)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(list) == 0 {
			t.Error("expected at least one operator")
		}
	})

	t.Run("Update", func(t *testing.T) {
		op := &model.Operator{
			Username:           "upd-" + uuid.NewString()[:8],
			PasswordHash:       "$2a$10$original",
			DisplayName:        "Original",
			MustChangePassword: true,
		}
		s.Create(ctx, op)

		op.DisplayName = "Updated"
		op.PasswordHash = "$2a$10$updated"
		op.MustChangePassword = false
		if err := s.Update(ctx, op); err != nil {
			t.Fatalf("Update: %v", err)
		}

		got, _ := s.GetByID(ctx, op.ID)
		if got.DisplayName != "Updated" {
			t.Errorf("DisplayName = %q, want %q", got.DisplayName, "Updated")
		}
		if got.PasswordHash != "$2a$10$updated" {
			t.Errorf("PasswordHash = %q, want %q", got.PasswordHash, "$2a$10$updated")
		}
		if got.MustChangePassword {
			t.Error("expected MustChangePassword=false after update")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		op := &model.Operator{
			Username:     "del-" + uuid.NewString()[:8],
			PasswordHash: "$2a$10$fakehash",
			DisplayName:  "ToDelete",
		}
		s.Create(ctx, op)

		if err := s.Delete(ctx, op.ID); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		got, _ := s.GetByID(ctx, op.ID)
		if got != nil {
			t.Error("expected nil after delete")
		}
	})

	t.Run("Count", func(t *testing.T) {
		before, err := s.Count(ctx)
		if err != nil {
			t.Fatalf("Count before: %v", err)
		}

		s.Create(ctx, &model.Operator{
			Username:     "cnt-" + uuid.NewString()[:8],
			PasswordHash: "$2a$10$fakehash",
			DisplayName:  "Counter",
		})

		after, err := s.Count(ctx)
		if err != nil {
			t.Fatalf("Count after: %v", err)
		}
		if after != before+1 {
			t.Errorf("Count after = %d, want %d", after, before+1)
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
