package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/model"
)

const outboxColumns = `id, request_id, webhook_url, webhook_secret, payload, status, attempts, max_retries, last_error, next_retry_at, created_at, delivered_at`

type OutboxStore struct {
	db *DB
}

func NewOutboxStore(db *DB) *OutboxStore {
	return &OutboxStore{db: db}
}

func (s *OutboxStore) CreateBatch(ctx context.Context, entries []model.OutboxEntry) error {
	if len(entries) == 0 {
		return nil
	}

	query := `INSERT INTO webhook_outbox (id, request_id, webhook_url, webhook_secret, payload, max_retries)
		VALUES ($1, $2, $3, $4, $5, $6)`

	for i := range entries {
		e := &entries[i]
		if e.ID == uuid.Nil {
			e.ID = uuid.New()
		}

		_, err := s.db.Pool.Exec(ctx, query,
			e.ID, e.RequestID, e.WebhookURL, e.WebhookSecret, e.Payload, e.MaxRetries,
		)
		if err != nil {
			return fmt.Errorf("inserting outbox entry %d: %w", i, err)
		}
	}

	return nil
}

func (s *OutboxStore) ListPending(ctx context.Context, limit int) ([]model.OutboxEntry, error) {
	query := `SELECT ` + outboxColumns + `
		FROM webhook_outbox
		WHERE status = 'pending' AND next_retry_at <= NOW()
		ORDER BY created_at ASC
		LIMIT $1`

	rows, err := s.db.Pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("querying pending outbox entries: %w", err)
	}
	defer rows.Close()

	var entries []model.OutboxEntry
	for rows.Next() {
		var e model.OutboxEntry
		if err := rows.Scan(
			&e.ID, &e.RequestID, &e.WebhookURL, &e.WebhookSecret, &e.Payload, &e.Status,
			&e.Attempts, &e.MaxRetries, &e.LastError, &e.NextRetryAt, &e.CreatedAt, &e.DeliveredAt,
		); err != nil {
			return nil, fmt.Errorf("scanning outbox entry: %w", err)
		}
		entries = append(entries, e)
	}

	return entries, nil
}

func (s *OutboxStore) MarkDelivered(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE webhook_outbox SET status = 'delivered', delivered_at = NOW() WHERE id = $1`
	_, err := s.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("marking outbox entry delivered: %w", err)
	}
	return nil
}

func (s *OutboxStore) MarkRetry(ctx context.Context, id uuid.UUID, attempts int, lastError string, nextRetryAt time.Time) error {
	query := `UPDATE webhook_outbox SET attempts = $1, last_error = $2, next_retry_at = $3 WHERE id = $4`
	_, err := s.db.Pool.Exec(ctx, query, attempts, lastError, nextRetryAt, id)
	if err != nil {
		return fmt.Errorf("marking outbox entry for retry: %w", err)
	}
	return nil
}

func (s *OutboxStore) MarkFailed(ctx context.Context, id uuid.UUID, attempts int, lastError string) error {
	query := `UPDATE webhook_outbox SET status = 'failed', attempts = $1, last_error = $2 WHERE id = $3`
	_, err := s.db.Pool.Exec(ctx, query, attempts, lastError, id)
	if err != nil {
		return fmt.Errorf("marking outbox entry failed: %w", err)
	}
	return nil
}

func (s *OutboxStore) DeleteDelivered(ctx context.Context, olderThan time.Time) (int64, error) {
	query := `DELETE FROM webhook_outbox WHERE status = 'delivered' AND delivered_at < $1`
	tag, err := s.db.Pool.Exec(ctx, query, olderThan)
	if err != nil {
		return 0, fmt.Errorf("deleting delivered outbox entries: %w", err)
	}
	return tag.RowsAffected(), nil
}
