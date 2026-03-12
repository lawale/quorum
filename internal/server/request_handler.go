package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/auth"
	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/service"
	"github.com/lawale/quorum/internal/store"
)

type RequestHandler struct {
	requestService *service.RequestService
}

func NewRequestHandler(rs *service.RequestService) *RequestHandler {
	return &RequestHandler{requestService: rs}
}

type createRequestBody struct {
	Type        string          `json:"type"`
	Payload     json.RawMessage `json:"payload"`
	CallbackURL *string         `json:"callback_url,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
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

	makerID := auth.UserIDFromContext(r.Context())

	req := &model.Request{
		Type:        body.Type,
		Payload:     body.Payload,
		MakerID:     makerID,
		CallbackURL: body.CallbackURL,
		Metadata:    body.Metadata,
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
			writeError(w, http.StatusInternalServerError, "failed to create request")
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
		writeError(w, http.StatusInternalServerError, "failed to get request")
		return
	}

	writeJSON(w, http.StatusOK, req)
}

func (h *RequestHandler) List(w http.ResponseWriter, r *http.Request) {
	filter := store.RequestFilter{
		Page:    intParam(r, "page", 1),
		PerPage: intParam(r, "per_page", 20),
	}

	if s := r.URL.Query().Get("status"); s != "" {
		status := model.RequestStatus(s)
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
		writeError(w, http.StatusInternalServerError, "failed to list requests")
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
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&body)
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
		default:
			writeError(w, http.StatusInternalServerError, "failed to process decision")
		}
		return
	}

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
			writeError(w, http.StatusInternalServerError, "failed to cancel request")
		}
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *RequestHandler) Audit(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request ID")
		return
	}

	// Verify request exists
	_, err = h.requestService.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrRequestNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get request")
		return
	}

	// The audit store is accessed through the service layer, but for now
	// we return it as part of a dedicated endpoint. We need to expose
	// audit listing through the request service.
	writeJSON(w, http.StatusOK, map[string]string{"message": "audit endpoint"})
}

func intParam(r *http.Request, key string, defaultVal int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return defaultVal
	}
	var v int
	if _, err := parseIntFromString(s, &v); err != nil {
		return defaultVal
	}
	return v
}

func parseIntFromString(s string, v *int) (bool, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return false, errors.New("not a number")
		}
		n = n*10 + int(c-'0')
	}
	*v = n
	return true, nil
}
