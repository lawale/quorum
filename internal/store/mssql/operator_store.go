package mssql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/store"
)

const operatorColumns = `id, username, password_hash, display_name, must_change_password, created_at, updated_at`

type OperatorStore struct {
	db *DB
}

func NewOperatorStore(db *DB) *OperatorStore {
	return &OperatorStore{db: db}
}

func (s *OperatorStore) Create(ctx context.Context, operator *model.Operator) error {
	query := `
		INSERT INTO [quorum].[operators] (` + operatorColumns + `)
		VALUES (@p1, @p2, @p3, @p4, @p5, @p6, @p7)`

	now := time.Now().UTC()
	if operator.ID == uuid.Nil {
		operator.ID = uuid.New()
	}
	operator.CreatedAt = now
	operator.UpdatedAt = now

	_, err := s.db.Pool.ExecContext(ctx, query,
		operator.ID, operator.Username, operator.PasswordHash, operator.DisplayName,
		operator.MustChangePassword, operator.CreatedAt, operator.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting operator: %w", err)
	}

	return nil
}

func (s *OperatorStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Operator, error) {
	query := `SELECT ` + operatorColumns + ` FROM [quorum].[operators] WHERE id = @p1`
	return s.scanOne(ctx, query, id)
}

func (s *OperatorStore) GetByUsername(ctx context.Context, username string) (*model.Operator, error) {
	query := `SELECT ` + operatorColumns + ` FROM [quorum].[operators] WHERE username = @p1`
	return s.scanOne(ctx, query, username)
}

func (s *OperatorStore) List(ctx context.Context, filter store.OperatorFilter) ([]model.Operator, int, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PerPage < 1 {
		filter.PerPage = 20
	}
	offset := (filter.Page - 1) * filter.PerPage

	var total int
	if err := s.db.Pool.QueryRowContext(ctx, "SELECT COUNT(*) FROM [quorum].[operators]").Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting operators: %w", err)
	}

	query := `SELECT ` + operatorColumns + ` FROM [quorum].[operators] ORDER BY created_at DESC OFFSET @p1 ROWS FETCH NEXT @p2 ROWS ONLY`
	rows, err := s.db.Pool.QueryContext(ctx, query, offset, filter.PerPage)
	if err != nil {
		return nil, 0, fmt.Errorf("listing operators: %w", err)
	}
	defer rows.Close()

	var operators []model.Operator
	for rows.Next() {
		var op model.Operator
		if err := rows.Scan(
			&op.ID, &op.Username, &op.PasswordHash, &op.DisplayName,
			&op.MustChangePassword, &op.CreatedAt, &op.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scanning operator: %w", err)
		}
		operators = append(operators, op)
	}

	return operators, total, nil
}

func (s *OperatorStore) Update(ctx context.Context, operator *model.Operator) error {
	query := `
		UPDATE [quorum].[operators] SET username = @p1, password_hash = @p2, display_name = @p3,
		must_change_password = @p4, updated_at = @p5
		WHERE id = @p6`

	operator.UpdatedAt = time.Now().UTC()

	_, err := s.db.Pool.ExecContext(ctx, query,
		operator.Username, operator.PasswordHash, operator.DisplayName,
		operator.MustChangePassword, operator.UpdatedAt, operator.ID,
	)
	if err != nil {
		return fmt.Errorf("updating operator: %w", err)
	}

	return nil
}

func (s *OperatorStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.Pool.ExecContext(ctx, "DELETE FROM [quorum].[operators] WHERE id = @p1", id)
	if err != nil {
		return fmt.Errorf("deleting operator: %w", err)
	}
	return nil
}

func (s *OperatorStore) Count(ctx context.Context) (int, error) {
	var count int
	err := s.db.Pool.QueryRowContext(ctx, "SELECT COUNT(*) FROM [quorum].[operators]").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting operators: %w", err)
	}
	return count, nil
}

func (s *OperatorStore) scanOne(ctx context.Context, query string, args ...any) (*model.Operator, error) {
	op := &model.Operator{}
	err := s.db.Pool.QueryRowContext(ctx, query, args...).Scan(
		&op.ID, &op.Username, &op.PasswordHash, &op.DisplayName,
		&op.MustChangePassword, &op.CreatedAt, &op.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("querying operator: %w", err)
	}
	return op, nil
}
