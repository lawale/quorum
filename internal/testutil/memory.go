package testutil

import (
	"context"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/auth"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/store"
)

// MemoryStores is a stateful, in-memory implementation of every store the
// request, policy and webhook services need. It honours the same contracts
// the SQL stores do (compare-and-set on status, unique approvals, nil for
// not found) so services and handlers can be exercised end to end without
// a database.
type MemoryStores struct {
	Requests  *MemoryRequestStore
	Approvals *MemoryApprovalStore
	Policies  *MemoryPolicyStore
	Webhooks  *MemoryWebhookStore
	Audits    *MemoryAuditStore
}

func NewMemoryStores() *MemoryStores {
	return &MemoryStores{
		Requests:  &MemoryRequestStore{items: map[uuid.UUID]*model.Request{}},
		Approvals: &MemoryApprovalStore{},
		Policies:  &MemoryPolicyStore{items: map[uuid.UUID]*model.Policy{}},
		Webhooks:  &MemoryWebhookStore{items: map[uuid.UUID]*model.Webhook{}},
		Audits:    &MemoryAuditStore{},
	}
}

func tenantFor(ctx context.Context, current string) string {
	if current != "" {
		return current
	}
	return auth.TenantIDFromContext(ctx)
}

func paginate[T any](items []T, page, perPage int) []T {
	if perPage <= 0 {
		perPage = 20
	}
	if page <= 0 {
		page = 1
	}
	start := (page - 1) * perPage
	if start >= len(items) {
		return []T{}
	}
	end := min(start+perPage, len(items))
	return items[start:end]
}

// --- requests ---------------------------------------------------------------

type MemoryRequestStore struct {
	mu    sync.Mutex
	items map[uuid.UUID]*model.Request
}

func (s *MemoryRequestStore) Create(ctx context.Context, req *model.Request) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if req.ID == uuid.Nil {
		req.ID = uuid.New()
	}
	now := time.Now().UTC()
	if req.CreatedAt.IsZero() {
		req.CreatedAt = now
	}
	req.UpdatedAt = now
	req.TenantID = tenantFor(ctx, req.TenantID)
	if req.Status == "" {
		req.Status = model.StatusPending
	}
	cp := *req
	s.items[req.ID] = &cp
	return nil
}

func (s *MemoryRequestStore) get(id uuid.UUID) *model.Request {
	if r, ok := s.items[id]; ok {
		cp := *r
		return &cp
	}
	return nil
}

func (s *MemoryRequestStore) GetByID(_ context.Context, id uuid.UUID) (*model.Request, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.get(id), nil
}

func (s *MemoryRequestStore) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (*model.Request, error) {
	return s.GetByID(ctx, id)
}

func (s *MemoryRequestStore) GetByIdempotencyKey(_ context.Context, key string) (*model.Request, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.items {
		if r.IdempotencyKey != nil && *r.IdempotencyKey == key {
			return s.get(r.ID), nil
		}
	}
	return nil, nil
}

func (s *MemoryRequestStore) FindPendingByFingerprint(_ context.Context, reqType, fingerprint string) (*model.Request, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.items {
		if r.Type == reqType && r.Status == model.StatusPending && r.Fingerprint != nil && *r.Fingerprint == fingerprint {
			return s.get(r.ID), nil
		}
	}
	return nil, nil
}

func (s *MemoryRequestStore) List(_ context.Context, filter store.RequestFilter) ([]model.Request, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []model.Request
	for _, r := range s.items {
		if filter.TenantID != nil && r.TenantID != *filter.TenantID {
			continue
		}
		if filter.Status != nil && r.Status != *filter.Status {
			continue
		}
		if filter.Type != nil && r.Type != *filter.Type {
			continue
		}
		if filter.MakerID != nil && r.MakerID != *filter.MakerID {
			continue
		}
		if filter.Search != nil && !strings.Contains(string(r.Payload), *filter.Search) && !strings.Contains(r.ID.String(), *filter.Search) {
			continue
		}
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return paginate(out, filter.Page, filter.PerPage), len(out), nil
}

func (s *MemoryRequestStore) UpdateStatus(_ context.Context, id uuid.UUID, status model.RequestStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.items[id]
	if !ok || r.Status != model.StatusPending {
		return store.ErrStatusConflict
	}
	r.Status = status
	r.UpdatedAt = time.Now().UTC()
	return nil
}

func (s *MemoryRequestStore) UpdateStageAndStatus(_ context.Context, id uuid.UUID, stage int, status model.RequestStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.items[id]
	if !ok || r.Status != model.StatusPending {
		return store.ErrStatusConflict
	}
	r.CurrentStage = stage
	r.Status = status
	r.UpdatedAt = time.Now().UTC()
	return nil
}

func (s *MemoryRequestStore) ListExpired(_ context.Context) ([]model.Request, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	var out []model.Request
	for _, r := range s.items {
		if r.Status == model.StatusPending && r.ExpiresAt != nil && r.ExpiresAt.Before(now) {
			out = append(out, *r)
		}
	}
	return out, nil
}

func (s *MemoryRequestStore) ListRecentByType(_ context.Context, reqType string, since time.Time, makerID string) ([]model.Request, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []model.Request
	for _, r := range s.items {
		if r.Type != reqType || r.CreatedAt.Before(since) {
			continue
		}
		if makerID != "" && r.MakerID != makerID {
			continue
		}
		out = append(out, *r)
	}
	return out, nil
}

// --- approvals --------------------------------------------------------------

type MemoryApprovalStore struct {
	mu    sync.Mutex
	items []model.Approval
}

func (s *MemoryApprovalStore) Create(_ context.Context, a *model.Approval) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.items {
		if existing.RequestID == a.RequestID && existing.CheckerID == a.CheckerID && existing.StageIndex == a.StageIndex {
			return store.ErrDuplicateApproval
		}
	}
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
	s.items = append(s.items, *a)
	return nil
}

