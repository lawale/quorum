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

type TenantStore struct {
	db *DB
}

func NewTenantStore(db *DB) *TenantStore {
	return &TenantStore{db: db}
}

func (s *TenantStore) Create(ctx context.Context, tenant *model.Tenant) error {
	query := `INSERT INTO tenants (id, slug, name, created_at, updated_at) VALUES ($1, $2, $3, $4, $5)`
	now := time.Now().UTC()
	if tenant.ID == uuid.Nil {
		tenant.ID = uuid.New()
	}
	tenant.CreatedAt = now
	tenant.UpdatedAt = now
	_, err := s.db.Pool.Exec(ctx, query, tenant.ID, tenant.Slug, tenant.Name, tenant.CreatedAt, tenant.UpdatedAt)
	if err != nil {
		return fmt.Errorf("inserting tenant: %w", err)
	}
	return nil
}

func (s *TenantStore) GetBySlug(ctx context.Context, slug string) (*model.Tenant, error) {
	query := `SELECT id, slug, name, created_at, updated_at FROM tenants WHERE slug = $1`
	t := &model.Tenant{}
	err := s.db.Pool.QueryRow(ctx, query, slug).Scan(&t.ID, &t.Slug, &t.Name, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("querying tenant by slug: %w", err)
	}
	return t, nil
}

func (s *TenantStore) GetByID(ctx context.Context, id uuid.UUID) (*model.Tenant, error) {
	query := `SELECT id, slug, name, created_at, updated_at FROM tenants WHERE id = $1`
	t := &model.Tenant{}
	err := s.db.Pool.QueryRow(ctx, query, id).Scan(&t.ID, &t.Slug, &t.Name, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("querying tenant by id: %w", err)
	}
	return t, nil
}

func (s *TenantStore) List(ctx context.Context, filter store.TenantFilter) ([]model.Tenant, int, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PerPage < 1 {
		filter.PerPage = 20
	}
	offset := (filter.Page - 1) * filter.PerPage

	var total int
	if err := s.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM tenants").Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting tenants: %w", err)
	}

	query := `SELECT id, slug, name, created_at, updated_at FROM tenants ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	rows, err := s.db.Pool.Query(ctx, query, filter.PerPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("listing tenants: %w", err)
	}
	defer rows.Close()
	var tenants []model.Tenant
	for rows.Next() {
		var t model.Tenant
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scanning tenant: %w", err)
		}
		tenants = append(tenants, t)
	}
	return tenants, total, nil
}

func (s *TenantStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.Pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("deleting tenant: %w", err)
	}
	return nil
}
