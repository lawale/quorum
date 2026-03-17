package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lawale/quorum/internal/auth"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/store"
)

type WebhookStore struct {
	db *DB
}

func NewWebhookStore(db *DB) *WebhookStore {
	return &WebhookStore{db: db}
}

func (s *WebhookStore) Create(ctx context.Context, webhook *model.Webhook) error {
	query := `
		INSERT INTO webhooks (id, tenant_id, url, events, secret, request_type, active, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	if webhook.ID == uuid.Nil {
		webhook.ID = uuid.New()
	}
	webhook.TenantID = auth.TenantIDFromContext(ctx)
	webhook.CreatedAt = time.Now().UTC()
	webhook.Active = true

	eventsJSON, err := marshalJSON(webhook.Events)
	if err != nil {
		return fmt.Errorf("marshaling events: %w", err)
	}

	_, err = s.db.Pool.Exec(ctx, query,
		webhook.ID, webhook.TenantID, webhook.URL, eventsJSON, webhook.Secret,
		webhook.RequestType, webhook.Active, webhook.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting webhook: %w", err)
	}

	return nil
}

func (s *WebhookStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Webhook, error) {
	query := `SELECT id, tenant_id, url, events, secret, request_type, active, created_at FROM webhooks WHERE id = $1`

	var args []any
	args = append(args, id)
	tenant := auth.TenantIDFromContext(ctx)
	if tenant != "" {
		query += " AND tenant_id = $2"
		args = append(args, tenant)
	}

	w := &model.Webhook{}
	var eventsJSON []byte
	err := s.db.Pool.QueryRow(ctx, query, args...).Scan(
		&w.ID, &w.TenantID, &w.URL, &eventsJSON, &w.Secret, &w.RequestType, &w.Active, &w.CreatedAt,
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

func (s *WebhookStore) List(ctx context.Context, filter store.WebhookFilter) ([]model.Webhook, int, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PerPage < 1 {
		filter.PerPage = 20
	}
	offset := (filter.Page - 1) * filter.PerPage

	tenant := auth.TenantIDFromContext(ctx)

	var where string
	var args []any
	argIdx := 1
	if tenant != "" {
		where = fmt.Sprintf("WHERE tenant_id = $%d", argIdx)
		args = append(args, tenant)
		argIdx++
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM webhooks %s", where)
	var total int
	if err := s.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting webhooks: %w", err)
	}

	dataQuery := fmt.Sprintf(
		`SELECT id, tenant_id, url, events, secret, request_type, active, created_at FROM webhooks %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	args = append(args, filter.PerPage, offset)

	rows, err := s.db.Pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing webhooks: %w", err)
	}
	defer rows.Close()

	var webhooks []model.Webhook
	for rows.Next() {
		var w model.Webhook
		var eventsJSON []byte
		if err := rows.Scan(&w.ID, &w.TenantID, &w.URL, &eventsJSON, &w.Secret, &w.RequestType, &w.Active, &w.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scanning webhook: %w", err)
		}
		if err := unmarshalJSON(eventsJSON, &w.Events); err != nil {
			return nil, 0, fmt.Errorf("unmarshaling events: %w", err)
		}
		webhooks = append(webhooks, w)
	}

	return webhooks, total, nil
}

func (s *WebhookStore) ListByEventAndType(ctx context.Context, event string, requestType string) ([]model.Webhook, error) {
	query := `
		SELECT id, tenant_id, url, events, secret, request_type, active, created_at
		FROM webhooks
		WHERE active = true AND events @> $1::jsonb
		AND (request_type IS NULL OR request_type = $2)`

	eventJSON := fmt.Sprintf(`["%s"]`, event)

	var args []any
	args = append(args, eventJSON, requestType)
	tenant := auth.TenantIDFromContext(ctx)
	if tenant != "" {
		query += " AND tenant_id = $3"
		args = append(args, tenant)
	}

	rows, err := s.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing webhooks by event: %w", err)
	}
	defer rows.Close()

	var webhooks []model.Webhook
	for rows.Next() {
		var w model.Webhook
		var eventsJSON []byte
		if err := rows.Scan(&w.ID, &w.TenantID, &w.URL, &eventsJSON, &w.Secret, &w.RequestType, &w.Active, &w.CreatedAt); err != nil {
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
	query := "DELETE FROM webhooks WHERE id = $1"
	var tag pgconn.CommandTag
	var err error
	tenant := auth.TenantIDFromContext(ctx)
	if tenant != "" {
		query += " AND tenant_id = $2"
		tag, err = s.db.Pool.Exec(ctx, query, id, tenant)
	} else {
		tag, err = s.db.Pool.Exec(ctx, query, id)
	}
	if err != nil {
		return fmt.Errorf("deleting webhook: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	return nil
}
