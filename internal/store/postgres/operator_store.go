package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
		INSERT INTO operators (` + operatorColumns + `)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	now := time.Now().UTC()
	if operator.ID == uuid.Nil {
		operator.ID = uuid.New()
	}
	operator.CreatedAt = now
	operator.UpdatedAt = now

	_, err := s.db.Pool.Exec(ctx, query,
		operator.ID, operator.Username, operator.PasswordHash, operator.DisplayName,
		operator.MustChangePassword, operator.CreatedAt, operator.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting operator: %w", err)
	}

	return nil
}

func (s *OperatorStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Operator, error) {
	query := `SELECT ` + operatorColumns + ` FROM operators WHERE id = $1`
	return s.scanOne(ctx, query, id)
}

func (s *OperatorStore) GetByUsername(ctx context.Context, username string) (*model.Operator, error) {
	query := `SELECT ` + operatorColumns + ` FROM operators WHERE username = $1`
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
	if err := s.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM operators").Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting operators: %w", err)
	}

	query := `SELECT ` + operatorColumns + ` FROM operators ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	rows, err := s.db.Pool.Query(ctx, query, filter.PerPage, offset)
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
		UPDATE operators SET username = $1, password_hash = $2, display_name = $3,
		must_change_password = $4, updated_at = $5
		WHERE id = $6`

	operator.UpdatedAt = time.Now().UTC()

	_, err := s.db.Pool.Exec(ctx, query,
		operator.Username, operator.PasswordHash, operator.DisplayName,
		operator.MustChangePassword, operator.UpdatedAt, operator.ID,
	)
	if err != nil {
		return fmt.Errorf("updating operator: %w", err)
	}

	return nil
}

func (s *OperatorStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.Pool.Exec(ctx, "DELETE FROM operators WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("deleting operator: %w", err)
	}
	return nil
}

func (s *OperatorStore) Count(ctx context.Context) (int, error) {
	var count int
	err := s.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM operators").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting operators: %w", err)
	}
	return count, nil
}

func (s *OperatorStore) scanOne(ctx context.Context, query string, args ...any) (*model.Operator, error) {
	op := &model.Operator{}
	err := s.db.Pool.QueryRow(ctx, query, args...).Scan(
		&op.ID, &op.Username, &op.PasswordHash, &op.DisplayName,
		&op.MustChangePassword, &op.CreatedAt, &op.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("querying operator: %w", err)
	}
	return op, nil
}
