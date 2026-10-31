// Package apitest boots the real Quorum HTTP server in-process on top of
// in-memory stores, so clients and handlers can be exercised end to end
// without a database. Every request and response can be validated against
// the published OpenAPI document as it passes through.
package apitest

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/lawale/quorum/api"
	"github.com/lawale/quorum/internal/auth"
	"github.com/lawale/quorum/internal/server"
	"github.com/lawale/quorum/internal/service"
	"github.com/lawale/quorum/internal/sse"
	"github.com/lawale/quorum/internal/testutil"
)

// Header names the in-process server trusts for identity.
const (
	UserIDHeader      = "X-User-ID"
	RolesHeader       = "X-User-Roles"
	PermissionsHeader = "X-User-Permissions"
	TenantIDHeader    = "X-Tenant-ID"
	BranchHeader      = "X-User-Branch"
)

// Version is what the harness server reports from /health.
const Version = "test"

// Server is a running in-process Quorum.
type Server struct {
	URL    string
	Stores *testutil.MemoryStores
	Hub    *sse.Hub

	Requests *service.RequestService
	Policies *service.PolicyService

	httpServer *httptest.Server
}

// New starts a server for the duration of the test. Identity comes from the
// trust headers above, with branch_id read from X-User-Branch so scope rules
// can be exercised. Any traffic that disagrees with the OpenAPI document
// fails t.
func New(t testing.TB) *Server {
	t.Helper()
	if !testing.Verbose() {
		quietLogs()
	}
	srv, err := Start(t.Errorf)
	if err != nil {
		t.Fatalf("loading OpenAPI document: %v", err)
	}
	t.Cleanup(srv.Close)
	return srv
}

// Start boots a server whose spec violations are passed to report, for
// callers that are not Go tests (see cmd/apiserver). Close it when done.
func Start(report func(format string, args ...any)) (*Server, error) {
	stores := testutil.NewMemoryStores()
	hub := sse.NewHub()

	requestSvc := service.NewRequestService(stores.Requests, stores.Approvals, stores.Policies, stores.Audits, nil)
	requestSvc.SetSSESignal(hub.Publish)
	policySvc := service.NewPolicyService(stores.Policies)
	webhookSvc := service.NewWebhookService(stores.Webhooks)

	provider := auth.NewTrustProvider(UserIDHeader, RolesHeader, PermissionsHeader, TenantIDHeader).
		WithAttributeHeaders(map[string]string{"branch_id": BranchHeader})

	srv := server.New(server.Config{
		Version:        Version,
		RequestService: requestSvc,
		PolicyService:  policySvc,
		WebhookService: webhookSvc,
		AuditStore:     stores.Audits,
		AuthProvider:   provider,
		EventHub:       hub,
	})

	handler, err := Validating(report, srv.Handler())
	if err != nil {
		return nil, err
	}
	hs := httptest.NewServer(handler)

	return &Server{
		URL:        hs.URL,
		Stores:     stores,
		Hub:        hub,
		Requests:   requestSvc,
		Policies:   policySvc,
		httpServer: hs,
	}, nil
}

// Close stops the server.
func (s *Server) Close() { s.httpServer.Close() }

var quietOnce sync.Once

// quietLogs silences the server's request log.
func quietLogs() {
	quietOnce.Do(func() {
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	})
}

var (
	specOnce   sync.Once
	specRouter routers.Router
	specErr    error

	coverageMu sync.Mutex
	coverage   = map[string]bool{}
)

func loadSpec() (routers.Router, error) {
	specOnce.Do(func() {
		loader := openapi3.NewLoader()
		doc, err := loader.LoadFromData(api.Spec)
		if err != nil {
			specErr = err
			return
		}
		if err := doc.Validate(loader.Context); err != nil {
			specErr = err
			return
		}
		for path, item := range doc.Paths.Map() {
			for method := range item.Operations() {
				coverage[method+" "+path] = false
			}
		}
		specRouter, specErr = gorillamux.NewRouter(doc)
	})
	return specRouter, specErr
}

// Validating wraps next so that every request it serves, and the response it
// produces, is checked against the OpenAPI document. A mismatch is passed to
// report (t.Errorf in a test). Event streams are passed through unchecked
// because they never end.
func Validating(report func(format string, args ...any), next http.Handler) (http.Handler, error) {
	router, err := loadSpec()
	if err != nil {
		return nil, err
	}
	opts := &openapi3filter.Options{
		AuthenticationFunc:    openapi3filter.NoopAuthenticationFunc,
		IncludeResponseStatus: true,
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route, pathParams, err := router.FindRoute(r)
		if err != nil {
			report("%s %s is not in the OpenAPI document: %v", r.Method, r.URL.Path, err)
			next.ServeHTTP(w, r)
			return
		}
		markCovered(route)

		if strings.HasSuffix(r.URL.Path, "/events") {
			next.ServeHTTP(w, r)
			return
		}

		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
		reqInput := &openapi3filter.RequestValidationInput{
			Request:    r,
			PathParams: pathParams,
			Route:      route,
			Options:    opts,
		}
		requestErr := openapi3filter.ValidateRequest(context.Background(), reqInput)
		r.Body = io.NopCloser(bytes.NewReader(body))

		rec := httptest.NewRecorder()
		next.ServeHTTP(rec, r)

		// Only hold the request to the document when the server accepted it;
		// a client is allowed to send junk and get a 4xx back.
		if requestErr != nil && rec.Code < 400 {
			report("%s %s: request accepted by server but rejected by OpenAPI document: %v", r.Method, r.URL.Path, requestErr)
		}

		respInput := &openapi3filter.ResponseValidationInput{
			RequestValidationInput: reqInput,
			Status:                 rec.Code,
			Header:                 rec.Header(),
			Options:                opts,
		}
		respInput.SetBodyBytes(rec.Body.Bytes())
		if err := openapi3filter.ValidateResponse(context.Background(), respInput); err != nil {
			report("%s %s -> %d: response does not match OpenAPI document: %v\nbody: %s", r.Method, r.URL.Path, rec.Code, err, rec.Body.String())
		}

		for k, v := range rec.Header() {
			w.Header()[k] = v
		}
		w.WriteHeader(rec.Code)
		_, _ = w.Write(rec.Body.Bytes())
	}), nil
}

func markCovered(route *routers.Route) {
	coverageMu.Lock()
	defer coverageMu.Unlock()
	coverage[route.Method+" "+route.Path] = true
}

// Uncovered lists the operations in the OpenAPI document that no validated
// request has reached yet. Call it from TestMain after m.Run to make sure a
// suite exercises the whole contract.
func Uncovered() []string {
	if _, err := loadSpec(); err != nil {
		return []string{"spec failed to load: " + err.Error()}
	}
	coverageMu.Lock()
	defer coverageMu.Unlock()
	var out []string
	for op, hit := range coverage {
		if !hit {
			out = append(out, op)
		}
	}
	return out
}
