package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/auth"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/service"
	"github.com/lawale/quorum/internal/store"
)

const maxPerPage = 100

type RequestHandler struct {
	requestService *service.RequestService
}

func NewRequestHandler(rs *service.RequestService) *RequestHandler {
	return &RequestHandler{requestService: rs}
}

type createRequestBody struct {
	Type              string          `json:"type"`
	Payload           json.RawMessage `json:"payload"`
	EligibleReviewers []string        `json:"eligible_reviewers,omitempty"`
	Metadata          json.RawMessage `json:"metadata,omitempty"`
}

func (h *RequestHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body createRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Type == "" {
		writeError(w, http.StatusBadRequest, "type is required")
		return
	}
	if body.Payload == nil {
		writeError(w, http.StatusBadRequest, "payload is required")
		return
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body.Payload, &obj); err != nil {
		writeError(w, http.StatusBadRequest, "payload must be a JSON object")
		return
	}
	if len(obj) == 0 {
		writeError(w, http.StatusBadRequest, "payload must not be empty")
		return
	}

	makerID := auth.UserIDFromContext(r.Context())

	req := &model.Request{
		Type:              body.Type,
		Payload:           body.Payload,
		MakerID:           makerID,
		EligibleReviewers: body.EligibleReviewers,
		Metadata:          body.Metadata,
	}

	// Check for idempotency key
	if key := r.Header.Get("Idempotency-Key"); key != "" {
		req.IdempotencyKey = &key
	}

	result, err := h.requestService.Create(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrDuplicateRequest):
			writeError(w, http.StatusConflict, err.Error())
		case errors.Is(err, service.ErrPolicyNotFound):
			writeError(w, http.StatusBadRequest, "no policy found for request type: "+body.Type)
		case errors.Is(err, service.ErrMissingIdentityFields):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeServerError(w, r, err, "failed to create request")
		}
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

func (h *RequestHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request ID")
		return
	}

	req, err := h.requestService.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrRequestNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeServerError(w, r, err, "failed to get request")
		return
	}

	viewerID := auth.UserIDFromContext(r.Context())
	roles := auth.RolesFromContext(r.Context())
	permissions := auth.PermissionsFromContext(r.Context())
	canAct := h.requestService.CanViewerAct(r.Context(), req, viewerID, roles, permissions)
	req.ViewerCanAct = &canAct

	writeJSON(w, http.StatusOK, req)
}

func (h *RequestHandler) List(w http.ResponseWriter, r *http.Request) {
	perPage := intParam(r, "per_page", 20)
	if perPage > maxPerPage {
		perPage = maxPerPage
	}

	filter := store.RequestFilter{
		Page:    intParam(r, "page", 1),
		PerPage: perPage,
	}

	if s := r.URL.Query().Get("status"); s != "" {
		status := model.RequestStatus(s)
		if !isValidStatus(status) {
			writeError(w, http.StatusBadRequest, "invalid status filter: must be one of pending, approved, rejected, cancelled, expired")
			return
		}
		filter.Status = &status
	}
	if t := r.URL.Query().Get("type"); t != "" {
		filter.Type = &t
	}
	if m := r.URL.Query().Get("maker_id"); m != "" {
		filter.MakerID = &m
	}

	requests, total, err := h.requestService.List(r.Context(), filter)
	if err != nil {
		writeServerError(w, r, err, "failed to list requests")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":  requests,
		"total": total,
		"page":  filter.Page,
	})
}

type decisionBody struct {
	Comment *string `json:"comment,omitempty"`
}

func (h *RequestHandler) Approve(w http.ResponseWriter, r *http.Request) {
	h.handleDecision(w, r, true)
}

func (h *RequestHandler) Reject(w http.ResponseWriter, r *http.Request) {
	h.handleDecision(w, r, false)
}

func (h *RequestHandler) handleDecision(w http.ResponseWriter, r *http.Request, approve bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request ID")
		return
	}

	var body decisionBody
	if r.Body != nil && r.Body != http.NoBody {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	checkerID := auth.UserIDFromContext(r.Context())
	roles := auth.RolesFromContext(r.Context())

	var result *model.Request
	if approve {
		result, err = h.requestService.Approve(r.Context(), id, checkerID, roles, body.Comment)
	} else {
		result, err = h.requestService.Reject(r.Context(), id, checkerID, roles, body.Comment)
	}

	if err != nil {
		switch {
		case errors.Is(err, service.ErrRequestNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, service.ErrRequestNotPending):
			writeError(w, http.StatusConflict, err.Error())
		case errors.Is(err, service.ErrSelfApproval):
			writeError(w, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrAlreadyActioned):
			writeError(w, http.StatusConflict, err.Error())
		case errors.Is(err, service.ErrInvalidCheckerRole):
			writeError(w, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrNotEligibleReviewer):
			writeError(w, http.StatusForbidden, err.Error())
		case errors.Is(err, auth.ErrAuthorizationDenied):
			writeError(w, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrInvalidCheckerPermission):
			writeError(w, http.StatusForbidden, err.Error())
		default:
			writeServerError(w, r, err, "failed to process decision")
		}
		return
	}

	permissions := auth.PermissionsFromContext(r.Context())
	canAct := h.requestService.CanViewerAct(r.Context(), result, checkerID, roles, permissions)
	result.ViewerCanAct = &canAct

	writeJSON(w, http.StatusOK, result)
}

func (h *RequestHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request ID")
		return
	}

	makerID := auth.UserIDFromContext(r.Context())
	result, err := h.requestService.Cancel(r.Context(), id, makerID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrRequestNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, service.ErrRequestNotPending):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeServerError(w, r, err, "failed to cancel request")
		}
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func intParam(r *http.Request, key string, defaultVal int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 1 {
		return defaultVal
	}
	return v
}

func isValidStatus(s model.RequestStatus) bool {
	switch s {
	case model.StatusPending, model.StatusApproved, model.StatusRejected, model.StatusCancelled, model.StatusExpired:
		return true
	}
	return false
}
