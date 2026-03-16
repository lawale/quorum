package mssql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/store"
)

const outboxColumns = `id, request_id, webhook_url, webhook_secret, payload, status, attempts, max_retries, last_error, next_retry_at, created_at, delivered_at`

const outboxColumnsQualified = `o.id, o.request_id, o.webhook_url, o.webhook_secret, o.payload, o.status, o.attempts, o.max_retries, o.last_error, o.next_retry_at, o.created_at, o.delivered_at`

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
	// Claim pending entries and reclaim stale processing entries whose lease has
	// expired (next_retry_at in the past). Setting next_retry_at to 5 minutes
	// from now acts as a lease: if the worker crashes or MarkDelivered fails,
	// the entry becomes eligible for reclaim after the lease expires.
	query := `UPDATE TOP(@p1) o
		SET o.status = 'processing', o.next_retry_at = DATEADD(MINUTE, 5, SYSDATETIMEOFFSET())
		OUTPUT inserted.` + outboxColumns + `
		FROM [quorum].[webhook_outbox] o WITH (UPDLOCK, ROWLOCK, READPAST)
		WHERE (o.status = 'pending' OR o.status = 'processing') AND o.next_retry_at <= SYSDATETIMEOFFSET()`

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

func (s *OutboxStore) List(ctx context.Context, filter store.OutboxFilter) ([]model.OutboxEntry, int, error) {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	perPage := filter.PerPage
	if perPage < 1 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	where := "WHERE 1=1"
	args := []any{}
	argIdx := 1

	if filter.TenantID != nil {
		where += fmt.Sprintf(" AND r.tenant_id = @p%d", argIdx)
		args = append(args, *filter.TenantID)
		argIdx++
	}
	if filter.Status != nil {
		where += fmt.Sprintf(" AND o.status = @p%d", argIdx)
		args = append(args, *filter.Status)
		argIdx++
	}
	if filter.RequestID != nil {
		where += fmt.Sprintf(" AND o.request_id = @p%d", argIdx)
		args = append(args, *filter.RequestID)
		argIdx++
	}

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM [quorum].[webhook_outbox] o JOIN [quorum].[requests] r ON r.id = o.request_id %s`, where)
	var total int
	if err := s.db.Pool.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting outbox entries: %w", err)
	}

	dataQuery := fmt.Sprintf(
		`SELECT %s FROM [quorum].[webhook_outbox] o JOIN [quorum].[requests] r ON r.id = o.request_id %s ORDER BY o.created_at DESC OFFSET @p%d ROWS FETCH NEXT @p%d ROWS ONLY`,
		outboxColumnsQualified, where, argIdx, argIdx+1,
	)
	args = append(args, offset, perPage)

	rows, err := s.db.Pool.QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing outbox entries: %w", err)
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
			return nil, 0, fmt.Errorf("scanning outbox entry: %w", err)
		}
		if payload.Valid {
			e.Payload = []byte(payload.String)
		}
		if lastError.Valid {
			e.LastError = &lastError.String
		}
		entries = append(entries, e)
	}

	return entries, total, nil
}

func (s *OutboxStore) CountByStatus(ctx context.Context, tenantID *string) (map[string]int, error) {
	var query string
	var args []any

	if tenantID != nil {
		query = `SELECT o.status, COUNT(*) FROM [quorum].[webhook_outbox] o JOIN [quorum].[requests] r ON r.id = o.request_id WHERE r.tenant_id = @p1 GROUP BY o.status`
		args = []any{*tenantID}
	} else {
		query = `SELECT status, COUNT(*) FROM [quorum].[webhook_outbox] GROUP BY status`
	}

	rows, err := s.db.Pool.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("counting outbox by status: %w", err)
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scanning status count: %w", err)
		}
		counts[status] = count
	}
	return counts, nil
}

func (s *OutboxStore) GetByID(ctx context.Context, id uuid.UUID) (*model.OutboxEntry, error) {
	query := `SELECT ` + outboxColumns + ` FROM [quorum].[webhook_outbox] WHERE id = @p1`
	var e model.OutboxEntry
	var payload, lastError sql.NullString
	err := s.db.Pool.QueryRowContext(ctx, query, id).Scan(
		&e.ID, &e.RequestID, &e.WebhookURL, &e.WebhookSecret, &payload, &e.Status,
		&e.Attempts, &e.MaxRetries, &lastError, &e.NextRetryAt, &e.CreatedAt, &e.DeliveredAt,
	)
	if err != nil {
		return nil, fmt.Errorf("getting outbox entry: %w", err)
	}
	if payload.Valid {
		e.Payload = []byte(payload.String)
	}
	if lastError.Valid {
		e.LastError = &lastError.String
	}
	return &e, nil
}

func (s *OutboxStore) ResetForRetry(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE [quorum].[webhook_outbox] SET status = 'pending', attempts = 0, last_error = NULL, next_retry_at = SYSDATETIMEOFFSET() WHERE id = @p1 AND status = 'failed'`
	result, err := s.db.Pool.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("resetting outbox entry for retry: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("entry not found or not in failed status")
	}
	return nil
}

func (s *OutboxStore) ResetAllFailedForRequest(ctx context.Context, requestID uuid.UUID) (int64, error) {
	query := `UPDATE [quorum].[webhook_outbox] SET status = 'pending', attempts = 0, last_error = NULL, next_retry_at = SYSDATETIMEOFFSET() WHERE request_id = @p1 AND status = 'failed'`
	result, err := s.db.Pool.ExecContext(ctx, query, requestID)
	if err != nil {
		return 0, fmt.Errorf("resetting failed entries for request: %w", err)
	}
	return result.RowsAffected()
}
