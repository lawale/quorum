package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/store"
)

var (
	ErrTenantNotFound     = errors.New("tenant not found")
	ErrTenantSlugExists   = errors.New("tenant slug already exists")
	ErrTenantSlugInvalid  = errors.New("tenant slug must be lowercase alphanumeric with hyphens only")
	ErrTenantSlugRequired = errors.New("tenant slug is required")
	ErrTenantNameRequired = errors.New("tenant name is required")
)

var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]$`)

type TenantService struct {
	tenants store.TenantStore

	mu    sync.RWMutex
	slugs map[string]struct{} // in-memory set of valid slugs
}

func NewTenantService(tenants store.TenantStore) *TenantService {
	return &TenantService{
		tenants: tenants,
		slugs:   make(map[string]struct{}),
	}
}

// LoadCache pre-populates the in-memory slug set from the database.
// Call this once during server startup.
func (s *TenantService) LoadCache(ctx context.Context) error {
	list, _, err := s.tenants.List(ctx, store.TenantFilter{Page: 1, PerPage: 10000})
	if err != nil {
		return fmt.Errorf("loading tenant cache: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.slugs = make(map[string]struct{}, len(list))
	for _, t := range list {
		s.slugs[t.Slug] = struct{}{}
	}
	return nil
}

// StartCacheRefresh runs a background goroutine that periodically reloads the
// tenant slug cache from the database. This keeps instances in sync when tenants
// are created or deleted on a different instance. The goroutine stops when ctx
// is cancelled.
func (s *TenantService) StartCacheRefresh(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.LoadCache(ctx); err != nil {
					slog.Error("failed to refresh tenant cache", "error", err)
				}
			}
		}
	}()
	slog.Info("tenant cache refresh started", "interval", interval)
}

// IsRegistered checks whether a tenant slug exists using the in-memory cache,
// avoiding a database round-trip on every API request.
func (s *TenantService) IsRegistered(slug string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.slugs[slug]
	return ok
}

func (s *TenantService) Create(ctx context.Context, slug, name string) (*model.Tenant, error) {
	if slug == "" {
		return nil, ErrTenantSlugRequired
	}
	if name == "" {
		return nil, ErrTenantNameRequired
	}
	if !slugPattern.MatchString(slug) {
		return nil, ErrTenantSlugInvalid
	}

	// Check for existing tenant with same slug
	existing, err := s.tenants.GetBySlug(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("checking existing tenant: %w", err)
	}
	if existing != nil {
		return nil, ErrTenantSlugExists
	}

	tenant := &model.Tenant{
		Slug: slug,
		Name: name,
	}

	if err := s.tenants.Create(ctx, tenant); err != nil {
		return nil, fmt.Errorf("creating tenant: %w", err)
	}

	s.mu.Lock()
	s.slugs[slug] = struct{}{}
	s.mu.Unlock()

	return tenant, nil
}

func (s *TenantService) GetBySlug(ctx context.Context, slug string) (*model.Tenant, error) {
	tenant, err := s.tenants.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if tenant == nil {
		return nil, ErrTenantNotFound
	}
	return tenant, nil
}

func (s *TenantService) List(ctx context.Context, filter store.TenantFilter) ([]model.Tenant, int, error) {
	return s.tenants.List(ctx, filter)
}

func (s *TenantService) Delete(ctx context.Context, id uuid.UUID) error {
	tenant, err := s.tenants.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("looking up tenant: %w", err)
	}
	if tenant == nil {
		return ErrTenantNotFound
	}

	if err := s.tenants.Delete(ctx, id); err != nil {
		return err
	}

	s.mu.Lock()
	delete(s.slugs, tenant.Slug)
	s.mu.Unlock()

	return nil
}
