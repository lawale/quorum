package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/metrics"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/signing"
	"github.com/lawale/quorum/internal/store"
)

// Dispatcher manages durable webhook delivery using an outbox table.
// Instead of an in-memory channel, webhooks are persisted to a database outbox
// and delivered by a background worker that wakes on signal or heartbeat.
type Dispatcher struct {
	outbox          store.OutboxStore
	audits          store.AuditStore
	client          *http.Client
	maxRetries      int
	retryDelay      time.Duration
	retryWindow     time.Duration
	maxRetryDelay   time.Duration
	heartbeat       time.Duration
	batchSize       int
	retentionPeriod time.Duration
	signal          chan struct{}
	metrics         *metrics.Metrics
}

// SetMetrics sets the optional Prometheus metrics collector.
func (d *Dispatcher) SetMetrics(m *metrics.Metrics) {
	d.metrics = m
}

// Config holds configuration for the webhook Dispatcher.
type Config struct {
	Timeout         time.Duration
	MaxRetries      int
	RetryDelay      time.Duration
	RetryWindow     time.Duration // max duration to keep retrying from first attempt; 0 = no window limit
	MaxRetryDelay   time.Duration // cap on individual retry delay; 0 = no cap
	Heartbeat       time.Duration
	BatchSize       int
	RetentionPeriod time.Duration // how long to keep delivered entries; 0 = never clean up
}

func NewDispatcher(outbox store.OutboxStore, audits store.AuditStore, cfg Config) *Dispatcher {
	heartbeat := cfg.Heartbeat
	if heartbeat == 0 {
		heartbeat = 30 * time.Second
	}
	batchSize := cfg.BatchSize
	if batchSize == 0 {
		batchSize = 50
	}
	retryWindow := cfg.RetryWindow
	if retryWindow == 0 {
		retryWindow = 72 * time.Hour
	}
	maxRetryDelay := cfg.MaxRetryDelay
	if maxRetryDelay == 0 {
		maxRetryDelay = time.Hour
	}
	return &Dispatcher{
		outbox:          outbox,
		audits:          audits,
		client:          &http.Client{Timeout: cfg.Timeout},
		maxRetries:      cfg.MaxRetries,
		retryDelay:      cfg.RetryDelay,
		retryWindow:     retryWindow,
		maxRetryDelay:   maxRetryDelay,
		heartbeat:       heartbeat,
		batchSize:       batchSize,
		retentionPeriod: cfg.RetentionPeriod,
		signal:          make(chan struct{}, 1),
	}
}

// Enqueue builds outbox entries for a resolved request and writes them using
// the provided stores. The caller should pass tx-bound stores so that the
// outbox writes are atomic with the status update.
func (d *Dispatcher) Enqueue(ctx context.Context, outbox store.OutboxStore, webhooks store.WebhookStore, req *model.Request, approvals []model.Approval) error {
	event := string(req.Status)

	payload := model.WebhookPayload{
		Event:     event,
		Request:   *req,
		Approvals: approvals,
		Timestamp: time.Now().UTC(),
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling webhook payload: %w", err)
	}

	var entries []model.OutboxEntry

	// Find matching global webhooks
	whs, err := webhooks.ListByEventAndType(ctx, event, req.Type)
	if err != nil {
		return fmt.Errorf("listing webhooks: %w", err)
	}

	for _, wh := range whs {
		entries = append(entries, model.OutboxEntry{
			RequestID:     req.ID,
			WebhookURL:    wh.URL,
			WebhookSecret: wh.Secret,
			Payload:       payloadJSON,
			MaxRetries:    d.maxRetries,
		})
	}

	if len(entries) == 0 {
		return nil
	}

	return outbox.CreateBatch(ctx, entries)
}

// Signal wakes the delivery worker to process pending outbox entries.
// Non-blocking: if a signal is already pending, this is a no-op.
func (d *Dispatcher) Signal() {
	select {
	case d.signal <- struct{}{}:
	default: // already signaled
	}
}

// Start begins the background delivery worker.
func (d *Dispatcher) Start(ctx context.Context) {
	go d.runWorker(ctx)
	slog.Info("webhook delivery worker started", "heartbeat", d.heartbeat, "batch_size", d.batchSize)
}

func (d *Dispatcher) runWorker(ctx context.Context) {
	ticker := time.NewTicker(d.heartbeat)
	defer ticker.Stop()

	// Retention cleanup runs at 10× the heartbeat interval (or every hour, whichever is shorter)
	var cleanupTicker *time.Ticker
	if d.retentionPeriod > 0 {
		cleanupInterval := d.heartbeat * 10
		if cleanupInterval > time.Hour {
			cleanupInterval = time.Hour
		}
		cleanupTicker = time.NewTicker(cleanupInterval)
		defer cleanupTicker.Stop()
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.signal:
			d.processBatch(ctx)
		case <-ticker.C:
			d.processBatch(ctx)
		case <-d.cleanupChan(cleanupTicker):
			d.cleanupDelivered(ctx)
		}
	}
}

// cleanupChan returns the ticker channel if cleanup is configured, or a nil
// channel (which blocks forever) if not.
func (d *Dispatcher) cleanupChan(t *time.Ticker) <-chan time.Time {
	if t != nil {
		return t.C
	}
	return nil
}