func (s *MemoryApprovalStore) ListByRequestID(_ context.Context, requestID uuid.UUID) ([]model.Approval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []model.Approval{}
	for _, a := range s.items {
		if a.RequestID == requestID {
			out = append(out, a)
		}
	}
	return out, nil
}

func (s *MemoryApprovalStore) CountByDecisionAndStage(_ context.Context, requestID uuid.UUID, decision model.Decision, stageIndex int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, a := range s.items {
		if a.RequestID == requestID && a.Decision == decision && a.StageIndex == stageIndex {
			n++
		}
	}
	return n, nil
}

func (s *MemoryApprovalStore) ExistsByCheckerAndStage(_ context.Context, requestID uuid.UUID, checkerID string, stageIndex int) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.items {
		if a.RequestID == requestID && a.CheckerID == checkerID && a.StageIndex == stageIndex {
			return true, nil
		}
	}
	return false, nil
}

// --- policies ---------------------------------------------------------------

type MemoryPolicyStore struct {
	mu    sync.Mutex
	items map[uuid.UUID]*model.Policy
}

func (s *MemoryPolicyStore) Create(ctx context.Context, p *model.Policy) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	now := time.Now().UTC()
	p.CreatedAt, p.UpdatedAt = now, now
	p.TenantID = tenantFor(ctx, p.TenantID)
	cp := *p
	s.items[p.ID] = &cp
	return nil
}

func (s *MemoryPolicyStore) GetByID(_ context.Context, id uuid.UUID) (*model.Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.items[id]; ok {
		cp := *p
		return &cp, nil
	}
	return nil, nil
}

func (s *MemoryPolicyStore) GetByRequestType(ctx context.Context, requestType string) (*model.Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tenant := auth.TenantIDFromContext(ctx)
	for _, p := range s.items {
		if p.RequestType == requestType && (tenant == "" || p.TenantID == tenant) {
			cp := *p
			return &cp, nil
		}
	}
	return nil, nil
}

func (s *MemoryPolicyStore) List(_ context.Context, filter store.PolicyFilter) ([]model.Policy, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []model.Policy
	for _, p := range s.items {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return paginate(out, filter.Page, filter.PerPage), len(out), nil
}

func (s *MemoryPolicyStore) Update(_ context.Context, p *model.Policy) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[p.ID]; !ok {
		return store.ErrNotFound
	}
	p.UpdatedAt = time.Now().UTC()
	cp := *p
	s.items[p.ID] = &cp
	return nil
}

func (s *MemoryPolicyStore) Delete(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[id]; !ok {
		return store.ErrNotFound
	}
	delete(s.items, id)
	return nil
}

func (s *MemoryPolicyStore) DistinctRequestTypes(_ context.Context) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, p := range s.items {
		if !slices.Contains(out, p.RequestType) {
			out = append(out, p.RequestType)
		}
	}
	sort.Strings(out)
	return out, nil
}

// --- webhooks ---------------------------------------------------------------

type MemoryWebhookStore struct {
	mu    sync.Mutex
	items map[uuid.UUID]*model.Webhook
}

func (s *MemoryWebhookStore) Create(ctx context.Context, w *model.Webhook) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	w.CreatedAt = time.Now().UTC()
	w.TenantID = tenantFor(ctx, w.TenantID)
	w.Active = true
	cp := *w
	s.items[w.ID] = &cp
	return nil
}

func (s *MemoryWebhookStore) GetByID(_ context.Context, id uuid.UUID) (*model.Webhook, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if w, ok := s.items[id]; ok {
		cp := *w
		return &cp, nil
	}
	return nil, nil
}

func (s *MemoryWebhookStore) List(_ context.Context, filter store.WebhookFilter) ([]model.Webhook, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []model.Webhook
	for _, w := range s.items {
		out = append(out, *w)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return paginate(out, filter.Page, filter.PerPage), len(out), nil
}

func (s *MemoryWebhookStore) ListByEventAndType(_ context.Context, event, requestType string) ([]model.Webhook, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []model.Webhook
	for _, w := range s.items {
		if !w.Active || !slices.Contains(w.Events, event) {
			continue
		}
		if w.RequestType != nil && *w.RequestType != requestType {
			continue
		}
		out = append(out, *w)
	}
	return out, nil
}

func (s *MemoryWebhookStore) Delete(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[id]; !ok {
		return store.ErrNotFound
	}
	delete(s.items, id)
	return nil
}

// --- audit ------------------------------------------------------------------

type MemoryAuditStore struct {
	mu    sync.Mutex
	items []model.AuditLog
}

func (s *MemoryAuditStore) Create(ctx context.Context, l *model.AuditLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	if l.CreatedAt.IsZero() {
		l.CreatedAt = time.Now().UTC()
	}
	l.TenantID = tenantFor(ctx, l.TenantID)
	s.items = append(s.items, *l)
	return nil
}

func (s *MemoryAuditStore) ListByRequestID(_ context.Context, requestID uuid.UUID) ([]model.AuditLog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []model.AuditLog{}
	for _, l := range s.items {
		if l.RequestID == requestID {
			out = append(out, l)
		}
	}
	return out, nil
}

var (
	_ store.RequestStore  = (*MemoryRequestStore)(nil)
	_ store.ApprovalStore = (*MemoryApprovalStore)(nil)
	_ store.PolicyStore   = (*MemoryPolicyStore)(nil)
	_ store.WebhookStore  = (*MemoryWebhookStore)(nil)
	_ store.AuditStore    = (*MemoryAuditStore)(nil)
)
