package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lawale/quorum/internal/auth"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/store"
)

const outboxColumns = `id, request_id, webhook_url, webhook_secret, payload, event_type, status, attempts, max_retries, last_error, next_retry_at, created_at, delivered_at`

// outboxColumnsQualified prefixes every column with the table alias "o." for use in JOINed queries.
const outboxColumnsQualified = `o.id, o.request_id, o.webhook_url, o.webhook_secret, o.payload, o.event_type, o.status, o.attempts, o.max_retries, o.last_error, o.next_retry_at, o.created_at, o.delivered_at`

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

	query := `INSERT INTO webhook_outbox (id, request_id, webhook_url, webhook_secret, payload, event_type, max_retries)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	for i := range entries {
		e := &entries[i]
		if e.ID == uuid.Nil {
			e.ID = uuid.New()
		}

		_, err := s.db.Pool.Exec(ctx, query,
			e.ID, e.RequestID, e.WebhookURL, e.WebhookSecret, e.Payload, e.EventType, e.MaxRetries,
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
	query := `UPDATE webhook_outbox
		SET status = 'processing', next_retry_at = NOW() + INTERVAL '5 minutes'
		WHERE id IN (
			SELECT id FROM webhook_outbox
			WHERE (status = 'pending' OR status = 'processing') AND next_retry_at <= NOW()
			ORDER BY created_at ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING ` + outboxColumns

	rows, err := s.db.Pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("claiming pending outbox entries: %w", err)
	}
	defer rows.Close()

	var entries []model.OutboxEntry
	for rows.Next() {
		var e model.OutboxEntry
		if err := rows.Scan(
			&e.ID, &e.RequestID, &e.WebhookURL, &e.WebhookSecret, &e.Payload, &e.EventType, &e.Status,
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
	query := `UPDATE webhook_outbox SET status = 'pending', attempts = $1, last_error = $2, next_retry_at = $3 WHERE id = $4`
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
		where += fmt.Sprintf(" AND r.tenant_id = $%d", argIdx)
		args = append(args, *filter.TenantID)
		argIdx++
	}
	if filter.Status != nil {
		where += fmt.Sprintf(" AND o.status = $%d", argIdx)
		args = append(args, *filter.Status)
		argIdx++
	}
	if filter.RequestID != nil {
		where += fmt.Sprintf(" AND o.request_id = $%d", argIdx)
		args = append(args, *filter.RequestID)
		argIdx++
	}
	if filter.Event != nil {
		where += fmt.Sprintf(" AND o.event_type ILIKE $%d", argIdx)
		args = append(args, "%"+*filter.Event+"%")
		argIdx++
	}

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM webhook_outbox o JOIN requests r ON r.id = o.request_id %s`, where)
	var total int
	if err := s.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting outbox entries: %w", err)
	}

	dataQuery := fmt.Sprintf(
		`SELECT %s FROM webhook_outbox o JOIN requests r ON r.id = o.request_id %s ORDER BY o.created_at DESC LIMIT $%d OFFSET $%d`,
		outboxColumnsQualified, where, argIdx, argIdx+1,
	)
	args = append(args, perPage, offset)

	rows, err := s.db.Pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing outbox entries: %w", err)
	}
	defer rows.Close()

	var entries []model.OutboxEntry
	for rows.Next() {
		var e model.OutboxEntry
		if err := rows.Scan(
			&e.ID, &e.RequestID, &e.WebhookURL, &e.WebhookSecret, &e.Payload, &e.EventType, &e.Status,
			&e.Attempts, &e.MaxRetries, &e.LastError, &e.NextRetryAt, &e.CreatedAt, &e.DeliveredAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scanning outbox entry: %w", err)
		}
		entries = append(entries, e)
	}

	return entries, total, nil
}

func (s *OutboxStore) CountByStatus(ctx context.Context, tenantID *string, since *time.Time) (map[string]int, error) {
	where := []string{}
	args := []any{}
	argIdx := 1
	needsJoin := tenantID != nil

	if tenantID != nil {
		where = append(where, fmt.Sprintf("r.tenant_id = $%d", argIdx))
		args = append(args, *tenantID)
		argIdx++
	}
	if since != nil {
		where = append(where, fmt.Sprintf("o.created_at >= $%d", argIdx))
		args = append(args, *since)
		needsJoin = true
	}

	var query string
	if needsJoin {
		whereClause := ""
		if len(where) > 0 {
			whereClause = " WHERE " + where[0]
			for _, w := range where[1:] {
				whereClause += " AND " + w
			}
		}
		query = `SELECT o.status, COUNT(*) FROM webhook_outbox o JOIN requests r ON r.id = o.request_id` + whereClause + ` GROUP BY o.status`
	} else {
		query = `SELECT status, COUNT(*) FROM webhook_outbox GROUP BY status`
	}

	rows, err := s.db.Pool.Query(ctx, query, args...)
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

func (s *OutboxStore) ResetAllFailed(ctx context.Context, tenantID *string) (int64, error) {
	var query string
	var args []any

	if tenantID != nil {
		query = `UPDATE webhook_outbox SET status = 'pending', attempts = 0, last_error = NULL, next_retry_at = NOW()
			WHERE status = 'failed' AND request_id IN (SELECT id FROM requests WHERE tenant_id = $1)`
		args = []any{*tenantID}
	} else {
		query = `UPDATE webhook_outbox SET status = 'pending', attempts = 0, last_error = NULL, next_retry_at = NOW() WHERE status = 'failed'`
	}

	tag, err := s.db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("resetting all failed outbox entries: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (s *OutboxStore) GetByID(ctx context.Context, id uuid.UUID) (*model.OutboxEntry, error) {
	args := []any{id}
	var query string

	if tenant := auth.TenantIDFromContext(ctx); tenant != "" {
		query = `SELECT ` + outboxColumnsQualified + ` FROM webhook_outbox o JOIN requests r ON r.id = o.request_id WHERE o.id = $1 AND r.tenant_id = $2`
		args = append(args, tenant)
	} else {
		query = `SELECT ` + outboxColumns + ` FROM webhook_outbox WHERE id = $1`
	}

	var e model.OutboxEntry
	err := s.db.Pool.QueryRow(ctx, query, args...).Scan(
		&e.ID, &e.RequestID, &e.WebhookURL, &e.WebhookSecret, &e.Payload, &e.EventType, &e.Status,
		&e.Attempts, &e.MaxRetries, &e.LastError, &e.NextRetryAt, &e.CreatedAt, &e.DeliveredAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting outbox entry: %w", err)
	}
	return &e, nil
}

func (s *OutboxStore) ResetForRetry(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE webhook_outbox SET status = 'pending', attempts = 0, last_error = NULL, next_retry_at = NOW() WHERE id = $1 AND status = 'failed'`
	tag, err := s.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("resetting outbox entry for retry: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("entry not found or not in failed status")
	}
	return nil
}

func (s *OutboxStore) ResetAllFailedForRequest(ctx context.Context, requestID uuid.UUID) (int64, error) {
	query := `UPDATE webhook_outbox SET status = 'pending', attempts = 0, last_error = NULL, next_retry_at = NOW() WHERE request_id = $1 AND status = 'failed'`
	tag, err := s.db.Pool.Exec(ctx, query, requestID)
	if err != nil {
		return 0, fmt.Errorf("resetting failed entries for request: %w", err)
	}
	return tag.RowsAffected(), nil
}
