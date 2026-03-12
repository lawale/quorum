package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lawale/quorum/internal/model"
)

type ApprovalStore struct {
	db *DB
}

func NewApprovalStore(db *DB) *ApprovalStore {
	return &ApprovalStore{db: db}
}

func (s *ApprovalStore) Create(ctx context.Context, approval *model.Approval) error {
	query := `
		INSERT INTO approvals (id, request_id, checker_id, decision, comment, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`

	if approval.ID == uuid.Nil {
		approval.ID = uuid.New()
	}
	approval.CreatedAt = time.Now().UTC()

	_, err := s.db.Pool.Exec(ctx, query,
		approval.ID, approval.RequestID, approval.CheckerID,
		approval.Decision, approval.Comment, approval.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting approval: %w", err)
	}

	return nil
}

func (s *ApprovalStore) ListByRequestID(ctx context.Context, requestID uuid.UUID) ([]model.Approval, error) {
	query := `
		SELECT id, request_id, checker_id, decision, comment, created_at
		FROM approvals WHERE request_id = $1 ORDER BY created_at ASC`

	rows, err := s.db.Pool.Query(ctx, query, requestID)
	if err != nil {
		return nil, fmt.Errorf("listing approvals: %w", err)
	}
	defer rows.Close()

	var approvals []model.Approval
	for rows.Next() {
		var a model.Approval
		if err := rows.Scan(&a.ID, &a.RequestID, &a.CheckerID, &a.Decision, &a.Comment, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning approval: %w", err)
		}
		approvals = append(approvals, a)
	}

	return approvals, nil
}

func (s *ApprovalStore) CountByDecision(ctx context.Context, requestID uuid.UUID, decision model.Decision) (int, error) {
	query := `SELECT COUNT(*) FROM approvals WHERE request_id = $1 AND decision = $2`

	var count int
	err := s.db.Pool.QueryRow(ctx, query, requestID, decision).Scan(&count)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("counting approvals: %w", err)
	}

	return count, nil
}

func (s *ApprovalStore) ExistsByChecker(ctx context.Context, requestID uuid.UUID, checkerID string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM approvals WHERE request_id = $1 AND checker_id = $2)`

	var exists bool
	err := s.db.Pool.QueryRow(ctx, query, requestID, checkerID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking approval existence: %w", err)
	}

	return exists, nil
}
