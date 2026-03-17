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

type ApprovalStore struct {
	db *DB
}

func NewApprovalStore(db *DB) *ApprovalStore {
	return &ApprovalStore{db: db}
}

func (s *ApprovalStore) Create(ctx context.Context, approval *model.Approval) error {
	query := `
		INSERT INTO approvals (id, request_id, checker_id, decision, stage_index, comment, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	if approval.ID == uuid.Nil {
		approval.ID = uuid.New()
	}
	approval.CreatedAt = time.Now().UTC()

	_, err := s.db.Pool.Exec(ctx, query,
		approval.ID, approval.RequestID, approval.CheckerID,
		approval.Decision, approval.StageIndex, approval.Comment, approval.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return store.ErrDuplicateApproval
		}
		return fmt.Errorf("inserting approval: %w", err)
	}

	return nil
}

func (s *ApprovalStore) ListByRequestID(ctx context.Context, requestID uuid.UUID) ([]model.Approval, error) {
	args := []any{requestID}
	var query string

	if tenant := auth.TenantIDFromContext(ctx); tenant != "" {
		query = `
			SELECT a.id, a.request_id, a.checker_id, a.decision, a.stage_index, a.comment, a.created_at
			FROM approvals a
			JOIN requests r ON r.id = a.request_id
			WHERE a.request_id = $1 AND r.tenant_id = $2
			ORDER BY a.created_at ASC`
		args = append(args, tenant)
	} else {
		query = `
			SELECT id, request_id, checker_id, decision, stage_index, comment, created_at
			FROM approvals WHERE request_id = $1
			ORDER BY created_at ASC`
	}

	rows, err := s.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing approvals: %w", err)
	}
	defer rows.Close()

	var approvals []model.Approval
	for rows.Next() {
		var a model.Approval
		if err := rows.Scan(&a.ID, &a.RequestID, &a.CheckerID, &a.Decision, &a.StageIndex, &a.Comment, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning approval: %w", err)
		}
		approvals = append(approvals, a)
	}

	return approvals, nil
}

func (s *ApprovalStore) CountByDecisionAndStage(ctx context.Context, requestID uuid.UUID, decision model.Decision, stageIndex int) (int, error) {
	query := `SELECT COUNT(*) FROM approvals WHERE request_id = $1 AND decision = $2 AND stage_index = $3`

	var count int
	err := s.db.Pool.QueryRow(ctx, query, requestID, decision, stageIndex).Scan(&count)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("counting approvals: %w", err)
	}

	return count, nil
}

func (s *ApprovalStore) ExistsByCheckerAndStage(ctx context.Context, requestID uuid.UUID, checkerID string, stageIndex int) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM approvals WHERE request_id = $1 AND checker_id = $2 AND stage_index = $3)`

	var exists bool
	err := s.db.Pool.QueryRow(ctx, query, requestID, checkerID, stageIndex).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking approval existence: %w", err)
	}

	return exists, nil
}
