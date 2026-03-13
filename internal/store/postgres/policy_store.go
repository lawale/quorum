package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lawale/quorum/internal/model"
)

type PolicyStore struct {
	db *DB
}

func NewPolicyStore(db *DB) *PolicyStore {
	return &PolicyStore{db: db}
}

func (s *PolicyStore) Create(ctx context.Context, policy *model.Policy) error {
	query := `
		INSERT INTO policies (id, name, request_type, stages, identity_fields, permission_check_url, auto_expire_duration, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	now := time.Now().UTC()
	if policy.ID == uuid.Nil {
		policy.ID = uuid.New()
	}
	policy.CreatedAt = now
	policy.UpdatedAt = now

	stagesJSON, err := json.Marshal(policy.Stages)
	if err != nil {
		return fmt.Errorf("marshaling stages: %w", err)
	}

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
		policy.ID, policy.Name, policy.RequestType, stagesJSON,
		identityFieldsJSON, policy.PermissionCheckURL, autoExpire, policy.CreatedAt, policy.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting policy: %w", err)
	}

	return nil
}

func (s *PolicyStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Policy, error) {
	return s.scanPolicy(ctx, "SELECT id, name, request_type, stages, identity_fields, permission_check_url, auto_expire_duration, created_at, updated_at FROM policies WHERE id = $1", id)
}

func (s *PolicyStore) GetByRequestType(ctx context.Context, requestType string) (*model.Policy, error) {
	return s.scanPolicy(ctx, "SELECT id, name, request_type, stages, identity_fields, permission_check_url, auto_expire_duration, created_at, updated_at FROM policies WHERE request_type = $1", requestType)
}

func (s *PolicyStore) List(ctx context.Context) ([]model.Policy, error) {
	query := `SELECT id, name, request_type, stages, identity_fields, permission_check_url, auto_expire_duration, created_at, updated_at FROM policies ORDER BY created_at DESC`

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
		UPDATE policies SET name = $1, stages = $2, identity_fields = $3,
		permission_check_url = $4, auto_expire_duration = $5, updated_at = $6
		WHERE id = $7`

	policy.UpdatedAt = time.Now().UTC()

	stagesJSON, err := json.Marshal(policy.Stages)
	if err != nil {
		return fmt.Errorf("marshaling stages: %w", err)
	}

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
		policy.Name, stagesJSON, identityFieldsJSON,
		policy.PermissionCheckURL, autoExpire, policy.UpdatedAt, policy.ID,
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
		if errors.Is(err, pgx.ErrNoRows) {
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
	var stagesJSON, identityFieldsJSON []byte
	var autoExpire *string

	err := row.Scan(
		&p.ID, &p.Name, &p.RequestType, &stagesJSON,
		&identityFieldsJSON, &p.PermissionCheckURL, &autoExpire, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if stagesJSON != nil {
		if err := json.Unmarshal(stagesJSON, &p.Stages); err != nil {
			return nil, fmt.Errorf("unmarshaling stages: %w", err)
		}
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
	var stagesJSON, identityFieldsJSON []byte
	var autoExpire *string

	err := rows.Scan(
		&p.ID, &p.Name, &p.RequestType, &stagesJSON,
		&identityFieldsJSON, &p.PermissionCheckURL, &autoExpire, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning policy: %w", err)
	}

	if stagesJSON != nil {
		if err := json.Unmarshal(stagesJSON, &p.Stages); err != nil {
			return nil, fmt.Errorf("unmarshaling stages: %w", err)
		}
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
