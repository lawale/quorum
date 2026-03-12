package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/wale/quorum/internal/model"
	"github.com/wale/quorum/internal/service"
)

type WebhookHandler struct {
	webhookService *service.WebhookService
}

func NewWebhookHandler(ws *service.WebhookService) *WebhookHandler {
	return &WebhookHandler{webhookService: ws}
}

type createWebhookBody struct {
	URL         string   `json:"url"`
	Events      []string `json:"events"`
	Secret      string   `json:"secret"`
	RequestType *string  `json:"request_type,omitempty"`
}

func (h *WebhookHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body createWebhookBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	webhook := &model.Webhook{
		URL:         body.URL,
		Events:      body.Events,
		Secret:      body.Secret,
		RequestType: body.RequestType,
	}

	if err := h.webhookService.Create(r.Context(), webhook); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, webhook)
}

func (h *WebhookHandler) List(w http.ResponseWriter, r *http.Request) {
	webhooks, err := h.webhookService.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list webhooks")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": webhooks})
}

func (h *WebhookHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid webhook ID")
		return
	}

	if err := h.webhookService.Delete(r.Context(), id); err != nil {
		if errors.Is(err, service.ErrWebhookNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete webhook")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
