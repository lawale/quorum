package mssql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/auth"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/store"
)

const requestColumns = `id, tenant_id, idempotency_key, type, payload, status, maker_id, eligible_reviewers, metadata, fingerprint, current_stage, expires_at, created_at, updated_at`

type RequestStore struct {
	db *DB
}

func NewRequestStore(db *DB) *RequestStore {
	return &RequestStore{db: db}
}

func (s *RequestStore) Create(ctx context.Context, req *model.Request) error {
	query := `
		INSERT INTO [quorum].[requests] (` + requestColumns + `)
		VALUES (@p1, @p2, @p3, @p4, @p5, @p6, @p7, @p8, @p9, @p10, @p11, @p12, @p13, @p14)`

	now := time.Now().UTC()
	if req.ID == uuid.Nil {
		req.ID = uuid.New()
	}
	req.TenantID = auth.TenantIDFromContext(ctx)
	req.CreatedAt = now
	req.UpdatedAt = now
	if req.Status == "" {
		req.Status = model.StatusPending
	}

	var eligibleJSON []byte
	if len(req.EligibleReviewers) > 0 {
		var err error
		eligibleJSON, err = json.Marshal(req.EligibleReviewers)
		if err != nil {
			return fmt.Errorf("marshaling eligible reviewers: %w", err)
		}
	}

	_, err := s.db.Pool.ExecContext(ctx, query,
		req.ID, req.TenantID, req.IdempotencyKey, req.Type, string(req.Payload), req.Status,
		req.MakerID, nullableString(eligibleJSON), nullableString(req.Metadata), req.Fingerprint,
		req.CurrentStage, req.ExpiresAt, req.CreatedAt, req.UpdatedAt,
	)
	if err != nil {
		if isDuplicateKeyError(err) {
			return store.ErrDuplicateRequest
		}
		return fmt.Errorf("inserting request: %w", err)
	}

	return nil
}

func (s *RequestStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Request, error) {
	tenantID := auth.TenantIDFromContext(ctx)
	if tenantID != "" {
		query := `SELECT ` + requestColumns + ` FROM [quorum].[requests] WHERE id = @p1 AND tenant_id = @p2`
		return s.scanOne(ctx, query, id, tenantID)
	}
	query := `SELECT ` + requestColumns + ` FROM [quorum].[requests] WHERE id = @p1`
	return s.scanOne(ctx, query, id)
}

func (s *RequestStore) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (*model.Request, error) {
	tenantID := auth.TenantIDFromContext(ctx)
	if tenantID != "" {
		query := `SELECT ` + requestColumns + ` FROM [quorum].[requests] WITH (UPDLOCK, ROWLOCK) WHERE id = @p1 AND tenant_id = @p2`
		return s.scanOne(ctx, query, id, tenantID)
	}
	query := `SELECT ` + requestColumns + ` FROM [quorum].[requests] WITH (UPDLOCK, ROWLOCK) WHERE id = @p1`
	return s.scanOne(ctx, query, id)
}

func (s *RequestStore) GetByIdempotencyKey(ctx context.Context, key string) (*model.Request, error) {
	tenantID := auth.TenantIDFromContext(ctx)
	if tenantID != "" {
		query := `SELECT ` + requestColumns + ` FROM [quorum].[requests] WHERE idempotency_key = @p1 AND tenant_id = @p2`
		return s.scanOne(ctx, query, key, tenantID)
	}
	query := `SELECT ` + requestColumns + ` FROM [quorum].[requests] WHERE idempotency_key = @p1`
	return s.scanOne(ctx, query, key)
}

func (s *RequestStore) FindPendingByFingerprint(ctx context.Context, reqType string, fingerprint string) (*model.Request, error) {
	tenantID := auth.TenantIDFromContext(ctx)
	if tenantID != "" {
		query := `SELECT TOP 1 ` + requestColumns + ` FROM [quorum].[requests] WHERE type = @p1 AND fingerprint = @p2 AND status = 'pending' AND tenant_id = @p3`
		return s.scanOne(ctx, query, reqType, fingerprint, tenantID)
	}
	query := `SELECT TOP 1 ` + requestColumns + ` FROM [quorum].[requests] WHERE type = @p1 AND fingerprint = @p2 AND status = 'pending'`
	return s.scanOne(ctx, query, reqType, fingerprint)
}

