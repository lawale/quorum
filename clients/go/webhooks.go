package client

import (
	"context"
	"net/http"
	"net/url"

	"github.com/google/uuid"
)

// CreateWebhook subscribes a URL to events. Deliveries are verified on your
// side with ParseWebhook and the same secret.
func (c *Client) CreateWebhook(ctx context.Context, in WebhookInput) (*Webhook, error) {
	var out Webhook
	if err := c.do(ctx, http.MethodPost, "/api/v1/webhooks", nil, nil, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListWebhooks returns one page of the tenant's subscriptions, newest first.
func (c *Client) ListWebhooks(ctx context.Context, p Paging) (*Page[Webhook], error) {
	q := url.Values{}
	setPaging(q, p.Page, p.PerPage)
	var out Page[Webhook]
	if err := c.do(ctx, http.MethodGet, "/api/v1/webhooks", q, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteWebhook removes a subscription.
func (c *Client) DeleteWebhook(ctx context.Context, id uuid.UUID) error {
	return c.do(ctx, http.MethodDelete, "/api/v1/webhooks/"+id.String(), nil, nil, nil, nil)
}
