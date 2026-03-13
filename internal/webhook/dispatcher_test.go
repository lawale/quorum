package webhook

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/testutil"
)

func TestDispatcher_Dispatch_QueuesMatchingWebhooks(t *testing.T) {
	wh1 := testutil.NewWebhook()
	wh2 := testutil.NewWebhook()
	webhooks := &testutil.MockWebhookStore{
		ListByEventAndTypeFunc: func(ctx context.Context, event, rt string) ([]model.Webhook, error) {
			return []model.Webhook{*wh1, *wh2}, nil
		},
	}

	dispatcher := NewDispatcher(webhooks, &testutil.MockAuditStore{}, 5*time.Second, 0, time.Millisecond)
	req := testutil.NewRequest(func(r *model.Request) { r.Status = model.StatusApproved })

	dispatcher.Dispatch(context.Background(), req, nil)

	// Should have 2 jobs in the queue (no callback URL)
	if len(dispatcher.queue) != 2 {
		t.Errorf("queue length = %d, want 2", len(dispatcher.queue))
	}
}

func TestDispatcher_Dispatch_IncludesCallbackURL(t *testing.T) {
	webhooks := &testutil.MockWebhookStore{
		ListByEventAndTypeFunc: func(ctx context.Context, event, rt string) ([]model.Webhook, error) {
			return []model.Webhook{*testutil.NewWebhook()}, nil
		},
	}

	dispatcher := NewDispatcher(webhooks, &testutil.MockAuditStore{}, 5*time.Second, 0, time.Millisecond)
	req := testutil.NewRequest(func(r *model.Request) {
		r.Status = model.StatusApproved
		r.CallbackURL = testutil.StringPtr("https://example.com/callback")
	})

	dispatcher.Dispatch(context.Background(), req, nil)

	// 1 from matching webhook + 1 from callback URL
	if len(dispatcher.queue) != 2 {
		t.Errorf("queue length = %d, want 2 (1 webhook + 1 callback)", len(dispatcher.queue))
	}
}

func TestDispatcher_Dispatch_NoCallbackURL(t *testing.T) {
	webhooks := &testutil.MockWebhookStore{
		ListByEventAndTypeFunc: func(ctx context.Context, event, rt string) ([]model.Webhook, error) {
			return []model.Webhook{*testutil.NewWebhook()}, nil
		},
	}

	dispatcher := NewDispatcher(webhooks, &testutil.MockAuditStore{}, 5*time.Second, 0, time.Millisecond)
	req := testutil.NewRequest(func(r *model.Request) {
		r.Status = model.StatusApproved
		r.CallbackURL = nil
	})

	dispatcher.Dispatch(context.Background(), req, nil)

	if len(dispatcher.queue) != 1 {
		t.Errorf("queue length = %d, want 1 (no callback)", len(dispatcher.queue))
	}
}

func TestDispatcher_Deliver_Success(t *testing.T) {
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

	dispatcher := NewDispatcher(&testutil.MockWebhookStore{}, audits, 5*time.Second, 0, time.Millisecond)

	job := deliveryJob{
		webhook: model.Webhook{URL: server.URL, Secret: "test-secret"},
		payload: model.WebhookPayload{
			Event:     "approved",
			Request:   *testutil.NewRequest(),
			Timestamp: time.Now().UTC(),
		},
		request: *testutil.NewRequest(),
	}

	dispatcher.deliver(context.Background(), job)

	if auditAction != "webhook_sent" {
		t.Errorf("audit action = %q, want %q", auditAction, "webhook_sent")
	}
}

func TestDispatcher_Deliver_RetryOnFailure(t *testing.T) {
	var mu sync.Mutex
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts++
		a := attempts
		mu.Unlock()
		if a <= 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
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

	dispatcher := NewDispatcher(&testutil.MockWebhookStore{}, audits, 5*time.Second, 2, time.Millisecond)

	job := deliveryJob{
		webhook: model.Webhook{URL: server.URL, Secret: "test"},
		payload: model.WebhookPayload{
			Event:     "approved",
			Request:   *testutil.NewRequest(),
			Timestamp: time.Now().UTC(),
		},
		request: *testutil.NewRequest(),
	}

	dispatcher.deliver(context.Background(), job)

	mu.Lock()
	totalAttempts := attempts
	mu.Unlock()

	if totalAttempts < 2 {
		t.Errorf("expected at least 2 attempts, got %d", totalAttempts)
	}
	if auditAction != "webhook_sent" {
		t.Errorf("audit action = %q, want %q (retry succeeded)", auditAction, "webhook_sent")
	}
}

func TestDispatcher_Deliver_ExhaustsRetries(t *testing.T) {
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

	dispatcher := NewDispatcher(&testutil.MockWebhookStore{}, audits, 5*time.Second, 1, time.Millisecond)

	job := deliveryJob{
		webhook: model.Webhook{URL: server.URL, Secret: "test"},
		payload: model.WebhookPayload{
			Event:     "approved",
			Request:   *testutil.NewRequest(),
			Timestamp: time.Now().UTC(),
		},
		request: *testutil.NewRequest(),
	}

	dispatcher.deliver(context.Background(), job)

	if auditAction != "webhook_failed" {
		t.Errorf("audit action = %q, want %q", auditAction, "webhook_failed")
	}
}

func TestDispatcher_Deliver_HMAC_Signature(t *testing.T) {
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

	audits := &testutil.MockAuditStore{
		CreateFunc: func(ctx context.Context, log *model.AuditLog) error { return nil },
	}

	secret := "my-webhook-secret"
	dispatcher := NewDispatcher(&testutil.MockWebhookStore{}, audits, 5*time.Second, 0, time.Millisecond)

	payload := model.WebhookPayload{
		Event:     "approved",
		Request:   *testutil.NewRequest(func(r *model.Request) { r.ID = uuid.MustParse("00000000-0000-0000-0000-000000000001") }),
		Timestamp: time.Now().UTC(),
	}
	job := deliveryJob{
		webhook: model.Webhook{URL: server.URL, Secret: secret},
		payload: payload,
		request: *testutil.NewRequest(),
	}

	dispatcher.deliver(context.Background(), job)

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

func TestDispatcher_Deliver_NoHMAC_WhenNoSecret(t *testing.T) {
	var receivedSig string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get("X-Signature-256")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	audits := &testutil.MockAuditStore{
		CreateFunc: func(ctx context.Context, log *model.AuditLog) error { return nil },
	}

	dispatcher := NewDispatcher(&testutil.MockWebhookStore{}, audits, 5*time.Second, 0, time.Millisecond)

	job := deliveryJob{
		webhook: model.Webhook{URL: server.URL, Secret: ""}, // No secret
		payload: model.WebhookPayload{
			Event:     "approved",
			Request:   *testutil.NewRequest(),
			Timestamp: time.Now().UTC(),
		},
		request: *testutil.NewRequest(),
	}

	dispatcher.deliver(context.Background(), job)

	if receivedSig != "" {
		t.Errorf("expected no X-Signature-256 header when secret is empty, got %q", receivedSig)
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
