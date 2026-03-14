package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/auth"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/store"
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
	if webhook.TenantID == "" {
		webhook.TenantID = auth.TenantIDFromContext(ctx)
	}

	if webhook.URL == "" {
		return errors.New("webhook URL is required")
	}

	// Validate that the URL is well-formed and uses http or https.
	parsed, err := url.Parse(webhook.URL)
	if err != nil {
		return fmt.Errorf("invalid webhook URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("webhook URL must use http or https scheme")
	}
	if parsed.Host == "" {
		return errors.New("webhook URL must have a host")
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
