package webhook

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/model"
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

func TestDispatcher_Enqueue_IncludesCallbackURL(t *testing.T) {
	webhooks := &testutil.MockWebhookStore{
		ListByEventAndTypeFunc: func(ctx context.Context, event, rt string) ([]model.Webhook, error) {
			return []model.Webhook{*testutil.NewWebhook()}, nil
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

	req := testutil.NewRequest(func(r *model.Request) {
		r.Status = model.StatusApproved
		r.CallbackURL = testutil.StringPtr("https://example.com/callback")
	})

	err := dispatcher.Enqueue(context.Background(), outbox, webhooks, req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 1 from matching webhook + 1 from callback URL
	if len(createdEntries) != 2 {
		t.Errorf("expected 2 outbox entries (1 webhook + 1 callback), got %d", len(createdEntries))
	}
}

func TestDispatcher_Enqueue_CallbackURL_Signed(t *testing.T) {
	webhooks := &testutil.MockWebhookStore{
		ListByEventAndTypeFunc: func(ctx context.Context, event, rt string) ([]model.Webhook, error) {
			return nil, nil
		},
	}

	var createdEntries []model.OutboxEntry
	outbox := &testutil.MockOutboxStore{
		CreateBatchFunc: func(ctx context.Context, entries []model.OutboxEntry) error {
			createdEntries = entries
			return nil
		},
	}

	callbackSecret := "my-callback-secret"
	dispatcher := NewDispatcher(&testutil.MockOutboxStore{}, &testutil.MockAuditStore{}, Config{
		Timeout:        5 * time.Second,
		CallbackSecret: callbackSecret,
	})

	req := testutil.NewRequest(func(r *model.Request) {
		r.Status = model.StatusApproved
		r.CallbackURL = testutil.StringPtr("https://example.com/callback")
	})

	err := dispatcher.Enqueue(context.Background(), outbox, webhooks, req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(createdEntries) != 1 {
		t.Fatalf("expected 1 outbox entry, got %d", len(createdEntries))
	}
	if createdEntries[0].WebhookSecret != callbackSecret {
		t.Errorf("callback webhook secret = %q, want %q", createdEntries[0].WebhookSecret, callbackSecret)
	}
}

func TestDispatcher_Enqueue_NoCallbackURL(t *testing.T) {
	webhooks := &testutil.MockWebhookStore{
		ListByEventAndTypeFunc: func(ctx context.Context, event, rt string) ([]model.Webhook, error) {
			return []model.Webhook{*testutil.NewWebhook()}, nil
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

	req := testutil.NewRequest(func(r *model.Request) {
		r.Status = model.StatusApproved
		r.CallbackURL = nil
	})

	err := dispatcher.Enqueue(context.Background(), outbox, webhooks, req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(createdEntries) != 1 {
		t.Errorf("expected 1 outbox entry (no callback), got %d", len(createdEntries))
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
	expectedSig := "sha256=" + computeHMAC(receivedBody, secret)
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

func TestComputeHMAC_Deterministic(t *testing.T) {
	payload := []byte(`{"event":"approved"}`)
	secret := "test-secret"

	sig1 := computeHMAC(payload, secret)
	sig2 := computeHMAC(payload, secret)

	if sig1 != sig2 {
		t.Errorf("HMAC not deterministic: %s vs %s", sig1, sig2)
	}
}

func TestComputeHMAC_DifferentSecrets(t *testing.T) {
	payload := []byte(`{"event":"approved"}`)

	sig1 := computeHMAC(payload, "secret1")
	sig2 := computeHMAC(payload, "secret2")

	if sig1 == sig2 {
		t.Error("HMAC should differ for different secrets")
	}
}
