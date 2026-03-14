package mssql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
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
		INSERT INTO [quorum].[approvals] (id, request_id, checker_id, decision, stage_index, comment, created_at)
		VALUES (@p1, @p2, @p3, @p4, @p5, @p6, @p7)`

	if approval.ID == uuid.Nil {
		approval.ID = uuid.New()
	}
	approval.CreatedAt = time.Now().UTC()

	_, err := s.db.Pool.ExecContext(ctx, query,
		approval.ID, approval.RequestID, approval.CheckerID,
		approval.Decision, approval.StageIndex, approval.Comment, approval.CreatedAt,
	)
	if err != nil {
		if isDuplicateKeyError(err) {
			return store.ErrDuplicateApproval
		}
		return fmt.Errorf("inserting approval: %w", err)
	}

	return nil
}

// isDuplicateKeyError returns true for MSSQL unique constraint violations
// (error numbers 2627 and 2601).
func isDuplicateKeyError(err error) bool {
	// go-mssqldb wraps errors; check the message for the SQL Server error numbers
	// as a portable fallback.
	var unwrapped interface{ SQLErrorNumber() int32 }
	if errors.As(err, &unwrapped) {
		num := unwrapped.SQLErrorNumber()
		return num == 2627 || num == 2601
	}
	msg := err.Error()
	return strings.Contains(msg, "Violation of UNIQUE KEY constraint") ||
		strings.Contains(msg, "Cannot insert duplicate key")
}

func (s *ApprovalStore) ListByRequestID(ctx context.Context, requestID uuid.UUID) ([]model.Approval, error) {
	query := `
		SELECT id, request_id, checker_id, decision, stage_index, comment, created_at
		FROM [quorum].[approvals] WHERE request_id = @p1 ORDER BY created_at ASC`

	rows, err := s.db.Pool.QueryContext(ctx, query, requestID)
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
	query := `SELECT COUNT(*) FROM [quorum].[approvals] WHERE request_id = @p1 AND decision = @p2 AND stage_index = @p3`

	var count int
	err := s.db.Pool.QueryRowContext(ctx, query, requestID, decision, stageIndex).Scan(&count)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, fmt.Errorf("counting approvals: %w", err)
	}

	return count, nil
}

func (s *ApprovalStore) ExistsByCheckerAndStage(ctx context.Context, requestID uuid.UUID, checkerID string, stageIndex int) (bool, error) {
	query := `SELECT CASE WHEN EXISTS(SELECT 1 FROM [quorum].[approvals] WHERE request_id = @p1 AND checker_id = @p2 AND stage_index = @p3) THEN 1 ELSE 0 END`

	var exists int
	err := s.db.Pool.QueryRowContext(ctx, query, requestID, checkerID, stageIndex).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking approval existence: %w", err)
	}

	return exists == 1, nil
}
