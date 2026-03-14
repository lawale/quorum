package mssql

import (
	"context"
	"database/sql"
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

	query := `INSERT INTO [quorum].[webhook_outbox] (id, request_id, webhook_url, webhook_secret, payload, max_retries)
		VALUES (@p1, @p2, @p3, @p4, @p5, @p6)`

	for i := range entries {
		e := &entries[i]
		if e.ID == uuid.Nil {
			e.ID = uuid.New()
		}

		_, err := s.db.Pool.ExecContext(ctx, query,
			e.ID, e.RequestID, e.WebhookURL, e.WebhookSecret, string(e.Payload), e.MaxRetries,
		)
		if err != nil {
			return fmt.Errorf("inserting outbox entry %d: %w", i, err)
		}
	}

	return nil
}

func (s *OutboxStore) ClaimBatch(ctx context.Context, limit int) ([]model.OutboxEntry, error) {
	query := `UPDATE TOP(@p1) o
		SET o.status = 'processing'
		OUTPUT inserted.` + outboxColumns + `
		FROM [quorum].[webhook_outbox] o WITH (UPDLOCK, ROWLOCK, READPAST)
		WHERE o.status = 'pending' AND o.next_retry_at <= SYSDATETIMEOFFSET()`

	rows, err := s.db.Pool.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("claiming pending outbox entries: %w", err)
	}
	defer rows.Close()

	var entries []model.OutboxEntry
	for rows.Next() {
		var e model.OutboxEntry
		var payload, lastError sql.NullString
		if err := rows.Scan(
			&e.ID, &e.RequestID, &e.WebhookURL, &e.WebhookSecret, &payload, &e.Status,
			&e.Attempts, &e.MaxRetries, &lastError, &e.NextRetryAt, &e.CreatedAt, &e.DeliveredAt,
		); err != nil {
			return nil, fmt.Errorf("scanning outbox entry: %w", err)
		}
		if payload.Valid {
			e.Payload = []byte(payload.String)
		}
		if lastError.Valid {
			e.LastError = &lastError.String
		}
		entries = append(entries, e)
	}

	return entries, nil
}

func (s *OutboxStore) MarkDelivered(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE [quorum].[webhook_outbox] SET status = 'delivered', delivered_at = SYSDATETIMEOFFSET() WHERE id = @p1`
	_, err := s.db.Pool.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("marking outbox entry delivered: %w", err)
	}
	return nil
}

func (s *OutboxStore) MarkRetry(ctx context.Context, id uuid.UUID, attempts int, lastError string, nextRetryAt time.Time) error {
	query := `UPDATE [quorum].[webhook_outbox] SET status = 'pending', attempts = @p1, last_error = @p2, next_retry_at = @p3 WHERE id = @p4`
	_, err := s.db.Pool.ExecContext(ctx, query, attempts, lastError, nextRetryAt, id)
	if err != nil {
		return fmt.Errorf("marking outbox entry for retry: %w", err)
	}
	return nil
}

func (s *OutboxStore) MarkFailed(ctx context.Context, id uuid.UUID, attempts int, lastError string) error {
	query := `UPDATE [quorum].[webhook_outbox] SET status = 'failed', attempts = @p1, last_error = @p2 WHERE id = @p3`
	_, err := s.db.Pool.ExecContext(ctx, query, attempts, lastError, id)
	if err != nil {
		return fmt.Errorf("marking outbox entry failed: %w", err)
	}
	return nil
}

func (s *OutboxStore) DeleteDelivered(ctx context.Context, olderThan time.Time) (int64, error) {
	query := `DELETE FROM [quorum].[webhook_outbox] WHERE status = 'delivered' AND delivered_at < @p1`
	result, err := s.db.Pool.ExecContext(ctx, query, olderThan)
	if err != nil {
		return 0, fmt.Errorf("deleting delivered outbox entries: %w", err)
	}
	return result.RowsAffected()
}