func (d *Dispatcher) cleanupDelivered(ctx context.Context) {
	olderThan := time.Now().Add(-d.retentionPeriod)
	deleted, err := d.outbox.DeleteDelivered(ctx, olderThan)
	if err != nil {
		slog.Error("failed to clean up delivered outbox entries", "error", err)
		return
	}
	if deleted > 0 {
		slog.Info("cleaned up delivered outbox entries", "deleted", deleted, "older_than", olderThan)
	}
}

func (d *Dispatcher) processBatch(ctx context.Context) {
	entries, err := d.outbox.ClaimBatch(ctx, d.batchSize)
	if err != nil {
		slog.Error("failed to claim pending outbox entries", "error", err)
		return
	}

	for _, entry := range entries {
		d.deliverEntry(ctx, entry)
	}
}

func (d *Dispatcher) deliverEntry(ctx context.Context, entry model.OutboxEntry) {
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, entry.WebhookURL, bytes.NewReader(entry.Payload))
	if err != nil {
		slog.Error("failed to create webhook request", "error", err, "url", entry.WebhookURL)
		d.handleFailure(ctx, entry, fmt.Sprintf("invalid request URL: %s", err))
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Quorum/1.0")

	// HMAC signature
	if entry.WebhookSecret != "" {
		sig := signing.ComputeHMAC(entry.Payload, entry.WebhookSecret)
		req.Header.Set("X-Signature-256", "sha256="+sig)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		d.handleFailure(ctx, entry, err.Error())
		return
	}
	resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if markErr := d.outbox.MarkDelivered(ctx, entry.ID); markErr != nil {
			slog.Error("failed to mark outbox entry delivered", "error", markErr, "entry_id", entry.ID)
			return // Don't audit success — entry will be retried on next claim
		}
		slog.Info("webhook delivered", "url", entry.WebhookURL, "status", resp.StatusCode, "request_id", entry.RequestID)
		d.auditWebhook(ctx, entry.RequestID, "webhook_sent", entry.WebhookURL)
		if d.metrics != nil {
			d.metrics.WebhookDeliveriesTotal.WithLabelValues("success").Inc()
			d.metrics.WebhookDeliveryDuration.Observe(time.Since(start).Seconds())
		}
		return
	}

	d.handleFailure(ctx, entry, fmt.Sprintf("unexpected status code: %d", resp.StatusCode))
}

func (d *Dispatcher) handleFailure(ctx context.Context, entry model.OutboxEntry, errMsg string) {
	attempts := entry.Attempts + 1

	windowExpired := d.retryWindow > 0 && time.Since(entry.CreatedAt) >= d.retryWindow
	if attempts > entry.MaxRetries || windowExpired {
		if err := d.outbox.MarkFailed(ctx, entry.ID, attempts, errMsg); err != nil {
			slog.Error("failed to mark outbox entry failed", "error", err, "entry_id", entry.ID)
		}
		reason := "retries exhausted"
		if windowExpired {
			reason = "retry window expired"
		}
		d.auditWebhook(ctx, entry.RequestID, "webhook_failed", entry.WebhookURL)
		slog.Error("webhook delivery failed permanently", "url", entry.WebhookURL, "request_id", entry.RequestID, "reason", reason, "attempts", attempts, "error", errMsg)
		if d.metrics != nil {
			d.metrics.WebhookDeliveriesTotal.WithLabelValues("failure").Inc()
		}
		return
	}

	backoff := d.computeBackoff(attempts)
	nextRetry := time.Now().Add(backoff)
	if err := d.outbox.MarkRetry(ctx, entry.ID, attempts, errMsg, nextRetry); err != nil {
		slog.Error("failed to schedule outbox entry retry", "error", err, "entry_id", entry.ID)
	}
	slog.Warn("webhook delivery failed, scheduling retry", "url", entry.WebhookURL, "attempt", attempts, "next_retry", nextRetry, "backoff", backoff, "error", errMsg)
}

// computeBackoff returns an exponential backoff duration with jitter.
// Formula: base * 2^(attempts-1), capped at maxRetryDelay, with +/-20% jitter.
func (d *Dispatcher) computeBackoff(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	// Exponential: base * 2^(attempts-1), cap the shift to avoid overflow
	shift := attempts - 1
	if shift > 30 {
		shift = 30
	}
	delay := d.retryDelay * time.Duration(1<<uint(shift))

	// Cap at maxRetryDelay
	if d.maxRetryDelay > 0 && delay > d.maxRetryDelay {
		delay = d.maxRetryDelay
	}

	// Add +/-20% jitter to avoid thundering herd
	if delay > 0 {
		jitter := time.Duration(rand.Int63n(int64(delay) / 5))
		if rand.Intn(2) == 0 {
			delay += jitter
		} else {
			delay -= jitter
		}
	}

	return delay
}

func (d *Dispatcher) auditWebhook(ctx context.Context, requestID uuid.UUID, action string, url string) {
	details, _ := json.Marshal(map[string]string{"url": url})
	log := &model.AuditLog{
		RequestID: requestID,
		Action:    action,
		ActorID:   "system",
		Details:   details,
	}
	if err := d.audits.Create(ctx, log); err != nil {
		slog.Error("failed to audit webhook", "error", err, "request_id", requestID)
	}
}
