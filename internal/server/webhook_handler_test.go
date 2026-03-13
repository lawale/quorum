package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/service"
	"github.com/lawale/quorum/internal/testutil"
)

func newTestWebhookHandler(webhooks *testutil.MockWebhookStore) *WebhookHandler {
	svc := service.NewWebhookService(webhooks)
	return NewWebhookHandler(svc)
}

func TestWebhookHandler_Create_Success(t *testing.T) {
	webhooks := &testutil.MockWebhookStore{
		CreateFunc: func(ctx context.Context, webhook *model.Webhook) error { return nil },
	}
	handler := newTestWebhookHandler(webhooks)

	body := `{"url":"https://example.com/hook","events":["approved"],"secret":"test-secret"}`
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
}

func TestWebhookHandler_Create_InvalidBody(t *testing.T) {
	handler := newTestWebhookHandler(&testutil.MockWebhookStore{})

	req := httptest.NewRequest("POST", "/", bytes.NewBufferString("not json"))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestWebhookHandler_Create_ValidationError(t *testing.T) {
	handler := newTestWebhookHandler(&testutil.MockWebhookStore{})

	// Missing URL
	body := `{"events":["approved"],"secret":"test-secret"}`
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestWebhookHandler_List_Success(t *testing.T) {
	webhooks := &testutil.MockWebhookStore{
		ListFunc: func(ctx context.Context) ([]model.Webhook, error) {
			return []model.Webhook{*testutil.NewWebhook()}, nil
		},
	}
	handler := newTestWebhookHandler(webhooks)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestWebhookHandler_Delete_Success(t *testing.T) {
	webhookID := uuid.New()
	existing := testutil.NewWebhook(func(w *model.Webhook) { w.ID = webhookID })
	webhooks := &testutil.MockWebhookStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Webhook, error) { return existing, nil },
		DeleteFunc:  func(ctx context.Context, id uuid.UUID) error { return nil },
	}
	handler := newTestWebhookHandler(webhooks)

	req := httptest.NewRequest("DELETE", "/", nil)
	req = req.WithContext(chiContext(req.Context(), "id", webhookID.String()))
	rec := httptest.NewRecorder()

	handler.Delete(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestWebhookHandler_Delete_NotFound(t *testing.T) {
	webhooks := &testutil.MockWebhookStore{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.Webhook, error) { return nil, nil },
	}
	handler := newTestWebhookHandler(webhooks)

	req := httptest.NewRequest("DELETE", "/", nil)
	req = req.WithContext(chiContext(req.Context(), "id", uuid.New().String()))
	rec := httptest.NewRecorder()

	handler.Delete(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestWebhookHandler_Delete_InvalidID(t *testing.T) {
	handler := newTestWebhookHandler(&testutil.MockWebhookStore{})

	req := httptest.NewRequest("DELETE", "/", nil)
	req = req.WithContext(chiContext(req.Context(), "id", "not-a-uuid"))
	rec := httptest.NewRecorder()

	handler.Delete(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
