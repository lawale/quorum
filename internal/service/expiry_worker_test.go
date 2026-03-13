package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/testutil"
)

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

	worker := NewExpiryWorker(requests, audits, time.Minute)
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

	worker := NewExpiryWorker(requests, audits, time.Minute)
	worker.processExpired(context.Background())

	if auditAction != "expired" {
		t.Errorf("audit action = %q, want %q", auditAction, "expired")
	}
	if auditActor != "system" {
		t.Errorf("audit actor = %q, want %q", auditActor, "system")
	}
}

func TestExpiryWorker_ProcessExpired_CallsOnExpire(t *testing.T) {
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

	worker := NewExpiryWorker(requests, audits, time.Minute)

	called := false
	var expiredStatus model.RequestStatus
	worker.SetOnExpire(func(ctx context.Context, r *model.Request, approvals []model.Approval) {
		called = true
		expiredStatus = r.Status
	})
	worker.processExpired(context.Background())

	if !called {
		t.Error("expected onExpire callback to be called")
	}
	if expiredStatus != model.StatusExpired {
		t.Errorf("expired request status = %v, want expired", expiredStatus)
	}
}

func TestExpiryWorker_ProcessExpired_NoExpired(t *testing.T) {
	requests := &testutil.MockRequestStore{
		ListExpiredFunc: func(ctx context.Context) ([]model.Request, error) {
			return nil, nil
		},
	}
	audits := &testutil.MockAuditStore{}

	worker := NewExpiryWorker(requests, audits, time.Minute)
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

	worker := NewExpiryWorker(requests, audits, time.Minute)
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

	expiredIDs := make(map[uuid.UUID]bool)
	worker := NewExpiryWorker(requests, audits, time.Minute)
	worker.SetOnExpire(func(ctx context.Context, r *model.Request, approvals []model.Approval) {
		expiredIDs[r.ID] = true
	})
	worker.processExpired(context.Background())

	// req1 should NOT have onExpire called (update failed), req2 should
	if expiredIDs[req1.ID] {
		t.Error("onExpire should not be called for req1 (update failed)")
	}
	if !expiredIDs[req2.ID] {
		t.Error("onExpire should be called for req2")
	}
}

func TestExpiryWorker_Start_CancelStops(t *testing.T) {
	requests := &testutil.MockRequestStore{
		ListExpiredFunc: func(ctx context.Context) ([]model.Request, error) {
			return nil, nil
		},
	}

	worker := NewExpiryWorker(requests, &testutil.MockAuditStore{}, 10*time.Millisecond)

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
