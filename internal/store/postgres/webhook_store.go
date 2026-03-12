package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lawale/quorum/internal/model"
)

type WebhookStore struct {
	db *DB
}

func NewWebhookStore(db *DB) *WebhookStore {
	return &WebhookStore{db: db}
}

func (s *WebhookStore) Create(ctx context.Context, webhook *model.Webhook) error {
	query := `
		INSERT INTO webhooks (id, url, events, secret, request_type, active, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	if webhook.ID == uuid.Nil {
		webhook.ID = uuid.New()
	}
	webhook.CreatedAt = time.Now().UTC()
	if !webhook.Active {
		webhook.Active = true
	}

	eventsJSON, err := marshalJSON(webhook.Events)
	if err != nil {
		return fmt.Errorf("marshaling events: %w", err)
	}

	_, err = s.db.Pool.Exec(ctx, query,
		webhook.ID, webhook.URL, eventsJSON, webhook.Secret,
		webhook.RequestType, webhook.Active, webhook.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting webhook: %w", err)
	}

	return nil
}

func (s *WebhookStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Webhook, error) {
	query := `SELECT id, url, events, secret, request_type, active, created_at FROM webhooks WHERE id = $1`

	w := &model.Webhook{}
	var eventsJSON []byte
	err := s.db.Pool.QueryRow(ctx, query, id).Scan(
		&w.ID, &w.URL, &eventsJSON, &w.Secret, &w.RequestType, &w.Active, &w.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("querying webhook: %w", err)
	}

	if err := unmarshalJSON(eventsJSON, &w.Events); err != nil {
		return nil, fmt.Errorf("unmarshaling events: %w", err)
	}

	return w, nil
}

func (s *WebhookStore) List(ctx context.Context) ([]model.Webhook, error) {
	query := `SELECT id, url, events, secret, request_type, active, created_at FROM webhooks ORDER BY created_at DESC`

	rows, err := s.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing webhooks: %w", err)
	}
	defer rows.Close()

	var webhooks []model.Webhook
	for rows.Next() {
		var w model.Webhook
		var eventsJSON []byte
		if err := rows.Scan(&w.ID, &w.URL, &eventsJSON, &w.Secret, &w.RequestType, &w.Active, &w.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning webhook: %w", err)
		}
		if err := unmarshalJSON(eventsJSON, &w.Events); err != nil {
			return nil, fmt.Errorf("unmarshaling events: %w", err)
		}
		webhooks = append(webhooks, w)
	}

	return webhooks, nil
}

func (s *WebhookStore) ListByEventAndType(ctx context.Context, event string, requestType string) ([]model.Webhook, error) {
	query := `
		SELECT id, url, events, secret, request_type, active, created_at
		FROM webhooks
		WHERE active = true AND events @> $1::jsonb
		AND (request_type IS NULL OR request_type = $2)`

	eventJSON := fmt.Sprintf(`["%s"]`, event)

	rows, err := s.db.Pool.Query(ctx, query, eventJSON, requestType)
	if err != nil {
		return nil, fmt.Errorf("listing webhooks by event: %w", err)
	}
	defer rows.Close()

	var webhooks []model.Webhook
	for rows.Next() {
		var w model.Webhook
		var eventsJSON []byte
		if err := rows.Scan(&w.ID, &w.URL, &eventsJSON, &w.Secret, &w.RequestType, &w.Active, &w.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning webhook: %w", err)
		}
		if err := unmarshalJSON(eventsJSON, &w.Events); err != nil {
			return nil, fmt.Errorf("unmarshaling events: %w", err)
		}
		webhooks = append(webhooks, w)
	}

	return webhooks, nil
}

func (s *WebhookStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.Pool.Exec(ctx, "DELETE FROM webhooks WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("deleting webhook: %w", err)
	}
	return nil
}
