package mssql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
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
		VALUES (@p1, @p2, @p3, @p4, @p5, @p6, @p7)`

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

	_, err = s.db.Pool.ExecContext(ctx, query,
		webhook.ID, webhook.URL, string(eventsJSON), webhook.Secret,
		webhook.RequestType, webhook.Active, webhook.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting webhook: %w", err)
	}

	return nil
}

func (s *WebhookStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Webhook, error) {
	query := `SELECT id, url, events, secret, request_type, active, created_at FROM webhooks WHERE id = @p1`

	w := &model.Webhook{}
	var eventsJSON string
	err := s.db.Pool.QueryRowContext(ctx, query, id).Scan(
		&w.ID, &w.URL, &eventsJSON, &w.Secret, &w.RequestType, &w.Active, &w.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying webhook: %w", err)
	}

	if err := unmarshalJSON([]byte(eventsJSON), &w.Events); err != nil {
		return nil, fmt.Errorf("unmarshaling events: %w", err)
	}

	return w, nil
}

func (s *WebhookStore) List(ctx context.Context) ([]model.Webhook, error) {
	query := `SELECT id, url, events, secret, request_type, active, created_at FROM webhooks ORDER BY created_at DESC`

	rows, err := s.db.Pool.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing webhooks: %w", err)
	}
	defer rows.Close()

	var webhooks []model.Webhook
	for rows.Next() {
		var w model.Webhook
		var eventsJSON string
		if err := rows.Scan(&w.ID, &w.URL, &eventsJSON, &w.Secret, &w.RequestType, &w.Active, &w.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning webhook: %w", err)
		}
		if err := unmarshalJSON([]byte(eventsJSON), &w.Events); err != nil {
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
		WHERE active = 1 AND EXISTS (
			SELECT 1 FROM OPENJSON(events) AS j WHERE j.value = @p1
		)
		AND (request_type IS NULL OR request_type = @p2)`

	rows, err := s.db.Pool.QueryContext(ctx, query, event, requestType)
	if err != nil {
		return nil, fmt.Errorf("listing webhooks by event: %w", err)
	}
	defer rows.Close()

	var webhooks []model.Webhook
	for rows.Next() {
		var w model.Webhook
		var eventsJSON string
		if err := rows.Scan(&w.ID, &w.URL, &eventsJSON, &w.Secret, &w.RequestType, &w.Active, &w.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning webhook: %w", err)
		}
		if err := unmarshalJSON([]byte(eventsJSON), &w.Events); err != nil {
			return nil, fmt.Errorf("unmarshaling events: %w", err)
		}
		webhooks = append(webhooks, w)
	}

	return webhooks, nil
}

func (s *WebhookStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.Pool.ExecContext(ctx, "DELETE FROM webhooks WHERE id = @p1", id)
	if err != nil {
		return fmt.Errorf("deleting webhook: %w", err)
	}
	return nil
}
