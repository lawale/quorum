package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/auth"
	"github.com/lawale/quorum/internal/metrics"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/store"
)

type ExpiryWorker struct {
	requests      store.RequestStore
	audits        store.AuditStore
	checkInterval time.Duration
	dispatch
	signalSSE func(requestID uuid.UUID)
	metrics   *metrics.Metrics
}

// SetSSESignal configures the callback invoked after a request expires
// to notify connected SSE clients.
func (w *ExpiryWorker) SetSSESignal(signal func(requestID uuid.UUID)) {
	w.signalSSE = signal
}

// SetMetrics sets the optional Prometheus metrics collector.
func (w *ExpiryWorker) SetMetrics(m *metrics.Metrics) {
	w.metrics = m
}

func NewExpiryWorker(requests store.RequestStore, approvals store.ApprovalStore, audits store.AuditStore, checkInterval time.Duration) *ExpiryWorker {
	return &ExpiryWorker{
		requests:      requests,
		audits:        audits,
		checkInterval: checkInterval,
		dispatch: dispatch{
			runInTx: passThrough(&store.Stores{Requests: requests, Approvals: approvals, Audits: audits}),
		},
	}
}

// SetWebhookDispatch makes expiry transactional and announces it to sink on
// the same transaction. runInTx is the database's transaction runner.
func (w *ExpiryWorker) SetWebhookDispatch(runInTx txRunner, sink EventSink) {
	if runInTx != nil {
		w.runInTx = runInTx
	}
	w.sink = sink
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
		if w.metrics != nil {
			w.metrics.ExpiryErrorsTotal.Inc()
		}
		return
	}

	for _, req := range expired {
		// Inject the request's tenant into context so webhook lookup is scoped correctly
		tenantCtx := auth.WithTenantID(ctx, req.TenantID)

		err := w.transact(tenantCtx, func(tx *store.Stores, emit emitFunc) error {
			if err := tx.Requests.UpdateStatus(tenantCtx, req.ID, model.StatusExpired); err != nil {
				return err
			}
			approvals, err := tx.Approvals.ListByRequestID(tenantCtx, req.ID)
			if err != nil {
				return fmt.Errorf("loading approvals for expiry event: %w", err)
			}
			return emit(model.StatusChanged(&req, model.StatusExpired, approvals))
		})
		if err != nil {
			if errors.Is(err, store.ErrStatusConflict) {
				slog.Debug("request already resolved, skipping expiry", "request_id", req.ID)
				continue
			}
			slog.Error("failed to expire request", "error", err, "request_id", req.ID)
			if w.metrics != nil {
				w.metrics.ExpiryErrorsTotal.Inc()
			}
			continue
		}
		if w.signalSSE != nil {
			w.signalSSE(req.ID)
		}

		// Audit
		log := &model.AuditLog{
			TenantID:  req.TenantID,
			RequestID: req.ID,
			Action:    "expired",
			ActorID:   "system",
		}
		if err := w.audits.Create(tenantCtx, log); err != nil {
			slog.Error("failed to audit expiry", "error", err, "request_id", req.ID)
			if w.metrics != nil {
				w.metrics.ExpiryErrorsTotal.Inc()
			}
		}

		if w.metrics != nil {
			w.metrics.RequestsTotal.WithLabelValues("expired").Inc()
			w.metrics.PendingRequestsGauge.Dec()
			w.metrics.RequestResolutionDuration.Observe(time.Since(req.CreatedAt).Seconds())
		}

		slog.Info("request expired", "request_id", req.ID)
	}
}
