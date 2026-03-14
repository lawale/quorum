package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/metrics"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/store"
)

// Dispatcher manages durable webhook delivery using an outbox table.
// Instead of an in-memory channel, webhooks are persisted to a database outbox
// and delivered by a background worker that wakes on signal or heartbeat.
type Dispatcher struct {
	outbox         store.OutboxStore
	audits         store.AuditStore
	client         *http.Client
	callbackSecret string
	maxRetries     int
	retryDelay     time.Duration
	heartbeat      time.Duration
	batchSize      int
	signal         chan struct{}
	metrics        *metrics.Metrics
}

// SetMetrics sets the optional Prometheus metrics collector.
func (d *Dispatcher) SetMetrics(m *metrics.Metrics) {
	d.metrics = m
}

// Config holds configuration for the webhook Dispatcher.
type Config struct {
	Timeout        time.Duration
	MaxRetries     int
	RetryDelay     time.Duration
	CallbackSecret string
	Heartbeat      time.Duration
	BatchSize      int
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
	return &Dispatcher{
		outbox:         outbox,
		audits:         audits,
		client:         &http.Client{Timeout: cfg.Timeout},
		callbackSecret: cfg.CallbackSecret,
		maxRetries:     cfg.MaxRetries,
		retryDelay:     cfg.RetryDelay,
		heartbeat:      heartbeat,
		batchSize:      batchSize,
		signal:         make(chan struct{}, 1),
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

	// Also enqueue delivery to the request's callback URL if set
	if req.CallbackURL != nil && *req.CallbackURL != "" {
		entries = append(entries, model.OutboxEntry{
			RequestID:     req.ID,
			WebhookURL:    *req.CallbackURL,
			WebhookSecret: d.callbackSecret,
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

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.signal:
			d.processBatch(ctx)
		case <-ticker.C:
			d.processBatch(ctx)
		}
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
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Quorum/1.0")

	// HMAC signature
	if entry.WebhookSecret != "" {
		sig := computeHMAC(entry.Payload, entry.WebhookSecret)
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

	if attempts > entry.MaxRetries {
		if err := d.outbox.MarkFailed(ctx, entry.ID, attempts, errMsg); err != nil {
			slog.Error("failed to mark outbox entry failed", "error", err, "entry_id", entry.ID)
		}
		d.auditWebhook(ctx, entry.RequestID, "webhook_failed", entry.WebhookURL)
		slog.Error("webhook delivery exhausted retries", "url", entry.WebhookURL, "request_id", entry.RequestID, "error", errMsg)
		if d.metrics != nil {
			d.metrics.WebhookDeliveriesTotal.WithLabelValues("failure").Inc()
		}
		return
	}

	nextRetry := time.Now().Add(d.retryDelay * time.Duration(attempts))
	if err := d.outbox.MarkRetry(ctx, entry.ID, attempts, errMsg, nextRetry); err != nil {
		slog.Error("failed to schedule outbox entry retry", "error", err, "entry_id", entry.ID)
	}
	slog.Warn("webhook delivery failed, scheduling retry", "url", entry.WebhookURL, "attempt", attempts, "next_retry", nextRetry, "error", errMsg)
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

func computeHMAC(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
