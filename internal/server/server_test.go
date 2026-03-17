package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lawale/quorum/internal/auth"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/service"
	"github.com/lawale/quorum/internal/store"
	"github.com/lawale/quorum/internal/testutil"
)

// newTestServer creates a server with mock stores that return sane defaults.
func newTestServer() *Server {
	requests := &testutil.MockRequestStore{
		ListFunc: func(ctx context.Context, filter store.RequestFilter) ([]model.Request, int, error) {
			return nil, 0, nil
		},
	}
	policies := &testutil.MockPolicyStore{
		ListFunc: func(ctx context.Context, filter store.PolicyFilter) ([]model.Policy, int, error) {
			return nil, 0, nil
		},
	}
	webhooks := &testutil.MockWebhookStore{
		ListFunc: func(ctx context.Context, filter store.WebhookFilter) ([]model.Webhook, int, error) {
			return nil, 0, nil
		},
	}
	audits := &testutil.MockAuditStore{}
	provider := &testutil.MockAuthProvider{
		AuthenticateFunc: func(r *http.Request) (*auth.Identity, error) {
			return &auth.Identity{UserID: "test-user", Roles: []string{"admin"}}, nil
		},
	}

	requestSvc := service.NewRequestService(requests, &testutil.MockApprovalStore{}, policies, audits, nil)
	policySvc := service.NewPolicyService(policies)
	webhookSvc := service.NewWebhookService(webhooks)

	return New(Config{
		RequestService: requestSvc,
		PolicyService:  policySvc,
		WebhookService: webhookSvc,
		AuditStore:     audits,
		AuthProvider:   provider,
	})
}

func TestHealthEndpoint(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("body status = %q, want %q", body["status"], "ok")
	}
}

func TestAPIRequiresAuth(t *testing.T) {
	requests := &testutil.MockRequestStore{}
	policies := &testutil.MockPolicyStore{}
	webhooks := &testutil.MockWebhookStore{}
	audits := &testutil.MockAuditStore{}

	// Provider that always returns error
	provider := &testutil.MockAuthProvider{
		AuthenticateFunc: func(r *http.Request) (*auth.Identity, error) {
			return nil, http.ErrNoCookie
		},
	}

	requestSvc := service.NewRequestService(requests, &testutil.MockApprovalStore{}, policies, audits, nil)
	policySvc := service.NewPolicyService(policies)
	webhookSvc := service.NewWebhookService(webhooks)

	srv := New(Config{
		RequestService: requestSvc,
		PolicyService:  policySvc,
		WebhookService: webhookSvc,
		AuditStore:     audits,
		AuthProvider:   provider,
	})

	req := httptest.NewRequest("GET", "/api/v1/requests", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHealthEndpoint_NoAuth(t *testing.T) {
	// Health endpoint should work even without auth headers
	requests := &testutil.MockRequestStore{}
	policies := &testutil.MockPolicyStore{}
	webhooks := &testutil.MockWebhookStore{}
	audits := &testutil.MockAuditStore{}

	// Provider that always fails
	provider := &testutil.MockAuthProvider{
		AuthenticateFunc: func(r *http.Request) (*auth.Identity, error) {
			return nil, http.ErrNoCookie
		},
	}

	requestSvc := service.NewRequestService(requests, &testutil.MockApprovalStore{}, policies, audits, nil)
	policySvc := service.NewPolicyService(policies)
	webhookSvc := service.NewWebhookService(webhooks)

	srv := New(Config{
		RequestService: requestSvc,
		PolicyService:  policySvc,
		WebhookService: webhookSvc,
		AuditStore:     audits,
		AuthProvider:   provider,
	})

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("health should return 200 without auth, got %d", rec.Code)
	}
}
