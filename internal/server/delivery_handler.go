package server

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/auth"
	"github.com/lawale/quorum/internal/store"
)

// DeliveryHandler exposes outbox/delivery status and manual retry to the console.
type DeliveryHandler struct {
	outbox       store.OutboxStore
	signalWorker func() // wakes the dispatcher after a manual retry
}

func NewDeliveryHandler(outbox store.OutboxStore, signalWorker func()) *DeliveryHandler {
	return &DeliveryHandler{outbox: outbox, signalWorker: signalWorker}
}

func (h *DeliveryHandler) List(w http.ResponseWriter, r *http.Request) {
	perPage := deliveryIntParam(r, "per_page", 20)
	if perPage > 100 {
		perPage = 100
	}

	filter := store.OutboxFilter{
		Page:    deliveryIntParam(r, "page", 1),
		PerPage: perPage,
	}

	if tenantID := auth.TenantIDFromContext(r.Context()); tenantID != "" {
		filter.TenantID = &tenantID
	}
	if s := r.URL.Query().Get("status"); s != "" {
		filter.Status = &s
	}
	if rid := r.URL.Query().Get("request_id"); rid != "" {
		id, err := uuid.Parse(rid)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid request_id")
			return
		}
		filter.RequestID = &id
	}

	entries, total, err := h.outbox.List(r.Context(), filter)
	if err != nil {
		writeServerError(w, r, err, "failed to list deliveries")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":  entries,
		"total": total,
		"page":  filter.Page,
	})
}

func (h *DeliveryHandler) Stats(w http.ResponseWriter, r *http.Request) {
	var tenantID *string
	if t := auth.TenantIDFromContext(r.Context()); t != "" {
		tenantID = &t
	}

	counts, err := h.outbox.CountByStatus(r.Context(), tenantID)
	if err != nil {
		writeServerError(w, r, err, "failed to get delivery stats")
		return
	}

	writeJSON(w, http.StatusOK, counts)
}

func (h *DeliveryHandler) Retry(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid delivery ID")
		return
	}

	if err := h.outbox.ResetForRetry(r.Context(), id); err != nil {
		writeServerError(w, r, err, "failed to reset delivery for retry")
		return
	}

	if h.signalWorker != nil {
		h.signalWorker()
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "queued"})
}

func (h *DeliveryHandler) RetryAllForRequest(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request ID")
		return
	}

	count, err := h.outbox.ResetAllFailedForRequest(r.Context(), id)
	if err != nil {
		writeServerError(w, r, err, "failed to reset deliveries for retry")
		return
	}

	if count > 0 && h.signalWorker != nil {
		h.signalWorker()
	}

	writeJSON(w, http.StatusOK, map[string]any{"reset": count})
}

func deliveryIntParam(r *http.Request, key string, defaultVal int) int {
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