func (s *RequestStore) List(ctx context.Context, filter store.RequestFilter) ([]model.Request, int, error) {
	var conditions []string
	var args []any
	argIdx := 1

	tenantID := auth.TenantIDFromContext(ctx)
	if tenantID != "" {
		conditions = append(conditions, fmt.Sprintf("tenant_id = @p%d", argIdx))
		args = append(args, tenantID)
		argIdx++
	}

	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = @p%d", argIdx))
		args = append(args, *filter.Status)
		argIdx++
	}
	if filter.Type != nil {
		conditions = append(conditions, fmt.Sprintf("type = @p%d", argIdx))
		args = append(args, *filter.Type)
		argIdx++
	}
	if filter.MakerID != nil {
		conditions = append(conditions, fmt.Sprintf("maker_id = @p%d", argIdx))
		args = append(args, *filter.MakerID)
		argIdx++
	}
	if filter.Search != nil {
		searchPattern := "%" + *filter.Search + "%"
		conditions = append(conditions, fmt.Sprintf("(CAST(id AS NVARCHAR(36)) LIKE @p%d OR type LIKE @p%d OR maker_id LIKE @p%d)", argIdx, argIdx, argIdx))
		args = append(args, searchPattern)
		argIdx++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM [quorum].[requests] %s", where)
	var total int
	if err := s.db.Pool.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
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

	query := fmt.Sprintf(
		`SELECT `+requestColumns+` FROM [quorum].[requests] %s ORDER BY created_at DESC OFFSET @p%d ROWS FETCH NEXT @p%d ROWS ONLY`,
		where, argIdx, argIdx+1,
	)
	args = append(args, offset, filter.PerPage)

	requests, _, err := s.scanMany(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	return requests, total, nil
}

func (s *RequestStore) UpdateStatus(ctx context.Context, id uuid.UUID, status model.RequestStatus) error {
	// CAS guard: only transition from 'pending'. Prevents duplicate terminal
	// transitions when two checkers race to approve/reject the same request.
	query := `UPDATE [quorum].[requests] SET status = @p1, updated_at = @p2 WHERE id = @p3 AND status = 'pending'`
	result, err := s.db.Pool.ExecContext(ctx, query, status, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("updating request status: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return store.ErrStatusConflict
	}
	return nil
}

func (s *RequestStore) UpdateStageAndStatus(ctx context.Context, id uuid.UUID, stage int, status model.RequestStatus) error {
	query := `UPDATE [quorum].[requests] SET current_stage = @p1, status = @p2, updated_at = @p3 WHERE id = @p4 AND status = 'pending'`
	result, err := s.db.Pool.ExecContext(ctx, query, stage, status, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("updating request stage and status: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return store.ErrStatusConflict
	}
	return nil
}

func (s *RequestStore) ListExpired(ctx context.Context) ([]model.Request, error) {
	query := `SELECT ` + requestColumns + ` FROM [quorum].[requests] WHERE status = 'pending' AND expires_at IS NOT NULL AND expires_at <= GETUTCDATE()`
	requests, _, err := s.scanMany(ctx, query)
	return requests, err
}

// scanOne scans a single request row, returning nil if not found.
func (s *RequestStore) scanOne(ctx context.Context, query string, args ...any) (*model.Request, error) {
	req := &model.Request{}
	var eligibleJSON, payload, metadata sql.NullString

	err := s.db.Pool.QueryRowContext(ctx, query, args...).Scan(
		&req.ID, &req.TenantID, &req.IdempotencyKey, &req.Type, &payload, &req.Status,
		&req.MakerID, &eligibleJSON, &metadata, &req.Fingerprint,
		&req.CurrentStage, &req.ExpiresAt, &req.CreatedAt, &req.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying request: %w", err)
	}

	if payload.Valid {
		req.Payload = json.RawMessage(payload.String)
	}
	if metadata.Valid {
		req.Metadata = json.RawMessage(metadata.String)
	}
	if eligibleJSON.Valid {
		if err := json.Unmarshal([]byte(eligibleJSON.String), &req.EligibleReviewers); err != nil {
			return nil, fmt.Errorf("unmarshaling eligible reviewers: %w", err)
		}
	}

	return req, nil
}

// scanMany scans multiple request rows.
func (s *RequestStore) scanMany(ctx context.Context, query string, args ...any) ([]model.Request, int, error) {
	rows, err := s.db.Pool.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("querying requests: %w", err)
	}
	defer rows.Close()

	var requests []model.Request
	for rows.Next() {
		var req model.Request
		var eligibleJSON, payload, metadata sql.NullString

		if err := rows.Scan(
			&req.ID, &req.TenantID, &req.IdempotencyKey, &req.Type, &payload, &req.Status,
			&req.MakerID, &eligibleJSON, &metadata, &req.Fingerprint,
			&req.CurrentStage, &req.ExpiresAt, &req.CreatedAt, &req.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scanning request: %w", err)
		}

		if payload.Valid {
			req.Payload = json.RawMessage(payload.String)
		}
		if metadata.Valid {
			req.Metadata = json.RawMessage(metadata.String)
		}
		if eligibleJSON.Valid {
			if err := json.Unmarshal([]byte(eligibleJSON.String), &req.EligibleReviewers); err != nil {
				return nil, 0, fmt.Errorf("unmarshaling eligible reviewers: %w", err)
			}
		}

		requests = append(requests, req)
	}

	return requests, len(requests), nil
}
