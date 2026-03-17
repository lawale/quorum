package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/service"
	"github.com/lawale/quorum/internal/store"
)

const consoleCookieName = "quorum_session"
const consoleBasePath = "/api/v1/console"

// ConsoleHandler handles admin console API endpoints for operator management.
type ConsoleHandler struct {
	operatorService *service.OperatorService
	tenantService   *service.TenantService
	secureCookies   bool
}

func NewConsoleHandler(os *service.OperatorService, ts *service.TenantService, secureCookies bool) *ConsoleHandler {
	return &ConsoleHandler{operatorService: os, tenantService: ts, secureCookies: secureCookies}
}

func (h *ConsoleHandler) setAuthCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     consoleCookieName,
		Value:    token,
		Path:     consoleBasePath,
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   86400,
	})
}

type setupBody struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type loginBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type changePasswordBody struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type createOperatorBody struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

// Setup creates the first operator. Only works when no operators exist.
func (h *ConsoleHandler) Setup(w http.ResponseWriter, r *http.Request) {
	var body setupBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Username == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	op, token, err := h.operatorService.Setup(r.Context(), body.Username, body.Password, body.DisplayName)
	if err != nil {
		if errors.Is(err, service.ErrSetupAlreadyDone) {
			writeError(w, http.StatusConflict, "setup has already been completed")
			return
		}
		writeServerError(w, r, err, "failed to complete setup")
		return
	}

	h.setAuthCookie(w, token)
	writeJSON(w, http.StatusCreated, map[string]any{
		"operator": op,
		"token":    token,
	})
}

// Login authenticates an operator and returns a JWT.
func (h *ConsoleHandler) Login(w http.ResponseWriter, r *http.Request) {
	var body loginBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Username == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	op, token, err := h.operatorService.Login(r.Context(), body.Username, body.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "invalid username or password")
			return
		}
		writeServerError(w, r, err, "failed to login")
		return
	}

	h.setAuthCookie(w, token)
	writeJSON(w, http.StatusOK, map[string]any{
		"operator": op,
		"token":    token,
	})
}

// Logout clears the session cookie.
func (h *ConsoleHandler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     consoleCookieName,
		Value:    "",
		Path:     consoleBasePath,
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

// NeedsSetup returns whether the system needs initial setup.
func (h *ConsoleHandler) NeedsSetup(w http.ResponseWriter, r *http.Request) {
	needs, err := h.operatorService.NeedsSetup(r.Context())
	if err != nil {
		writeServerError(w, r, err, "failed to check setup status")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"needs_setup": needs,
	})
}

// Me returns the current authenticated operator.
func (h *ConsoleHandler) Me(w http.ResponseWriter, r *http.Request) {
	opID := operatorIDFromContext(r.Context())
	if opID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	op, err := h.operatorService.GetCurrentOperator(r.Context(), opID)
	if err != nil {
		if errors.Is(err, service.ErrOperatorNotFound) {
			writeError(w, http.StatusNotFound, "operator not found")
			return
		}
		writeServerError(w, r, err, "failed to get operator")
		return
	}

	writeJSON(w, http.StatusOK, op)
}

// ChangePassword changes the current operator's password.
func (h *ConsoleHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	opID := operatorIDFromContext(r.Context())
	if opID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var body changePasswordBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.CurrentPassword == "" || body.NewPassword == "" {
		writeError(w, http.StatusBadRequest, "current_password and new_password are required")
		return
	}

	err := h.operatorService.ChangePassword(r.Context(), opID, body.CurrentPassword, body.NewPassword)
	if err != nil {
		if errors.Is(err, service.ErrIncorrectPassword) {
			writeError(w, http.StatusBadRequest, "current password is incorrect")
			return
		}
		if errors.Is(err, service.ErrOperatorNotFound) {
			writeError(w, http.StatusNotFound, "operator not found")
			return
		}
		writeServerError(w, r, err, "failed to change password")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "password changed successfully"})
}

func (h *ConsoleHandler) ListOperators(w http.ResponseWriter, r *http.Request) {
	perPage := intParam(r, "per_page", 20)
	if perPage > maxPerPage {
		perPage = maxPerPage
	}
	filter := store.OperatorFilter{
		Page:    intParam(r, "page", 1),
		PerPage: perPage,
	}
	operators, total, err := h.operatorService.ListOperators(r.Context(), filter)
	if err != nil {
		writeServerError(w, r, err, "failed to list operators")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": operators, "total": total, "page": filter.Page})
}

// CreateOperator creates a new operator.
func (h *ConsoleHandler) CreateOperator(w http.ResponseWriter, r *http.Request) {
	var body createOperatorBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Username == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	op, err := h.operatorService.CreateOperator(r.Context(), body.Username, body.Password, body.DisplayName)
	if err != nil {
		if errors.Is(err, service.ErrUsernameExists) {
			writeError(w, http.StatusConflict, "username already exists")
			return
		}
		writeServerError(w, r, err, "failed to create operator")
		return
	}

	writeJSON(w, http.StatusCreated, op)
}

// DeleteOperator deletes an operator by ID.
func (h *ConsoleHandler) DeleteOperator(w http.ResponseWriter, r *http.Request) {
	callerID := operatorIDFromContext(r.Context())
	if callerID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	targetID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid operator ID")
		return
	}

	if err := h.operatorService.DeleteOperator(r.Context(), callerID, targetID); err != nil {
		if errors.Is(err, service.ErrCannotDeleteSelf) {
			writeError(w, http.StatusBadRequest, "cannot delete yourself")
			return
		}
		if errors.Is(err, service.ErrLastOperator) {
			writeError(w, http.StatusBadRequest, "cannot delete the last operator")
			return
		}
		if errors.Is(err, service.ErrOperatorNotFound) {
			writeError(w, http.StatusNotFound, "operator not found")
			return
		}
		writeServerError(w, r, err, "failed to delete operator")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- Tenant management ---

type createTenantBody struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

func (h *ConsoleHandler) ListTenants(w http.ResponseWriter, r *http.Request) {
	perPage := intParam(r, "per_page", 20)
	if perPage > maxPerPage {
		perPage = maxPerPage
	}
	filter := store.TenantFilter{
		Page:    intParam(r, "page", 1),
		PerPage: perPage,
	}
	tenants, total, err := h.tenantService.List(r.Context(), filter)
	if err != nil {
		writeServerError(w, r, err, "failed to list tenants")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": tenants, "total": total, "page": filter.Page})
}

// CreateTenant registers a new tenant.
func (h *ConsoleHandler) CreateTenant(w http.ResponseWriter, r *http.Request) {
	var body createTenantBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Slug == "" || body.Name == "" {
		writeError(w, http.StatusBadRequest, "slug and name are required")
		return
	}

	tenant, err := h.tenantService.Create(r.Context(), body.Slug, body.Name)
	if err != nil {
		if errors.Is(err, service.ErrTenantSlugInvalid) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, service.ErrTenantSlugExists) {
			writeError(w, http.StatusConflict, "a tenant with this slug already exists")
			return
		}
		writeServerError(w, r, err, "failed to create tenant")
		return
	}

	writeJSON(w, http.StatusCreated, tenant)
}

// DeleteTenant removes a tenant by ID.
func (h *ConsoleHandler) DeleteTenant(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tenant ID")
		return
	}

	if err := h.tenantService.Delete(r.Context(), id); err != nil {
		if errors.Is(err, service.ErrTenantNotFound) {
			writeError(w, http.StatusNotFound, "tenant not found")
			return
		}
		writeServerError(w, r, err, "failed to delete tenant")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
