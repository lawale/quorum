package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/testutil"
)

func TestWebhookCreate_Success(t *testing.T) {
	webhooks := &testutil.MockWebhookStore{
		CreateFunc: func(ctx context.Context, webhook *model.Webhook) error {
			return nil
		},
	}
	svc := NewWebhookService(webhooks)

	webhook := testutil.NewWebhook()
	err := svc.Create(context.Background(), webhook)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWebhookCreate_MissingURL(t *testing.T) {
	svc := NewWebhookService(&testutil.MockWebhookStore{})

	webhook := testutil.NewWebhook(func(w *model.Webhook) { w.URL = "" })
	err := svc.Create(context.Background(), webhook)
	if err == nil {
		t.Fatal("expected error for missing URL")
	}
}

func TestWebhookCreate_MissingEvents(t *testing.T) {
	svc := NewWebhookService(&testutil.MockWebhookStore{})

	webhook := testutil.NewWebhook(func(w *model.Webhook) { w.Events = nil })
	err := svc.Create(context.Background(), webhook)
	if err == nil {
		t.Fatal("expected error for missing events")
	}
}

func TestWebhookCreate_MissingSecret(t *testing.T) {
	svc := NewWebhookService(&testutil.MockWebhookStore{})

	webhook := testutil.NewWebhook(func(w *model.Webhook) { w.Secret = "" })
	err := svc.Create(context.Background(), webhook)
	if err == nil {
		t.Fatal("expected error for missing secret")
	}
}

func TestWebhookList_Success(t *testing.T) {
	expected := []model.Webhook{*testutil.NewWebhook(), *testutil.NewWebhook()}
	webhooks := &testutil.MockWebhookStore{
		ListFunc: func(ctx context.Context) ([]model.Webhook, error) {
			return expected, nil
		},
	}
	svc := NewWebhookService(webhooks)

	result, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 webhooks, got %d", len(result))
	}
}

func TestWebhookDelete_Success(t *testing.T) {
	existing := testutil.NewWebhook()
	webhooks := &testutil.MockWebhookStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Webhook, error) {
			return existing, nil
		},
		DeleteFunc: func(ctx context.Context, id uuid.UUID) error {
			return nil
		},
	}
	svc := NewWebhookService(webhooks)

	err := svc.Delete(context.Background(), existing.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWebhookDelete_NotFound(t *testing.T) {
	webhooks := &testutil.MockWebhookStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Webhook, error) {
			return nil, nil
		},
	}
	svc := NewWebhookService(webhooks)

	err := svc.Delete(context.Background(), uuid.New())
	if !errors.Is(err, ErrWebhookNotFound) {
		t.Fatalf("expected ErrWebhookNotFound, got: %v", err)
	}
}

func TestWebhookGetMatchingWebhooks_DelegatesToStore(t *testing.T) {
	expected := []model.Webhook{*testutil.NewWebhook()}
	webhooks := &testutil.MockWebhookStore{
		ListByEventAndTypeFunc: func(ctx context.Context, event, rt string) ([]model.Webhook, error) {
			if event != "approved" || rt != "transfer" {
				t.Errorf("unexpected args: event=%q, rt=%q", event, rt)
			}
			return expected, nil
		},
	}
	svc := NewWebhookService(webhooks)

	result, err := svc.GetMatchingWebhooks(context.Background(), "approved", "transfer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 webhook, got %d", len(result))
	}
}
