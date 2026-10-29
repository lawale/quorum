package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/store"
	"github.com/lawale/quorum/internal/testutil"
)

// noApprovals is an approval store for requests nobody has voted on.
func noApprovals() *testutil.MockApprovalStore {
	return &testutil.MockApprovalStore{
		ListByRequestIDFunc: func(ctx context.Context, requestID uuid.UUID) ([]model.Approval, error) { return nil, nil },
	}
}

func TestExpiryWorker_ProcessExpired_UpdatesStatus(t *testing.T) {
	req1 := testutil.NewRequest(func(r *model.Request) { r.ID = uuid.New() })
	req2 := testutil.NewRequest(func(r *model.Request) { r.ID = uuid.New() })

	updatedIDs := make(map[uuid.UUID]bool)
	requests := &testutil.MockRequestStore{
		ListExpiredFunc: func(ctx context.Context) ([]model.Request, error) {
			return []model.Request{*req1, *req2}, nil
		},
		UpdateStatusFunc: func(ctx context.Context, id uuid.UUID, status model.RequestStatus) error {
			if status != model.StatusExpired {
				t.Errorf("expected StatusExpired, got %v", status)
			}
			updatedIDs[id] = true
			return nil
		},
	}
	audits := &testutil.MockAuditStore{
		CreateFunc: func(ctx context.Context, log *model.AuditLog) error { return nil },
	}

	worker := NewExpiryWorker(requests, noApprovals(), audits, time.Minute)
	worker.processExpired(context.Background())

	if !updatedIDs[req1.ID] || !updatedIDs[req2.ID] {
		t.Error("expected both requests to be updated to expired")
	}
}

func TestExpiryWorker_ProcessExpired_CreatesAuditLogs(t *testing.T) {
	req := testutil.NewRequest()
	requests := &testutil.MockRequestStore{
		ListExpiredFunc: func(ctx context.Context) ([]model.Request, error) {
			return []model.Request{*req}, nil
		},
		UpdateStatusFunc: func(ctx context.Context, id uuid.UUID, status model.RequestStatus) error {
			return nil
		},
	}

	var auditAction string
	var auditActor string
	audits := &testutil.MockAuditStore{
		CreateFunc: func(ctx context.Context, log *model.AuditLog) error {
			auditAction = log.Action
			auditActor = log.ActorID
			return nil
		},
	}

	worker := NewExpiryWorker(requests, noApprovals(), audits, time.Minute)
	worker.processExpired(context.Background())

	if auditAction != "expired" {
		t.Errorf("audit action = %q, want %q", auditAction, "expired")
	}
	if auditActor != "system" {
		t.Errorf("audit actor = %q, want %q", auditActor, "system")
	}
}

func TestExpiryWorker_ProcessExpired_CallsWebhookDispatch(t *testing.T) {
	req := testutil.NewRequest()
	requests := &testutil.MockRequestStore{
		ListExpiredFunc: func(ctx context.Context) ([]model.Request, error) {
			return []model.Request{*req}, nil
		},
		UpdateStatusFunc: func(ctx context.Context, id uuid.UUID, status model.RequestStatus) error {
			return nil
		},
	}
	audits := &testutil.MockAuditStore{
		CreateFunc: func(ctx context.Context, log *model.AuditLog) error { return nil },
	}

	worker := NewExpiryWorker(requests, noApprovals(), audits, time.Minute)

	sink := &fakeSink{}
	worker.SetWebhookDispatch(passThrough(&store.Stores{Requests: requests, Approvals: noApprovals()}), sink)
	worker.processExpired(context.Background())

	if len(sink.events) != 1 || sink.events[0].Name != "expired" || sink.events[0].Request.Status != model.StatusExpired {
		t.Errorf("events = %+v, want one expired event", sink.events)
	}
	if sink.signals != 1 {
		t.Errorf("signals = %d, want 1", sink.signals)
	}
}

func TestExpiryWorker_ProcessExpired_NoExpired(t *testing.T) {
	requests := &testutil.MockRequestStore{
		ListExpiredFunc: func(ctx context.Context) ([]model.Request, error) {
			return nil, nil
		},
	}
	audits := &testutil.MockAuditStore{}

	worker := NewExpiryWorker(requests, noApprovals(), audits, time.Minute)
	// Should not panic
	worker.processExpired(context.Background())
}

func TestExpiryWorker_ProcessExpired_ListError(t *testing.T) {
	requests := &testutil.MockRequestStore{
		ListExpiredFunc: func(ctx context.Context) ([]model.Request, error) {
			return nil, errors.New("db error")
		},
	}
	audits := &testutil.MockAuditStore{}

	worker := NewExpiryWorker(requests, noApprovals(), audits, time.Minute)
	// Should not panic, just log
	worker.processExpired(context.Background())
}

func TestExpiryWorker_ProcessExpired_UpdateStatusError(t *testing.T) {
	req1 := testutil.NewRequest(func(r *model.Request) { r.ID = uuid.MustParse("00000000-0000-0000-0000-000000000001") })
	req2 := testutil.NewRequest(func(r *model.Request) { r.ID = uuid.MustParse("00000000-0000-0000-0000-000000000002") })

	requests := &testutil.MockRequestStore{
		ListExpiredFunc: func(ctx context.Context) ([]model.Request, error) {
			return []model.Request{*req1, *req2}, nil
		},
		UpdateStatusFunc: func(ctx context.Context, id uuid.UUID, status model.RequestStatus) error {
			if id == req1.ID {
				return errors.New("update failed")
			}
			return nil
		},
	}
	audits := &testutil.MockAuditStore{
		CreateFunc: func(ctx context.Context, log *model.AuditLog) error { return nil },
	}

	worker := NewExpiryWorker(requests, noApprovals(), audits, time.Minute)
	sink := &fakeSink{}
	worker.SetWebhookDispatch(passThrough(&store.Stores{Requests: requests, Approvals: noApprovals()}), sink)
	worker.processExpired(context.Background())

	// req1's update failed inside the tx, so only req2 is announced
	if len(sink.events) != 1 || sink.events[0].Request.ID != req2.ID {
		t.Errorf("events = %+v, want only req2 expired", sink.events)
	}
}

func TestExpiryWorker_Start_CancelStops(t *testing.T) {
	requests := &testutil.MockRequestStore{
		ListExpiredFunc: func(ctx context.Context) ([]model.Request, error) {
			return nil, nil
		},
	}

	worker := NewExpiryWorker(requests, noApprovals(), &testutil.MockAuditStore{}, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		worker.Start(ctx)
	}()

	// Let it tick a couple times
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Should complete quickly after cancel
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - worker stopped
	case <-time.After(2 * time.Second):
		t.Error("worker did not stop after context cancellation")
	}
}
