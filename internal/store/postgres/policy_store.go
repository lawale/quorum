package postgres

import (
	"context"
	"encoding/json"
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

type PolicyStore struct {
	db *DB
}

func NewPolicyStore(db *DB) *PolicyStore {
	return &PolicyStore{db: db}
}

func (s *PolicyStore) Create(ctx context.Context, policy *model.Policy) error {
	query := `
		INSERT INTO policies (id, tenant_id, name, request_type, stages, identity_fields, dynamic_authorization_url, dynamic_authorization_secret, auto_expire_duration, display_template, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	now := time.Now().UTC()
	if policy.ID == uuid.Nil {
		policy.ID = uuid.New()
	}
	policy.TenantID = auth.TenantIDFromContext(ctx)
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

	_, err = s.db.Pool.Exec(ctx, query,
		policy.ID, policy.TenantID, policy.Name, policy.RequestType, stagesJSON,
		identityFieldsJSON, policy.DynamicAuthorizationURL, policy.DynamicAuthorizationSecret, policy.AutoExpireDuration, policy.DisplayTemplate, policy.CreatedAt, policy.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting policy: %w", err)
	}

	return nil
}

func (s *PolicyStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Policy, error) {
	query := "SELECT id, tenant_id, name, request_type, stages, identity_fields, dynamic_authorization_url, dynamic_authorization_secret, auto_expire_duration, display_template, created_at, updated_at FROM policies WHERE id = $1"
	tenant := auth.TenantIDFromContext(ctx)
	if tenant != "" {
		query += " AND tenant_id = $2"
		return s.scanPolicy(ctx, query, id, tenant)
	}
	return s.scanPolicy(ctx, query, id)
}

func (s *PolicyStore) GetByRequestType(ctx context.Context, requestType string) (*model.Policy, error) {
	query := "SELECT id, tenant_id, name, request_type, stages, identity_fields, dynamic_authorization_url, dynamic_authorization_secret, auto_expire_duration, display_template, created_at, updated_at FROM policies WHERE request_type = $1"
	tenant := auth.TenantIDFromContext(ctx)
	if tenant != "" {
		query += " AND tenant_id = $2"
		return s.scanPolicy(ctx, query, requestType, tenant)
	}
	return s.scanPolicy(ctx, query, requestType)
}

func (s *PolicyStore) List(ctx context.Context, filter store.PolicyFilter) ([]model.Policy, int, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PerPage < 1 {
		filter.PerPage = 20
	}
	offset := (filter.Page - 1) * filter.PerPage

	tenant := auth.TenantIDFromContext(ctx)

	var where string
	var args []any
	argIdx := 1
	if tenant != "" {
		where = fmt.Sprintf("WHERE tenant_id = $%d", argIdx)
		args = append(args, tenant)
		argIdx++
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM policies %s", where)
	var total int
	if err := s.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting policies: %w", err)
	}

	dataQuery := fmt.Sprintf(
		`SELECT id, tenant_id, name, request_type, stages, identity_fields, dynamic_authorization_url, dynamic_authorization_secret, auto_expire_duration, display_template, created_at, updated_at FROM policies %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	args = append(args, filter.PerPage, offset)

	rows, err := s.db.Pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing policies: %w", err)
	}
	defer rows.Close()

	var policies []model.Policy
	for rows.Next() {
		p, err := s.scanPolicyRow(rows)
		if err != nil {
			return nil, 0, err
		}
		policies = append(policies, *p)
	}

	return policies, total, nil
}

func (s *PolicyStore) Update(ctx context.Context, policy *model.Policy) error {
	query := `
		UPDATE policies SET name = $1, stages = $2, identity_fields = $3,
		dynamic_authorization_url = $4, dynamic_authorization_secret = $5, auto_expire_duration = $6, display_template = $7, updated_at = $8
		WHERE id = $9`

	policy.UpdatedAt = time.Now().UTC()

	stagesJSON, err := json.Marshal(policy.Stages)
	if err != nil {
		return fmt.Errorf("marshaling stages: %w", err)
	}

	identityFieldsJSON, err := json.Marshal(policy.IdentityFields)
	if err != nil {
		return fmt.Errorf("marshaling identity fields: %w", err)
	}

	args := []any{
		policy.Name, stagesJSON, identityFieldsJSON,
		policy.DynamicAuthorizationURL, policy.DynamicAuthorizationSecret, policy.AutoExpireDuration, policy.DisplayTemplate, policy.UpdatedAt, policy.ID,
	}

	tenant := auth.TenantIDFromContext(ctx)
	if tenant != "" {
		query += " AND tenant_id = $10"
		args = append(args, tenant)
	}

	_, err = s.db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("updating policy: %w", err)
	}

	return nil
}

func (s *PolicyStore) Delete(ctx context.Context, id uuid.UUID) error {
	query := "DELETE FROM policies WHERE id = $1"
	var tag pgconn.CommandTag
	var err error
	tenant := auth.TenantIDFromContext(ctx)
	if tenant != "" {
		query += " AND tenant_id = $2"
		tag, err = s.db.Pool.Exec(ctx, query, id, tenant)
	} else {
		tag, err = s.db.Pool.Exec(ctx, query, id)
	}
	if err != nil {
		return fmt.Errorf("deleting policy: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
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
	var autoExpire *time.Duration

	err := row.Scan(
		&p.ID, &p.TenantID, &p.Name, &p.RequestType, &stagesJSON,
		&identityFieldsJSON, &p.DynamicAuthorizationURL, &p.DynamicAuthorizationSecret, &autoExpire, &p.DisplayTemplate, &p.CreatedAt, &p.UpdatedAt,
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

	p.AutoExpireDuration = autoExpire

	return p, nil
}

func (s *PolicyStore) scanPolicyRow(rows pgx.Rows) (*model.Policy, error) {
	p := &model.Policy{}
	var stagesJSON, identityFieldsJSON []byte
	var autoExpire *time.Duration

	err := rows.Scan(
		&p.ID, &p.TenantID, &p.Name, &p.RequestType, &stagesJSON,
		&identityFieldsJSON, &p.DynamicAuthorizationURL, &p.DynamicAuthorizationSecret, &autoExpire, &p.DisplayTemplate, &p.CreatedAt, &p.UpdatedAt,
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

	p.AutoExpireDuration = autoExpire

	return p, nil
}
