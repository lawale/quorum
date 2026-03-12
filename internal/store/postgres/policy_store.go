package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/wale/maker-checker/internal/model"
)

type PolicyStore struct {
	db *DB
}

func NewPolicyStore(db *DB) *PolicyStore {
	return &PolicyStore{db: db}
}

func (s *PolicyStore) Create(ctx context.Context, policy *model.Policy) error {
	query := `
		INSERT INTO policies (id, name, request_type, required_approvals, allowed_checker_roles, rejection_policy, max_checkers, identity_fields, auto_expire_duration, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

	now := time.Now().UTC()
	if policy.ID == uuid.Nil {
		policy.ID = uuid.New()
	}
	policy.CreatedAt = now
	policy.UpdatedAt = now

	identityFieldsJSON, err := json.Marshal(policy.IdentityFields)
	if err != nil {
		return fmt.Errorf("marshaling identity fields: %w", err)
	}

	var autoExpire *string
	if policy.AutoExpireDuration != nil {
		s := policy.AutoExpireDuration.String()
		autoExpire = &s
	}

	_, err = s.db.Pool.Exec(ctx, query,
		policy.ID, policy.Name, policy.RequestType, policy.RequiredApprovals,
		policy.AllowedCheckerRoles, policy.RejectionPolicy, policy.MaxCheckers,
		identityFieldsJSON, autoExpire, policy.CreatedAt, policy.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting policy: %w", err)
	}

	return nil
}

func (s *PolicyStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Policy, error) {
	return s.scanPolicy(ctx, "SELECT id, name, request_type, required_approvals, allowed_checker_roles, rejection_policy, max_checkers, identity_fields, auto_expire_duration, created_at, updated_at FROM policies WHERE id = $1", id)
}

func (s *PolicyStore) GetByRequestType(ctx context.Context, requestType string) (*model.Policy, error) {
	return s.scanPolicy(ctx, "SELECT id, name, request_type, required_approvals, allowed_checker_roles, rejection_policy, max_checkers, identity_fields, auto_expire_duration, created_at, updated_at FROM policies WHERE request_type = $1", requestType)
}

func (s *PolicyStore) List(ctx context.Context) ([]model.Policy, error) {
	query := `SELECT id, name, request_type, required_approvals, allowed_checker_roles, rejection_policy, max_checkers, identity_fields, auto_expire_duration, created_at, updated_at FROM policies ORDER BY created_at DESC`

	rows, err := s.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing policies: %w", err)
	}
	defer rows.Close()

	var policies []model.Policy
	for rows.Next() {
		p, err := s.scanPolicyRow(rows)
		if err != nil {
			return nil, err
		}
		policies = append(policies, *p)
	}

	return policies, nil
}

func (s *PolicyStore) Update(ctx context.Context, policy *model.Policy) error {
	query := `
		UPDATE policies SET name = $1, required_approvals = $2, allowed_checker_roles = $3,
		rejection_policy = $4, max_checkers = $5, identity_fields = $6, auto_expire_duration = $7, updated_at = $8
		WHERE id = $9`

	policy.UpdatedAt = time.Now().UTC()

	identityFieldsJSON, err := json.Marshal(policy.IdentityFields)
	if err != nil {
		return fmt.Errorf("marshaling identity fields: %w", err)
	}

	var autoExpire *string
	if policy.AutoExpireDuration != nil {
		s := policy.AutoExpireDuration.String()
		autoExpire = &s
	}

	_, err = s.db.Pool.Exec(ctx, query,
		policy.Name, policy.RequiredApprovals, policy.AllowedCheckerRoles,
		policy.RejectionPolicy, policy.MaxCheckers, identityFieldsJSON,
		autoExpire, policy.UpdatedAt, policy.ID,
	)
	if err != nil {
		return fmt.Errorf("updating policy: %w", err)
	}

	return nil
}

func (s *PolicyStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.Pool.Exec(ctx, "DELETE FROM policies WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("deleting policy: %w", err)
	}
	return nil
}

func (s *PolicyStore) scanPolicy(ctx context.Context, query string, args ...any) (*model.Policy, error) {
	row := s.db.Pool.QueryRow(ctx, query, args...)
	p, err := s.scanSingleRow(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying policy: %w", err)
	}
	return p, nil
}

type scannable interface {
	Scan(dest ...any) error
}

func (s *PolicyStore) scanSingleRow(row pgx.Row) (*model.Policy, error) {
	p := &model.Policy{}
	var identityFieldsJSON []byte
	var autoExpire *string

	err := row.Scan(
		&p.ID, &p.Name, &p.RequestType, &p.RequiredApprovals,
		&p.AllowedCheckerRoles, &p.RejectionPolicy, &p.MaxCheckers,
		&identityFieldsJSON, &autoExpire, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if identityFieldsJSON != nil {
		if err := json.Unmarshal(identityFieldsJSON, &p.IdentityFields); err != nil {
			return nil, fmt.Errorf("unmarshaling identity fields: %w", err)
		}
	}

	if autoExpire != nil {
		d, err := time.ParseDuration(*autoExpire)
		if err != nil {
			return nil, fmt.Errorf("parsing auto expire duration: %w", err)
		}
		p.AutoExpireDuration = &d
	}

	return p, nil
}

func (s *PolicyStore) scanPolicyRow(rows pgx.Rows) (*model.Policy, error) {
	p := &model.Policy{}
	var identityFieldsJSON []byte
	var autoExpire *string

	err := rows.Scan(
		&p.ID, &p.Name, &p.RequestType, &p.RequiredApprovals,
		&p.AllowedCheckerRoles, &p.RejectionPolicy, &p.MaxCheckers,
		&identityFieldsJSON, &autoExpire, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning policy: %w", err)
	}

	if identityFieldsJSON != nil {
		if err := json.Unmarshal(identityFieldsJSON, &p.IdentityFields); err != nil {
			return nil, fmt.Errorf("unmarshaling identity fields: %w", err)
		}
	}

	if autoExpire != nil {
		d, err := time.ParseDuration(*autoExpire)
		if err != nil {
			return nil, fmt.Errorf("parsing auto expire duration: %w", err)
		}
		p.AutoExpireDuration = &d
	}

	return p, nil
}
