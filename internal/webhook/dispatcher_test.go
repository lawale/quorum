package webhook

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/signing"
	"github.com/lawale/quorum/internal/testutil"
)

func TestDispatcher_Enqueue_WritesMatchingWebhooks(t *testing.T) {
	wh1 := testutil.NewWebhook()
	wh2 := testutil.NewWebhook()
	webhooks := &testutil.MockWebhookStore{
		ListByEventAndTypeFunc: func(ctx context.Context, event, rt string) ([]model.Webhook, error) {
			return []model.Webhook{*wh1, *wh2}, nil
		},
	}

	var createdEntries []model.OutboxEntry
	outbox := &testutil.MockOutboxStore{
		CreateBatchFunc: func(ctx context.Context, entries []model.OutboxEntry) error {
			createdEntries = entries
			return nil
		},
	}

	dispatcher := NewDispatcher(&testutil.MockOutboxStore{}, &testutil.MockAuditStore{}, Config{
		Timeout: 5 * time.Second,
	})

	req := testutil.NewRequest(func(r *model.Request) { r.Status = model.StatusApproved })

	err := dispatcher.Enqueue(context.Background(), outbox, webhooks, req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(createdEntries) != 2 {
		t.Errorf("expected 2 outbox entries, got %d", len(createdEntries))
	}
}

func TestDispatcher_Enqueue_NoMatchingWebhooks(t *testing.T) {
	webhooks := &testutil.MockWebhookStore{
		ListByEventAndTypeFunc: func(ctx context.Context, event, rt string) ([]model.Webhook, error) {
			return nil, nil
		},
	}

	outbox := &testutil.MockOutboxStore{
		CreateBatchFunc: func(ctx context.Context, entries []model.OutboxEntry) error {
			t.Fatal("CreateBatch should not be called when there are no entries")
			return nil
		},
	}

	dispatcher := NewDispatcher(&testutil.MockOutboxStore{}, &testutil.MockAuditStore{}, Config{
		Timeout: 5 * time.Second,
	})

	req := testutil.NewRequest(func(r *model.Request) {
		r.Status = model.StatusApproved
	})

	err := dispatcher.Enqueue(context.Background(), outbox, webhooks, req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDispatcher_DeliverEntry_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	var auditAction string
	audits := &testutil.MockAuditStore{
		CreateFunc: func(ctx context.Context, log *model.AuditLog) error {
			auditAction = log.Action
			return nil
		},
	}

	outbox := &testutil.MockOutboxStore{
		MarkDeliveredFunc: func(ctx context.Context, id uuid.UUID) error {
			return nil
		},
	}

	dispatcher := NewDispatcher(outbox, audits, Config{Timeout: 5 * time.Second})

	entry := model.OutboxEntry{
		ID:            uuid.New(),
		RequestID:     uuid.New(),
		WebhookURL:    server.URL,
		WebhookSecret: "test-secret",
		Payload:       []byte(`{"event":"approved"}`),
		MaxRetries:    3,
	}

	dispatcher.deliverEntry(context.Background(), entry)

	if auditAction != "webhook_sent" {
		t.Errorf("audit action = %q, want %q", auditAction, "webhook_sent")
	}
}

func TestDispatcher_DeliverEntry_FailureSchedulesRetry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	var retryAttempts int
	outbox := &testutil.MockOutboxStore{
		MarkRetryFunc: func(ctx context.Context, id uuid.UUID, attempts int, lastError string, nextRetryAt time.Time) error {
			retryAttempts = attempts
			return nil
		},
	}

	dispatcher := NewDispatcher(outbox, &testutil.MockAuditStore{}, Config{
		Timeout:    5 * time.Second,
		MaxRetries: 3,
		RetryDelay: time.Millisecond,
	})

	entry := model.OutboxEntry{
		ID:         uuid.New(),
		RequestID:  uuid.New(),
		WebhookURL: server.URL,
		Payload:    []byte(`{"event":"approved"}`),
		Attempts:   0,
		MaxRetries: 3,
		CreatedAt:  time.Now(),
	}

	dispatcher.deliverEntry(context.Background(), entry)

	if retryAttempts != 1 {
		t.Errorf("retry attempts = %d, want 1", retryAttempts)
	}
}

func TestDispatcher_DeliverEntry_ExhaustsRetries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError) // Always fail
	}))
	defer server.Close()

	var auditAction string
	audits := &testutil.MockAuditStore{
		CreateFunc: func(ctx context.Context, log *model.AuditLog) error {
			auditAction = log.Action
			return nil
		},
	}

	outbox := &testutil.MockOutboxStore{
		MarkFailedFunc: func(ctx context.Context, id uuid.UUID, attempts int, lastError string) error {
			return nil
		},
	}

	dispatcher := NewDispatcher(outbox, audits, Config{
		Timeout:    5 * time.Second,
		MaxRetries: 1,
		RetryDelay: time.Millisecond,
	})

	entry := model.OutboxEntry{
		ID:         uuid.New(),
		RequestID:  uuid.New(),
		WebhookURL: server.URL,
		Payload:    []byte(`{"event":"approved"}`),
		Attempts:   1, // Already attempted once, max is 1
		MaxRetries: 1,
	}

	dispatcher.deliverEntry(context.Background(), entry)

	if auditAction != "webhook_failed" {
		t.Errorf("audit action = %q, want %q", auditAction, "webhook_failed")
	}
}

