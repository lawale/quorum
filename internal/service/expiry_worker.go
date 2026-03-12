package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/wale/quorum/internal/model"
	"github.com/wale/quorum/internal/store"
)

type ExpiryWorker struct {
	requests      store.RequestStore
	audits        store.AuditStore
	checkInterval time.Duration
	onExpire      func(ctx context.Context, req *model.Request, approvals []model.Approval)
}

func NewExpiryWorker(requests store.RequestStore, audits store.AuditStore, checkInterval time.Duration) *ExpiryWorker {
	return &ExpiryWorker{
		requests:      requests,
		audits:        audits,
		checkInterval: checkInterval,
	}
}

func (w *ExpiryWorker) SetOnExpire(fn func(ctx context.Context, req *model.Request, approvals []model.Approval)) {
	w.onExpire = fn
}

func (w *ExpiryWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.checkInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.processExpired(ctx)
			}
		}
	}()
	slog.Info("expiry worker started", "interval", w.checkInterval)
}

func (w *ExpiryWorker) processExpired(ctx context.Context) {
	expired, err := w.requests.ListExpired(ctx)
	if err != nil {
		slog.Error("failed to list expired requests", "error", err)
		return
	}

	for _, req := range expired {
		if err := w.requests.UpdateStatus(ctx, req.ID, model.StatusExpired); err != nil {
			slog.Error("failed to expire request", "error", err, "request_id", req.ID)
			continue
		}

		// Audit
		log := &model.AuditLog{
			RequestID: req.ID,
			Action:    "expired",
			ActorID:   "system",
		}
		if err := w.audits.Create(ctx, log); err != nil {
			slog.Error("failed to audit expiry", "error", err, "request_id", req.ID)
		}

		slog.Info("request expired", "request_id", req.ID)

		if w.onExpire != nil {
			req.Status = model.StatusExpired
			w.onExpire(ctx, &req, nil)
		}
	}
}
