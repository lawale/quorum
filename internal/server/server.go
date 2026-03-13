package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/auth"
	"github.com/lawale/quorum/internal/metrics"
	"github.com/lawale/quorum/internal/service"
	"github.com/lawale/quorum/internal/store"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	router          chi.Router
	requestHandler  *RequestHandler
	policyHandler   *PolicyHandler
	webhookHandler  *WebhookHandler
	consoleHandler  *ConsoleHandler
	consoleSPA      http.Handler
	operatorService *service.OperatorService
	auditStore      store.AuditStore
	authProvider    auth.Provider
	consoleEnabled  bool
	metrics         *metrics.Metrics
	metricsPath     string
	registry        *prometheus.Registry
}

type Config struct {
	RequestService  *service.RequestService
	PolicyService   *service.PolicyService
	WebhookService  *service.WebhookService
	OperatorService *service.OperatorService
	AuditStore      store.AuditStore
	AuthProvider    auth.Provider
	ConsoleEnabled  bool
	ConsoleSPA      http.Handler // SPA handler from console package (nil when built without tag)
	Metrics         *metrics.Metrics
	MetricsPath     string
	Registry        *prometheus.Registry
}

func New(cfg Config) *Server {
	s := &Server{
		router:          chi.NewRouter(),
		requestHandler:  NewRequestHandler(cfg.RequestService),
		policyHandler:   NewPolicyHandler(cfg.PolicyService),
		webhookHandler:  NewWebhookHandler(cfg.WebhookService),
		auditStore:      cfg.AuditStore,
		authProvider:    cfg.AuthProvider,
		consoleEnabled:  cfg.ConsoleEnabled,
		consoleSPA:      cfg.ConsoleSPA,
		operatorService: cfg.OperatorService,
		metrics:         cfg.Metrics,
		metricsPath:     cfg.MetricsPath,
		registry:        cfg.Registry,
	}

	if cfg.OperatorService != nil {
		s.consoleHandler = NewConsoleHandler(cfg.OperatorService)
	}

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	r := s.router

	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(loggingMiddleware)
	if s.metrics != nil {
		r.Use(metrics.HTTPMiddleware(s.metrics))
	}

	// Health check — no auth
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Metrics endpoint — no auth
	if s.registry != nil {
		r.Handle(s.metricsPath, promhttp.HandlerFor(s.registry, promhttp.HandlerOpts{}))
	}

	// Console SPA — static files, no auth
	if s.consoleSPA != nil {
		r.Handle("/console", http.RedirectHandler("/console/", http.StatusMovedPermanently))
		r.Handle("/console/*", http.StripPrefix("/console", s.consoleSPA))
	}

	// Console API — JWT auth for admin console
	if s.consoleEnabled && s.consoleHandler != nil {
		r.Route("/api/v1/console", func(r chi.Router) {
			// Public endpoints — no auth
			r.Get("/auth/status", s.consoleHandler.NeedsSetup)
			r.Post("/auth/setup", s.consoleHandler.Setup)
			r.Post("/auth/login", s.consoleHandler.Login)

			// Authenticated endpoints — JWT required
			r.Group(func(r chi.Router) {
				r.Use(consoleJWTMiddleware(s.operatorService))

				// Operator management
				r.Get("/me", s.consoleHandler.Me)
				r.Put("/me/password", s.consoleHandler.ChangePassword)
				r.Get("/operators", s.consoleHandler.ListOperators)
				r.Post("/operators", s.consoleHandler.CreateOperator)
				r.Delete("/operators/{id}", s.consoleHandler.DeleteOperator)

				// Data endpoints — reuse existing handlers via JWT auth
				r.Get("/policies", s.policyHandler.List)
				r.Post("/policies", s.policyHandler.Create)
				r.Get("/policies/{id}", s.policyHandler.Get)
				r.Put("/policies/{id}", s.policyHandler.Update)
				r.Delete("/policies/{id}", s.policyHandler.Delete)

				r.Get("/webhooks", s.webhookHandler.List)
				r.Post("/webhooks", s.webhookHandler.Create)
				r.Delete("/webhooks/{id}", s.webhookHandler.Delete)

				r.Get("/requests", s.requestHandler.List)
				r.Get("/requests/{id}", s.requestHandler.Get)
				r.Get("/requests/{id}/audit", s.handleAudit)
			})
		})
	}

	// API v1 — requires auth
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(authMiddleware(s.authProvider))

		// Requests
		r.Route("/requests", func(r chi.Router) {
			r.Post("/", s.requestHandler.Create)
			r.Get("/", s.requestHandler.List)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", s.requestHandler.Get)
				r.Post("/approve", s.requestHandler.Approve)
				r.Post("/reject", s.requestHandler.Reject)
				r.Post("/cancel", s.requestHandler.Cancel)
				r.Get("/audit", s.handleAudit)
			})
		})

		// Policies
		r.Route("/policies", func(r chi.Router) {
			r.Post("/", s.policyHandler.Create)
			r.Get("/", s.policyHandler.List)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", s.policyHandler.Get)
				r.Put("/", s.policyHandler.Update)
				r.Delete("/", s.policyHandler.Delete)
			})
		})

		// Webhooks
		r.Route("/webhooks", func(r chi.Router) {
			r.Post("/", s.webhookHandler.Create)
			r.Get("/", s.webhookHandler.List)
			r.Delete("/{id}", s.webhookHandler.Delete)
		})
	})
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request ID")
		return
	}

	logs, err := s.auditStore.ListByRequestID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get audit logs")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": logs})
}

func (s *Server) Handler() http.Handler {
	return s.router
}
