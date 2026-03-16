package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/service"
	"github.com/lawale/quorum/internal/sse"
)

const sseKeepaliveInterval = 30 * time.Second

// SSEHandler serves Server-Sent Events for real-time request status updates.
type SSEHandler struct {
	hub            *sse.Hub
	requestService *service.RequestService
}

// NewSSEHandler creates a new SSE handler.
func NewSSEHandler(hub *sse.Hub, rs *service.RequestService) *SSEHandler {
	return &SSEHandler{hub: hub, requestService: rs}
}

// Events streams SSE notifications for a single request. The client receives
// an "updated" event whenever the request's state changes (approval, rejection,
// stage advance, cancellation, expiry). The payload is minimal — the widget
// re-fetches the full state via the REST API on each event.
func (h *SSEHandler) Events(w http.ResponseWriter, r *http.Request) {
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
		writeServerError(w, r, err, "failed to get request for SSE")
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable Nginx buffering

	// If the request is already terminal, send a single status event and close.
	if req.Status.IsTerminal() {
		data, _ := json.Marshal(map[string]string{
			"request_id": id.String(),
			"status":     string(req.Status),
		})
		fmt.Fprintf(w, "event: status\ndata: %s\n\n", data)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		return
	}

	// Use ResponseController to extend write deadlines for this long-lived connection.
	rc := http.NewResponseController(w)

	// Subscribe for real-time notifications on this request.
	sub := h.hub.Subscribe(id)
	defer h.hub.Unsubscribe(id, sub)

	// Flush headers so the client knows the SSE connection is established.
	if err := rc.Flush(); err != nil {
		return
	}

	keepalive := time.NewTicker(sseKeepaliveInterval)
	defer keepalive.Stop()

	for {
		select {
		case <-r.Context().Done():
			// Client disconnected.
			return

		case <-sub.C():
			// State change — send an update event.
			_ = rc.SetWriteDeadline(time.Now().Add(sseKeepaliveInterval + 15*time.Second))
			data, _ := json.Marshal(map[string]string{
				"request_id": id.String(),
			})
			if _, err := fmt.Fprintf(w, "event: updated\ndata: %s\n\n", data); err != nil {
				return
			}
			if err := rc.Flush(); err != nil {
				return
			}

		case <-keepalive.C:
			// Keep the connection alive through proxies.
			_ = rc.SetWriteDeadline(time.Now().Add(sseKeepaliveInterval + 15*time.Second))
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return
			}
			if err := rc.Flush(); err != nil {
				return
			}
		}
	}
}