func TestDispatcher_DeliverEntry_HMAC_Signature(t *testing.T) {
	var receivedSig string
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get("X-Signature-256")
		buf := make([]byte, r.ContentLength)
		r.Body.Read(buf)
		receivedBody = buf
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	outbox := &testutil.MockOutboxStore{
		MarkDeliveredFunc: func(ctx context.Context, id uuid.UUID) error { return nil },
	}
	audits := &testutil.MockAuditStore{
		CreateFunc: func(ctx context.Context, log *model.AuditLog) error { return nil },
	}

	secret := "my-webhook-secret"
	dispatcher := NewDispatcher(outbox, audits, Config{Timeout: 5 * time.Second})

	payload := []byte(`{"event":"approved","request_id":"00000000-0000-0000-0000-000000000001"}`)
	entry := model.OutboxEntry{
		ID:            uuid.New(),
		RequestID:     uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		WebhookURL:    server.URL,
		WebhookSecret: secret,
		Payload:       payload,
		MaxRetries:    3,
	}

	dispatcher.deliverEntry(context.Background(), entry)

	if receivedSig == "" {
		t.Fatal("expected X-Signature-256 header")
	}
	// Verify it starts with "sha256="
	if len(receivedSig) < 7 || receivedSig[:7] != "sha256=" {
		t.Errorf("signature should start with sha256=, got %q", receivedSig)
	}

	// Verify the HMAC matches the actual received body
	expectedSig := "sha256=" + signing.ComputeHMAC(receivedBody, secret)
	if receivedSig != expectedSig {
		t.Errorf("signature mismatch: got %q, want %q", receivedSig, expectedSig)
	}
}

func TestDispatcher_DeliverEntry_NoHMAC_WhenNoSecret(t *testing.T) {
	var receivedSig string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get("X-Signature-256")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	outbox := &testutil.MockOutboxStore{
		MarkDeliveredFunc: func(ctx context.Context, id uuid.UUID) error { return nil },
	}
	audits := &testutil.MockAuditStore{
		CreateFunc: func(ctx context.Context, log *model.AuditLog) error { return nil },
	}

	dispatcher := NewDispatcher(outbox, audits, Config{Timeout: 5 * time.Second})

	entry := model.OutboxEntry{
		ID:            uuid.New(),
		RequestID:     uuid.New(),
		WebhookURL:    server.URL,
		WebhookSecret: "", // No secret
		Payload:       []byte(`{"event":"approved"}`),
		MaxRetries:    3,
	}

	dispatcher.deliverEntry(context.Background(), entry)

	if receivedSig != "" {
		t.Errorf("expected no X-Signature-256 header when secret is empty, got %q", receivedSig)
	}
}

