package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/wale/maker-checker/internal/model"
	"github.com/wale/maker-checker/internal/service"
)

type PolicyHandler struct {
	policyService *service.PolicyService
}

func NewPolicyHandler(ps *service.PolicyService) *PolicyHandler {
	return &PolicyHandler{policyService: ps}
}

type createPolicyBody struct {
	Name                string          `json:"name"`
	RequestType         string          `json:"request_type"`
	RequiredApprovals   int             `json:"required_approvals"`
	AllowedCheckerRoles json.RawMessage `json:"allowed_checker_roles,omitempty"`
	RejectionPolicy     string          `json:"rejection_policy,omitempty"`
	MaxCheckers         *int            `json:"max_checkers,omitempty"`
	IdentityFields      []string        `json:"identity_fields,omitempty"`
	AutoExpireDuration  string          `json:"auto_expire_duration,omitempty"`
}

func (h *PolicyHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body createPolicyBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if body.RequestType == "" {
		writeError(w, http.StatusBadRequest, "request_type is required")
		return
	}

	policy := &model.Policy{
		Name:                body.Name,
		RequestType:         body.RequestType,
		RequiredApprovals:   body.RequiredApprovals,
		AllowedCheckerRoles: body.AllowedCheckerRoles,
		RejectionPolicy:     model.RejectionPolicy(body.RejectionPolicy),
		MaxCheckers:         body.MaxCheckers,
		IdentityFields:      body.IdentityFields,
	}

	if body.AutoExpireDuration != "" {
		d, err := time.ParseDuration(body.AutoExpireDuration)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid auto_expire_duration format, use Go duration syntax (e.g. 24h, 30m)")
			return
		}
		policy.AutoExpireDuration = &d
	}

	if err := h.policyService.Create(r.Context(), policy); err != nil {
		if errors.Is(err, service.ErrPolicyTypeConflict) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create policy")
		return
	}

	writeJSON(w, http.StatusCreated, policy)
}

func (h *PolicyHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid policy ID")
		return
	}

	policy, err := h.policyService.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrPolicyNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get policy")
		return
	}

	writeJSON(w, http.StatusOK, policy)
}

func (h *PolicyHandler) List(w http.ResponseWriter, r *http.Request) {
	policies, err := h.policyService.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list policies")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": policies})
}

type updatePolicyBody struct {
	Name                string          `json:"name"`
	RequiredApprovals   int             `json:"required_approvals"`
	AllowedCheckerRoles json.RawMessage `json:"allowed_checker_roles,omitempty"`
	RejectionPolicy     string          `json:"rejection_policy,omitempty"`
	MaxCheckers         *int            `json:"max_checkers,omitempty"`
	IdentityFields      []string        `json:"identity_fields,omitempty"`
	AutoExpireDuration  string          `json:"auto_expire_duration,omitempty"`
}

func (h *PolicyHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid policy ID")
		return
	}

	existing, err := h.policyService.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrPolicyNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get policy")
		return
	}

	var body updatePolicyBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Name != "" {
		existing.Name = body.Name
	}
	if body.RequiredApprovals > 0 {
		existing.RequiredApprovals = body.RequiredApprovals
	}
	if body.AllowedCheckerRoles != nil {
		existing.AllowedCheckerRoles = body.AllowedCheckerRoles
	}
	if body.RejectionPolicy != "" {
		existing.RejectionPolicy = model.RejectionPolicy(body.RejectionPolicy)
	}
	if body.MaxCheckers != nil {
		existing.MaxCheckers = body.MaxCheckers
	}
	if body.IdentityFields != nil {
		existing.IdentityFields = body.IdentityFields
	}
	if body.AutoExpireDuration != "" {
		d, err := time.ParseDuration(body.AutoExpireDuration)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid auto_expire_duration format")
			return
		}
		existing.AutoExpireDuration = &d
	}

	if err := h.policyService.Update(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update policy")
		return
	}

	writeJSON(w, http.StatusOK, existing)
}

func (h *PolicyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid policy ID")
		return
	}

	if err := h.policyService.Delete(r.Context(), id); err != nil {
		if errors.Is(err, service.ErrPolicyNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete policy")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
