package server

import (
	"encoding/json"
	"errors"
	"net/http"

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

func (h *PolicyHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body model.PolicyInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
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

	policy := body.Policy()

	if err := h.policyService.Create(r.Context(), policy); err != nil {
		if errors.Is(err, service.ErrPolicyTypeConflict) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, service.ErrNoStages) || errors.Is(err, service.ErrInvalidStageIndex) || errors.Is(err, service.ErrInvalidDisplayTemplate) || errors.Is(err, service.ErrThresholdNoMaxCheckers) || errors.Is(err, service.ErrInvalidAuthorizationMode) || errors.Is(err, service.ErrInvalidScopeRule) || errors.Is(err, service.ErrInvalidBypassRule) {
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
	writeJSON(w, http.StatusOK, model.Page[model.Policy]{Data: policies, Total: total, Page: filter.Page})
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

	var body model.PolicyUpdate
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	body.ApplyTo(existing)

	if err := h.policyService.Update(r.Context(), existing); err != nil {
		if errors.Is(err, service.ErrNoStages) || errors.Is(err, service.ErrInvalidStageIndex) || errors.Is(err, service.ErrInvalidDisplayTemplate) || errors.Is(err, service.ErrThresholdNoMaxCheckers) || errors.Is(err, service.ErrInvalidAuthorizationMode) || errors.Is(err, service.ErrInvalidScopeRule) || errors.Is(err, service.ErrInvalidBypassRule) {
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
	writeJSON(w, http.StatusOK, model.List[string]{Data: types})
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
