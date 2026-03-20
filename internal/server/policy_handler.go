package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/service"
	"github.com/lawale/quorum/internal/store"
)

type PolicyHandler struct {
	policyService *service.PolicyService
}

func NewPolicyHandler(ps *service.PolicyService) *PolicyHandler {
	return &PolicyHandler{policyService: ps}
}

type stageBody struct {
	Index               int             `json:"index"`
	Name                string          `json:"name,omitempty"`
	RequiredApprovals   int             `json:"required_approvals"`
	AllowedCheckerRoles json.RawMessage `json:"allowed_checker_roles,omitempty"`
	AllowedPermissions  json.RawMessage `json:"allowed_permissions,omitempty"`
	AuthorizationMode   string          `json:"authorization_mode,omitempty"`
	RejectionPolicy     string          `json:"rejection_policy,omitempty"`
	MaxCheckers         *int            `json:"max_checkers,omitempty"`
}

type createPolicyBody struct {
	Name                       string          `json:"name"`
	RequestType                string          `json:"request_type"`
	Stages                     []stageBody     `json:"stages"`
	IdentityFields             []string        `json:"identity_fields,omitempty"`
	DynamicAuthorizationURL    *string         `json:"dynamic_authorization_url,omitempty"`
	DynamicAuthorizationSecret *string         `json:"dynamic_authorization_secret,omitempty"`
	AutoExpireDuration         string          `json:"auto_expire_duration,omitempty"`
	DisplayTemplate            json.RawMessage `json:"display_template,omitempty"`
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
	if len(body.Stages) == 0 {
		writeError(w, http.StatusBadRequest, "at least one stage is required")
		return
	}

	stages := make([]model.ApprovalStage, len(body.Stages))
	for i, sb := range body.Stages {
		stages[i] = model.ApprovalStage{
			Index:               sb.Index,
			Name:                sb.Name,
			RequiredApprovals:   sb.RequiredApprovals,
			AllowedCheckerRoles: sb.AllowedCheckerRoles,
			AllowedPermissions:  sb.AllowedPermissions,
			AuthorizationMode:   model.AuthorizationMode(sb.AuthorizationMode),
			RejectionPolicy:     model.RejectionPolicy(sb.RejectionPolicy),
			MaxCheckers:         sb.MaxCheckers,
		}
	}

	policy := &model.Policy{
		Name:                       body.Name,
		RequestType:                body.RequestType,
		Stages:                     stages,
		IdentityFields:             body.IdentityFields,
		DynamicAuthorizationURL:    body.DynamicAuthorizationURL,
		DynamicAuthorizationSecret: body.DynamicAuthorizationSecret,
		DisplayTemplate:            body.DisplayTemplate,
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
		if errors.Is(err, service.ErrNoStages) || errors.Is(err, service.ErrInvalidStageIndex) || errors.Is(err, service.ErrInvalidDisplayTemplate) || errors.Is(err, service.ErrThresholdNoMaxCheckers) || errors.Is(err, service.ErrInvalidAuthorizationMode) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeServerError(w, r, err, "failed to create policy")
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
		writeServerError(w, r, err, "failed to get policy")
		return
	}

	writeJSON(w, http.StatusOK, policy)
}

func (h *PolicyHandler) List(w http.ResponseWriter, r *http.Request) {
	perPage := intParam(r, "per_page", 20)
	if perPage > maxPerPage {
		perPage = maxPerPage
	}
	filter := store.PolicyFilter{
		Page:    intParam(r, "page", 1),
		PerPage: perPage,
	}
	policies, total, err := h.policyService.List(r.Context(), filter)
	if err != nil {
		writeServerError(w, r, err, "failed to list policies")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": policies, "total": total, "page": filter.Page})
}

type updatePolicyBody struct {
	Name                       string          `json:"name"`
	Stages                     []stageBody     `json:"stages,omitempty"`
	IdentityFields             []string        `json:"identity_fields,omitempty"`
	DynamicAuthorizationURL    *string         `json:"dynamic_authorization_url,omitempty"`
	DynamicAuthorizationSecret *string         `json:"dynamic_authorization_secret,omitempty"`
	AutoExpireDuration         string          `json:"auto_expire_duration,omitempty"`
	DisplayTemplate            json.RawMessage `json:"display_template,omitempty"`
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
		writeServerError(w, r, err, "failed to get policy")
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
	if body.Stages != nil {
		stages := make([]model.ApprovalStage, len(body.Stages))
		for i, sb := range body.Stages {
			stages[i] = model.ApprovalStage{
				Index:               sb.Index,
				Name:                sb.Name,
				RequiredApprovals:   sb.RequiredApprovals,
				AllowedCheckerRoles: sb.AllowedCheckerRoles,
				AllowedPermissions:  sb.AllowedPermissions,
				AuthorizationMode:   model.AuthorizationMode(sb.AuthorizationMode),
				RejectionPolicy:     model.RejectionPolicy(sb.RejectionPolicy),
				MaxCheckers:         sb.MaxCheckers,
			}
		}
		existing.Stages = stages
	}
	if body.IdentityFields != nil {
		existing.IdentityFields = body.IdentityFields
	}
	if body.DynamicAuthorizationURL != nil {
		existing.DynamicAuthorizationURL = body.DynamicAuthorizationURL
	}
	if body.DynamicAuthorizationSecret != nil {
		existing.DynamicAuthorizationSecret = body.DynamicAuthorizationSecret
	}
	if body.AutoExpireDuration != "" {
		d, err := time.ParseDuration(body.AutoExpireDuration)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid auto_expire_duration format")
			return
		}
		existing.AutoExpireDuration = &d
	}
	if body.DisplayTemplate != nil {
		existing.DisplayTemplate = body.DisplayTemplate
	}

	if err := h.policyService.Update(r.Context(), existing); err != nil {
		if errors.Is(err, service.ErrNoStages) || errors.Is(err, service.ErrInvalidStageIndex) || errors.Is(err, service.ErrInvalidDisplayTemplate) || errors.Is(err, service.ErrThresholdNoMaxCheckers) || errors.Is(err, service.ErrInvalidAuthorizationMode) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeServerError(w, r, err, "failed to update policy")
		return
	}

	writeJSON(w, http.StatusOK, existing)
}

func (h *PolicyHandler) RequestTypes(w http.ResponseWriter, r *http.Request) {
	types, err := h.policyService.DistinctRequestTypes(r.Context())
	if err != nil {
		writeServerError(w, r, err, "failed to list request types")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": types})
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
		writeServerError(w, r, err, "failed to delete policy")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
