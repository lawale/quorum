package mssql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/model"
)

const policyColumns = `id, name, request_type, stages, identity_fields, permission_check_url, auto_expire_duration, display_template, created_at, updated_at`

type PolicyStore struct {
	db *DB
}

func NewPolicyStore(db *DB) *PolicyStore {
	return &PolicyStore{db: db}
}

func (s *PolicyStore) Create(ctx context.Context, policy *model.Policy) error {
	query := `
		INSERT INTO [quorum].[policies] (` + policyColumns + `)
		VALUES (@p1, @p2, @p3, @p4, @p5, @p6, @p7, @p8, @p9, @p10)`

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

	_, err = s.db.Pool.ExecContext(ctx, query,
		policy.ID, policy.Name, policy.RequestType, string(stagesJSON),
		nullableString(identityFieldsJSON), policy.PermissionCheckURL, autoExpire, nullableString(policy.DisplayTemplate), policy.CreatedAt, policy.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting policy: %w", err)
	}

	return nil
}

func (s *PolicyStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Policy, error) {
	return s.scanPolicy(ctx, "SELECT "+policyColumns+" FROM [quorum].[policies] WHERE id = @p1", id)
}

func (s *PolicyStore) GetByRequestType(ctx context.Context, requestType string) (*model.Policy, error) {
	return s.scanPolicy(ctx, "SELECT "+policyColumns+" FROM [quorum].[policies] WHERE request_type = @p1", requestType)
}

func (s *PolicyStore) List(ctx context.Context) ([]model.Policy, error) {
	query := `SELECT ` + policyColumns + ` FROM [quorum].[policies] ORDER BY created_at DESC`

	rows, err := s.db.Pool.QueryContext(ctx, query)
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
		UPDATE [quorum].[policies] SET name = @p1, stages = @p2, identity_fields = @p3,
		permission_check_url = @p4, auto_expire_duration = @p5, display_template = @p6, updated_at = @p7
		WHERE id = @p8`

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

	_, err = s.db.Pool.ExecContext(ctx, query,
		policy.Name, string(stagesJSON), nullableString(identityFieldsJSON),
		policy.PermissionCheckURL, autoExpire, nullableString(policy.DisplayTemplate), policy.UpdatedAt, policy.ID,
	)
	if err != nil {
		return fmt.Errorf("updating policy: %w", err)
	}

	return nil
}

func (s *PolicyStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.Pool.ExecContext(ctx, "DELETE FROM [quorum].[policies] WHERE id = @p1", id)
	if err != nil {
		return fmt.Errorf("deleting policy: %w", err)
	}
	return nil
}

func (s *PolicyStore) scanPolicy(ctx context.Context, query string, args ...any) (*model.Policy, error) {
	row := s.db.Pool.QueryRowContext(ctx, query, args...)
	p, err := s.scanSingleRow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying policy: %w", err)
	}
	return p, nil
}

func (s *PolicyStore) scanSingleRow(row *sql.Row) (*model.Policy, error) {
	p := &model.Policy{}
	var stagesJSON, identityFieldsJSON, displayTemplate sql.NullString
	var autoExpire *string

	err := row.Scan(
		&p.ID, &p.Name, &p.RequestType, &stagesJSON,
		&identityFieldsJSON, &p.PermissionCheckURL, &autoExpire, &displayTemplate, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if stagesJSON.Valid {
		if err := json.Unmarshal([]byte(stagesJSON.String), &p.Stages); err != nil {
			return nil, fmt.Errorf("unmarshaling stages: %w", err)
		}
	}
	if identityFieldsJSON.Valid {
		if err := json.Unmarshal([]byte(identityFieldsJSON.String), &p.IdentityFields); err != nil {
			return nil, fmt.Errorf("unmarshaling identity fields: %w", err)
		}
	}
	if displayTemplate.Valid {
		p.DisplayTemplate = json.RawMessage(displayTemplate.String)
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

func (s *PolicyStore) scanPolicyRow(rows *sql.Rows) (*model.Policy, error) {
	p := &model.Policy{}
	var stagesJSON, identityFieldsJSON, displayTemplate sql.NullString
	var autoExpire *string

	err := rows.Scan(
		&p.ID, &p.Name, &p.RequestType, &stagesJSON,
		&identityFieldsJSON, &p.PermissionCheckURL, &autoExpire, &displayTemplate, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning policy: %w", err)
	}

	if stagesJSON.Valid {
		if err := json.Unmarshal([]byte(stagesJSON.String), &p.Stages); err != nil {
			return nil, fmt.Errorf("unmarshaling stages: %w", err)
		}
	}
	if identityFieldsJSON.Valid {
		if err := json.Unmarshal([]byte(identityFieldsJSON.String), &p.IdentityFields); err != nil {
			return nil, fmt.Errorf("unmarshaling identity fields: %w", err)
		}
	}
	if displayTemplate.Valid {
		p.DisplayTemplate = json.RawMessage(displayTemplate.String)
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
