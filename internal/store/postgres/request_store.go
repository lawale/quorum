package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/store"
)

const requestColumns = `id, idempotency_key, type, payload, status, maker_id, callback_url, eligible_reviewers, metadata, fingerprint, current_stage, expires_at, created_at, updated_at`

type RequestStore struct {
	db *DB
}

func NewRequestStore(db *DB) *RequestStore {
	return &RequestStore{db: db}
}

func (s *RequestStore) Create(ctx context.Context, req *model.Request) error {
	query := `
		INSERT INTO requests (` + requestColumns + `)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`

	now := time.Now().UTC()
	if req.ID == uuid.Nil {
		req.ID = uuid.New()
	}
	req.CreatedAt = now
	req.UpdatedAt = now
	if req.Status == "" {
		req.Status = model.StatusPending
	}

	var eligibleJSON []byte
	if len(req.EligibleReviewers) > 0 {
		eligibleJSON, _ = json.Marshal(req.EligibleReviewers)
	}

	_, err := s.db.Pool.Exec(ctx, query,
		req.ID, req.IdempotencyKey, req.Type, req.Payload, req.Status,
		req.MakerID, req.CallbackURL, eligibleJSON, req.Metadata, req.Fingerprint,
		req.CurrentStage, req.ExpiresAt, req.CreatedAt, req.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting request: %w", err)
	}

	return nil
}

func (s *RequestStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Request, error) {
	query := `SELECT ` + requestColumns + ` FROM requests WHERE id = $1`
	return s.scanOne(ctx, query, id)
}

func (s *RequestStore) GetByIdempotencyKey(ctx context.Context, key string) (*model.Request, error) {
	query := `SELECT ` + requestColumns + ` FROM requests WHERE idempotency_key = $1`
	return s.scanOne(ctx, query, key)
}

func (s *RequestStore) FindPendingByFingerprint(ctx context.Context, reqType string, fingerprint string) (*model.Request, error) {
	query := `SELECT ` + requestColumns + ` FROM requests WHERE type = $1 AND fingerprint = $2 AND status = 'pending' LIMIT 1`
	return s.scanOne(ctx, query, reqType, fingerprint)
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

	query := fmt.Sprintf(`SELECT `+requestColumns+` FROM requests %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)
	args = append(args, filter.PerPage, offset)

	return s.scanMany(ctx, query, args...)
}

func (s *RequestStore) UpdateStatus(ctx context.Context, id uuid.UUID, status model.RequestStatus) error {
	// CAS guard: only transition from 'pending'. Prevents duplicate terminal
	// transitions when two checkers race to approve/reject the same request.
	query := `UPDATE requests SET status = $1, updated_at = $2 WHERE id = $3 AND status = 'pending'`
	tag, err := s.db.Pool.Exec(ctx, query, status, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("updating request status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return store.ErrStatusConflict
	}
	return nil
}

func (s *RequestStore) UpdateStageAndStatus(ctx context.Context, id uuid.UUID, stage int, status model.RequestStatus) error {
	query := `UPDATE requests SET current_stage = $1, status = $2, updated_at = $3 WHERE id = $4 AND status = 'pending'`
	tag, err := s.db.Pool.Exec(ctx, query, stage, status, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("updating request stage and status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return store.ErrStatusConflict
	}
	return nil
}

func (s *RequestStore) ListExpired(ctx context.Context) ([]model.Request, error) {
	query := `SELECT ` + requestColumns + ` FROM requests WHERE status = 'pending' AND expires_at IS NOT NULL AND expires_at <= NOW()`
	requests, _, err := s.scanMany(ctx, query)
	return requests, err
}

// scanOne scans a single request row, returning nil if not found.
func (s *RequestStore) scanOne(ctx context.Context, query string, args ...any) (*model.Request, error) {
	req := &model.Request{}
	var eligibleJSON []byte

	err := s.db.Pool.QueryRow(ctx, query, args...).Scan(
		&req.ID, &req.IdempotencyKey, &req.Type, &req.Payload, &req.Status,
		&req.MakerID, &req.CallbackURL, &eligibleJSON, &req.Metadata, &req.Fingerprint,
		&req.CurrentStage, &req.ExpiresAt, &req.CreatedAt, &req.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("querying request: %w", err)
	}

	if eligibleJSON != nil {
		if err := json.Unmarshal(eligibleJSON, &req.EligibleReviewers); err != nil {
			return nil, fmt.Errorf("unmarshaling eligible reviewers: %w", err)
		}
	}

	return req, nil
}

// scanMany scans multiple request rows.
func (s *RequestStore) scanMany(ctx context.Context, query string, args ...any) ([]model.Request, int, error) {
	rows, err := s.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("querying requests: %w", err)
	}
	defer rows.Close()

	var requests []model.Request
	for rows.Next() {
		var req model.Request
		var eligibleJSON []byte

		if err := rows.Scan(
			&req.ID, &req.IdempotencyKey, &req.Type, &req.Payload, &req.Status,
			&req.MakerID, &req.CallbackURL, &eligibleJSON, &req.Metadata, &req.Fingerprint,
			&req.CurrentStage, &req.ExpiresAt, &req.CreatedAt, &req.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scanning request: %w", err)
		}

		if eligibleJSON != nil {
			if err := json.Unmarshal(eligibleJSON, &req.EligibleReviewers); err != nil {
				return nil, 0, fmt.Errorf("unmarshaling eligible reviewers: %w", err)
			}
		}

		requests = append(requests, req)
	}

	return requests, len(requests), nil
}
