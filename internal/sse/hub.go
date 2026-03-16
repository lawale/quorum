// Package sse provides an in-process pub/sub hub for server-sent event
// notifications, keyed by request UUID. It allows SSE HTTP handlers to
// subscribe for real-time notifications when a request's state changes.
package sse

import (
	"sync"

	"github.com/google/uuid"
)

// Hub is a lightweight, thread-safe pub/sub dispatcher keyed by request ID.
// Subscribers receive a signal whenever Publish is called for their request.
type Hub struct {
	mu   sync.RWMutex
	subs map[uuid.UUID]map[*Subscriber]struct{}
}

// Subscriber holds the notification channel for a single SSE connection.
type Subscriber struct {
	ch chan struct{} // buffered(1) — coalesces rapid signals
}

// C returns the channel that receives signals on state changes.
func (s *Subscriber) C() <-chan struct{} {
	return s.ch
}

// NewHub creates a new event hub.
func NewHub() *Hub {
	return &Hub{
		subs: make(map[uuid.UUID]map[*Subscriber]struct{}),
	}
}

// Subscribe registers a new subscriber for the given request ID.
// The returned Subscriber's channel receives a signal each time Publish
// is called for that request. Call Unsubscribe to clean up.
func (h *Hub) Subscribe(requestID uuid.UUID) *Subscriber {
	sub := &Subscriber{
		ch: make(chan struct{}, 1),
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.subs[requestID] == nil {
		h.subs[requestID] = make(map[*Subscriber]struct{})
	}
	h.subs[requestID][sub] = struct{}{}

	return sub
}

// Unsubscribe removes a subscriber and cleans up the request key if no
// subscribers remain.
func (h *Hub) Unsubscribe(requestID uuid.UUID, sub *Subscriber) {
	h.mu.Lock()
	defer h.mu.Unlock()

	subs := h.subs[requestID]
	if subs == nil {
		return
	}
	delete(subs, sub)
	if len(subs) == 0 {
		delete(h.subs, requestID)
	}
}

// Publish sends a non-blocking signal to all subscribers for the given
// request ID. Subscribers that already have a pending signal are skipped
// (the buffered channel coalesces rapid publishes). This is safe to call
// from any goroutine — it takes a read lock since it only reads the
// subscriber map and does non-blocking channel sends.
func (h *Hub) Publish(requestID uuid.UUID) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for sub := range h.subs[requestID] {
		select {
		case sub.ch <- struct{}{}:
		default:
			// already has a pending signal — coalesce
		}
	}
}

// Len returns the total number of active subscribers across all requests.
// Intended for metrics and debugging.
func (h *Hub) Len() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	n := 0
	for _, subs := range h.subs {
		n += len(subs)
	}
	return n
}
