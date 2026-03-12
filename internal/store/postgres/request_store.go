package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/store"
)

type RequestStore struct {
	db *DB
}

func NewRequestStore(db *DB) *RequestStore {
	return &RequestStore{db: db}
}

func (s *RequestStore) Create(ctx context.Context, req *model.Request) error {
	query := `
		INSERT INTO requests (id, idempotency_key, type, payload, status, maker_id, callback_url, metadata, fingerprint, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	now := time.Now().UTC()
	if req.ID == uuid.Nil {
		req.ID = uuid.New()
	}
	req.CreatedAt = now
	req.UpdatedAt = now
	if req.Status == "" {
		req.Status = model.StatusPending
	}

	_, err := s.db.Pool.Exec(ctx, query,
		req.ID, req.IdempotencyKey, req.Type, req.Payload, req.Status,
		req.MakerID, req.CallbackURL, req.Metadata, req.Fingerprint,
		req.ExpiresAt, req.CreatedAt, req.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting request: %w", err)
	}

	return nil
}

func (s *RequestStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Request, error) {
	query := `
		SELECT id, idempotency_key, type, payload, status, maker_id, callback_url, metadata, fingerprint, expires_at, created_at, updated_at
		FROM requests WHERE id = $1`

	req := &model.Request{}
	err := s.db.Pool.QueryRow(ctx, query, id).Scan(
		&req.ID, &req.IdempotencyKey, &req.Type, &req.Payload, &req.Status,
		&req.MakerID, &req.CallbackURL, &req.Metadata, &req.Fingerprint,
		&req.ExpiresAt, &req.CreatedAt, &req.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying request: %w", err)
	}

	return req, nil
}

func (s *RequestStore) GetByIdempotencyKey(ctx context.Context, key string) (*model.Request, error) {
	query := `
		SELECT id, idempotency_key, type, payload, status, maker_id, callback_url, metadata, fingerprint, expires_at, created_at, updated_at
		FROM requests WHERE idempotency_key = $1`

	req := &model.Request{}
	err := s.db.Pool.QueryRow(ctx, query, key).Scan(
		&req.ID, &req.IdempotencyKey, &req.Type, &req.Payload, &req.Status,
		&req.MakerID, &req.CallbackURL, &req.Metadata, &req.Fingerprint,
		&req.ExpiresAt, &req.CreatedAt, &req.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying request by idempotency key: %w", err)
	}

	return req, nil
}

func (s *RequestStore) FindPendingByFingerprint(ctx context.Context, reqType string, fingerprint string) (*model.Request, error) {
	query := `
		SELECT id, idempotency_key, type, payload, status, maker_id, callback_url, metadata, fingerprint, expires_at, created_at, updated_at
		FROM requests WHERE type = $1 AND fingerprint = $2 AND status = 'pending'
		LIMIT 1`

	req := &model.Request{}
	err := s.db.Pool.QueryRow(ctx, query, reqType, fingerprint).Scan(
		&req.ID, &req.IdempotencyKey, &req.Type, &req.Payload, &req.Status,
		&req.MakerID, &req.CallbackURL, &req.Metadata, &req.Fingerprint,
		&req.ExpiresAt, &req.CreatedAt, &req.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying request by fingerprint: %w", err)
	}

	return req, nil
}

func (s *RequestStore) List(ctx context.Context, filter store.RequestFilter) ([]model.Request, int, error) {
	var conditions []string
	var args []any
	argIdx := 1

	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *filter.Status)
		argIdx++
	}
	if filter.Type != nil {
		conditions = append(conditions, fmt.Sprintf("type = $%d", argIdx))
		args = append(args, *filter.Type)
		argIdx++
	}
	if filter.MakerID != nil {
		conditions = append(conditions, fmt.Sprintf("maker_id = $%d", argIdx))
		args = append(args, *filter.MakerID)
		argIdx++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM requests %s", where)
	var total int
	if err := s.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting requests: %w", err)
	}

	// Pagination
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PerPage < 1 {
		filter.PerPage = 20
	}
	offset := (filter.Page - 1) * filter.PerPage

	query := fmt.Sprintf(`
		SELECT id, idempotency_key, type, payload, status, maker_id, callback_url, metadata, fingerprint, expires_at, created_at, updated_at
		FROM requests %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)
	args = append(args, filter.PerPage, offset)

	rows, err := s.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing requests: %w", err)
	}
	defer rows.Close()

	var requests []model.Request
	for rows.Next() {
		var req model.Request
		if err := rows.Scan(
			&req.ID, &req.IdempotencyKey, &req.Type, &req.Payload, &req.Status,
			&req.MakerID, &req.CallbackURL, &req.Metadata, &req.Fingerprint,
			&req.ExpiresAt, &req.CreatedAt, &req.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scanning request: %w", err)
		}
		requests = append(requests, req)
	}

	return requests, total, nil
}

func (s *RequestStore) UpdateStatus(ctx context.Context, id uuid.UUID, status model.RequestStatus) error {
	query := `UPDATE requests SET status = $1, updated_at = $2 WHERE id = $3`
	_, err := s.db.Pool.Exec(ctx, query, status, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("updating request status: %w", err)
	}
	return nil
}

func (s *RequestStore) ListExpired(ctx context.Context) ([]model.Request, error) {
	query := `
		SELECT id, idempotency_key, type, payload, status, maker_id, callback_url, metadata, fingerprint, expires_at, created_at, updated_at
		FROM requests WHERE status = 'pending' AND expires_at IS NOT NULL AND expires_at <= NOW()`

	rows, err := s.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing expired requests: %w", err)
	}
	defer rows.Close()

	var requests []model.Request
	for rows.Next() {
		var req model.Request
		if err := rows.Scan(
			&req.ID, &req.IdempotencyKey, &req.Type, &req.Payload, &req.Status,
			&req.MakerID, &req.CallbackURL, &req.Metadata, &req.Fingerprint,
			&req.ExpiresAt, &req.CreatedAt, &req.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning expired request: %w", err)
		}
		requests = append(requests, req)
	}

	return requests, nil
}
