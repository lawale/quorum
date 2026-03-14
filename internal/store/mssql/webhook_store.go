package mssql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/auth"
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
		INSERT INTO [quorum].[webhooks] (id, tenant_id, url, events, secret, request_type, active, created_at)
		VALUES (@p1, @p2, @p3, @p4, @p5, @p6, @p7, @p8)`

	if webhook.ID == uuid.Nil {
		webhook.ID = uuid.New()
	}
	webhook.TenantID = auth.TenantIDFromContext(ctx)
	webhook.CreatedAt = time.Now().UTC()
	if !webhook.Active {
		webhook.Active = true
	}

	eventsJSON, err := marshalJSON(webhook.Events)
	if err != nil {
		return fmt.Errorf("marshaling events: %w", err)
	}

	_, err = s.db.Pool.ExecContext(ctx, query,
		webhook.ID, webhook.TenantID, webhook.URL, string(eventsJSON), webhook.Secret,
		webhook.RequestType, webhook.Active, webhook.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting webhook: %w", err)
	}

	return nil
}

func (s *WebhookStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Webhook, error) {
	tenantID := auth.TenantIDFromContext(ctx)
	var query string
	var args []any
	if tenantID != "" {
		query = `SELECT id, tenant_id, url, events, secret, request_type, active, created_at FROM [quorum].[webhooks] WHERE id = @p1 AND tenant_id = @p2`
		args = []any{id, tenantID}
	} else {
		query = `SELECT id, tenant_id, url, events, secret, request_type, active, created_at FROM [quorum].[webhooks] WHERE id = @p1`
		args = []any{id}
	}

	w := &model.Webhook{}
	var eventsJSON string
	err := s.db.Pool.QueryRowContext(ctx, query, args...).Scan(
		&w.ID, &w.TenantID, &w.URL, &eventsJSON, &w.Secret, &w.RequestType, &w.Active, &w.CreatedAt,
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
	var query string
	var args []any
	tenantID := auth.TenantIDFromContext(ctx)
	if tenantID != "" {
		query = `SELECT id, tenant_id, url, events, secret, request_type, active, created_at FROM [quorum].[webhooks] WHERE tenant_id = @p1 ORDER BY created_at DESC`
		args = append(args, tenantID)
	} else {
		query = `SELECT id, tenant_id, url, events, secret, request_type, active, created_at FROM [quorum].[webhooks] ORDER BY created_at DESC`
	}

	rows, err := s.db.Pool.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing webhooks: %w", err)
	}
	defer rows.Close()

	var webhooks []model.Webhook
	for rows.Next() {
		var w model.Webhook
		var eventsJSON string
		if err := rows.Scan(&w.ID, &w.TenantID, &w.URL, &eventsJSON, &w.Secret, &w.RequestType, &w.Active, &w.CreatedAt); err != nil {
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
	tenantID := auth.TenantIDFromContext(ctx)
	var query string
	var args []any
	if tenantID != "" {
		query = `
		SELECT id, tenant_id, url, events, secret, request_type, active, created_at
		FROM [quorum].[webhooks]
		WHERE active = 1 AND tenant_id = @p1 AND EXISTS (
			SELECT 1 FROM OPENJSON(events) AS j WHERE j.value = @p2
		)
		AND (request_type IS NULL OR request_type = @p3)`
		args = []any{tenantID, event, requestType}
	} else {
		query = `
		SELECT id, tenant_id, url, events, secret, request_type, active, created_at
		FROM [quorum].[webhooks]
		WHERE active = 1 AND EXISTS (
			SELECT 1 FROM OPENJSON(events) AS j WHERE j.value = @p1
		)
		AND (request_type IS NULL OR request_type = @p2)`
		args = []any{event, requestType}
	}

	rows, err := s.db.Pool.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing webhooks by event: %w", err)
	}
	defer rows.Close()

	var webhooks []model.Webhook
	for rows.Next() {
		var w model.Webhook
		var eventsJSON string
		if err := rows.Scan(&w.ID, &w.TenantID, &w.URL, &eventsJSON, &w.Secret, &w.RequestType, &w.Active, &w.CreatedAt); err != nil {
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
	tenantID := auth.TenantIDFromContext(ctx)
	if tenantID != "" {
		_, err := s.db.Pool.ExecContext(ctx, "DELETE FROM [quorum].[webhooks] WHERE id = @p1 AND tenant_id = @p2", id, tenantID)
		if err != nil {
			return fmt.Errorf("deleting webhook: %w", err)
		}
		return nil
	}
	_, err := s.db.Pool.ExecContext(ctx, "DELETE FROM [quorum].[webhooks] WHERE id = @p1", id)
	if err != nil {
		return fmt.Errorf("deleting webhook: %w", err)
	}
	return nil
}
