package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/wale/quorum/internal/auth"
	"github.com/wale/quorum/internal/service"
	"github.com/wale/quorum/internal/store"
)

type Server struct {
	router         chi.Router
	requestHandler *RequestHandler
	policyHandler  *PolicyHandler
	webhookHandler *WebhookHandler
	auditStore     store.AuditStore
	authProvider   auth.Provider
}

type Config struct {
	RequestService *service.RequestService
	PolicyService  *service.PolicyService
	WebhookService *service.WebhookService
	AuditStore     store.AuditStore
	AuthProvider   auth.Provider
}

func New(cfg Config) *Server {
	s := &Server{
		router:         chi.NewRouter(),
		requestHandler: NewRequestHandler(cfg.RequestService),
		policyHandler:  NewPolicyHandler(cfg.PolicyService),
		webhookHandler: NewWebhookHandler(cfg.WebhookService),
		auditStore:     cfg.AuditStore,
		authProvider:   cfg.AuthProvider,
	}

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	r := s.router

	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(loggingMiddleware)

	// Health check — no auth
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

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
