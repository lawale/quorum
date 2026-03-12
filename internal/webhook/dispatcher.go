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
	"github.com/wale/maker-checker/internal/model"
	"github.com/wale/maker-checker/internal/store"
)

type Dispatcher struct {
	webhooks   store.WebhookStore
	audits     store.AuditStore
	client     *http.Client
	maxRetries int
	retryDelay time.Duration
	queue      chan deliveryJob
}

type deliveryJob struct {
	webhook model.Webhook
	payload model.WebhookPayload
	request model.Request
}

func NewDispatcher(webhooks store.WebhookStore, audits store.AuditStore, timeout time.Duration, maxRetries int, retryDelay time.Duration) *Dispatcher {
	return &Dispatcher{
		webhooks:   webhooks,
		audits:     audits,
		client:     &http.Client{Timeout: timeout},
		maxRetries: maxRetries,
		retryDelay: retryDelay,
		queue:      make(chan deliveryJob, 100),
	}
}

// Start begins processing webhook deliveries in the background.
func (d *Dispatcher) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case job := <-d.queue:
				d.deliver(ctx, job)
			}
		}
	}()
}

// Dispatch queues webhook delivery for a resolved request.
func (d *Dispatcher) Dispatch(ctx context.Context, req *model.Request, approvals []model.Approval) {
	event := string(req.Status)

	payload := model.WebhookPayload{
		Event:     event,
		Request:   *req,
		Approvals: approvals,
		Timestamp: time.Now().UTC(),
	}

	// Find matching global webhooks
	webhooks, err := d.webhooks.ListByEventAndType(ctx, event, req.Type)
	if err != nil {
		slog.Error("failed to find matching webhooks", "error", err, "request_id", req.ID)
		return
	}

	for _, wh := range webhooks {
		d.queue <- deliveryJob{webhook: wh, payload: payload, request: *req}
	}

	// Also deliver to the request's callback URL if set
	if req.CallbackURL != nil && *req.CallbackURL != "" {
		callbackWebhook := model.Webhook{
			URL:    *req.CallbackURL,
			Secret: "", // No HMAC for per-request callbacks
		}
		d.queue <- deliveryJob{webhook: callbackWebhook, payload: payload, request: *req}
	}
}

func (d *Dispatcher) deliver(ctx context.Context, job deliveryJob) {
	body, err := json.Marshal(job.payload)
	if err != nil {
		slog.Error("failed to marshal webhook payload", "error", err)
		return
	}

	var lastErr error
	for attempt := 0; attempt <= d.maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(d.retryDelay * time.Duration(attempt))
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, job.webhook.URL, bytes.NewReader(body))
		if err != nil {
			slog.Error("failed to create webhook request", "error", err, "url", job.webhook.URL)
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "MakerChecker/1.0")

		// HMAC signature
		if job.webhook.Secret != "" {
			sig := computeHMAC(body, job.webhook.Secret)
			req.Header.Set("X-Signature-256", "sha256="+sig)
		}

		resp, err := d.client.Do(req)
		if err != nil {
			lastErr = err
			slog.Warn("webhook delivery failed", "url", job.webhook.URL, "attempt", attempt+1, "error", err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			slog.Info("webhook delivered", "url", job.webhook.URL, "status", resp.StatusCode, "request_id", job.request.ID)
			d.auditWebhook(ctx, job.request.ID, "webhook_sent", job.webhook.URL)
			return
		}

		lastErr = fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		slog.Warn("webhook delivery returned non-2xx", "url", job.webhook.URL, "status", resp.StatusCode, "attempt", attempt+1)
	}

	slog.Error("webhook delivery exhausted retries", "url", job.webhook.URL, "request_id", job.request.ID, "error", lastErr)
	d.auditWebhook(ctx, job.request.ID, "webhook_failed", job.webhook.URL)
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
