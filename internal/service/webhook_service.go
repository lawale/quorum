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
	ErrWebhookNotFound   = errors.New("webhook not found")
	ErrWebhookValidation = errors.New("webhook validation error")
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
		return fmt.Errorf("%w: webhook URL is required", ErrWebhookValidation)
	}

	// Validate that the URL is well-formed and uses http or https.
	parsed, err := url.Parse(webhook.URL)
	if err != nil {
		return fmt.Errorf("%w: invalid webhook URL: %s", ErrWebhookValidation, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%w: webhook URL must use http or https scheme", ErrWebhookValidation)
	}
	if parsed.Host == "" {
		return fmt.Errorf("%w: webhook URL must have a host", ErrWebhookValidation)
	}

	if len(webhook.Events) == 0 {
		return fmt.Errorf("%w: at least one event is required", ErrWebhookValidation)
	}
	if webhook.Secret == "" {
		return fmt.Errorf("%w: webhook secret is required", ErrWebhookValidation)
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
