package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/wale/maker-checker/internal/model"
	"github.com/wale/maker-checker/internal/store"
)

var (
	ErrWebhookNotFound = errors.New("webhook not found")
)

type WebhookService struct {
	webhooks store.WebhookStore
}

func NewWebhookService(webhooks store.WebhookStore) *WebhookService {
	return &WebhookService{webhooks: webhooks}
}

func (s *WebhookService) Create(ctx context.Context, webhook *model.Webhook) error {
	if webhook.URL == "" {
		return errors.New("webhook URL is required")
	}
	if len(webhook.Events) == 0 {
		return errors.New("at least one event is required")
	}
	if webhook.Secret == "" {
		return errors.New("webhook secret is required")
	}

	return s.webhooks.Create(ctx, webhook)
}

func (s *WebhookService) List(ctx context.Context) ([]model.Webhook, error) {
	return s.webhooks.List(ctx)
}

func (s *WebhookService) Delete(ctx context.Context, id uuid.UUID) error {
	existing, err := s.webhooks.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("looking up webhook: %w", err)
	}
	if existing == nil {
		return ErrWebhookNotFound
	}

	return s.webhooks.Delete(ctx, id)
}

func (s *WebhookService) GetMatchingWebhooks(ctx context.Context, event string, requestType string) ([]model.Webhook, error) {
	return s.webhooks.ListByEventAndType(ctx, event, requestType)
}