func TestDispatcher_Signal_NonBlocking(t *testing.T) {
	dispatcher := NewDispatcher(&testutil.MockOutboxStore{}, &testutil.MockAuditStore{}, Config{
		Timeout: 5 * time.Second,
	})

	// Signal twice — should not block
	dispatcher.Signal()
	dispatcher.Signal()

	// Channel should have exactly one signal
	select {
	case <-dispatcher.signal:
		// OK
	default:
		t.Error("expected signal in channel")
	}

	select {
	case <-dispatcher.signal:
		t.Error("expected at most one signal in channel")
	default:
		// OK — channel is empty after draining
	}
}

func TestComputeBackoff_ExponentialGrowth(t *testing.T) {
	d := &Dispatcher{
		retryDelay:    30 * time.Second,
		maxRetryDelay: time.Hour,
	}

	// Check that delays roughly double (within jitter tolerance)
	prev := time.Duration(0)
	for attempt := 1; attempt <= 6; attempt++ {
		delay := d.computeBackoff(attempt)
		if attempt > 1 && delay < prev/2 {
			t.Errorf("attempt %d: delay %v should be roughly 2x previous %v", attempt, delay, prev)
		}
		prev = delay
	}
}

func TestComputeBackoff_CapsAtMaxDelay(t *testing.T) {
	d := &Dispatcher{
		retryDelay:    30 * time.Second,
		maxRetryDelay: 5 * time.Minute,
	}

	// With base=30s and cap=5m, after a few attempts the delay should cap
	for attempt := 1; attempt <= 20; attempt++ {
		delay := d.computeBackoff(attempt)
		// With 20% jitter, the max is 5m + 20% = 6m
		maxWithJitter := d.maxRetryDelay + d.maxRetryDelay/5
		if delay > maxWithJitter {
			t.Errorf("attempt %d: delay %v exceeds max %v (with jitter tolerance %v)", attempt, delay, d.maxRetryDelay, maxWithJitter)
		}
	}
}

func TestHandleFailure_RetryWindowExpired(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	var failedCalled bool
	outbox := &testutil.MockOutboxStore{
		MarkFailedFunc: func(ctx context.Context, id uuid.UUID, attempts int, lastError string) error {
			failedCalled = true
			return nil
		},
	}
	audits := &testutil.MockAuditStore{
		CreateFunc: func(ctx context.Context, log *model.AuditLog) error { return nil },
	}

	d := NewDispatcher(outbox, audits, Config{
		Timeout:     5 * time.Second,
		MaxRetries:  20,
		RetryDelay:  30 * time.Second,
		RetryWindow: time.Hour, // 1h window
	})

	// Entry created 2 hours ago — window should be expired
	entry := model.OutboxEntry{
		ID:         uuid.New(),
		RequestID:  uuid.New(),
		WebhookURL: server.URL,
		Payload:    []byte(`{"event":"approved"}`),
		Attempts:   2, // Only 2 attempts, well under max of 20
		MaxRetries: 20,
		CreatedAt:  time.Now().Add(-2 * time.Hour),
	}

	d.handleFailure(context.Background(), entry, "test error")

	if !failedCalled {
		t.Error("expected MarkFailed to be called when retry window expired")
	}
}

func TestHandleFailure_WithinWindow_SchedulesRetry(t *testing.T) {
	var retryCalled bool
	outbox := &testutil.MockOutboxStore{
		MarkRetryFunc: func(ctx context.Context, id uuid.UUID, attempts int, lastError string, nextRetryAt time.Time) error {
			retryCalled = true
			return nil
		},
	}

	d := NewDispatcher(outbox, &testutil.MockAuditStore{}, Config{
		Timeout:     5 * time.Second,
		MaxRetries:  20,
		RetryDelay:  time.Millisecond,
		RetryWindow: 72 * time.Hour,
	})

	// Entry created 1 hour ago — well within the 72h window
	entry := model.OutboxEntry{
		ID:         uuid.New(),
		RequestID:  uuid.New(),
		WebhookURL: "http://localhost:9999",
		Payload:    []byte(`{"event":"approved"}`),
		Attempts:   2,
		MaxRetries: 20,
		CreatedAt:  time.Now().Add(-1 * time.Hour),
	}

	d.handleFailure(context.Background(), entry, "test error")

	if !retryCalled {
		t.Error("expected MarkRetry to be called when within retry window")
	}
}
