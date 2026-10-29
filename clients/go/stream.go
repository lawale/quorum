package client

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Event is one message from a request's event stream.
type Event struct {
	RequestID uuid.UUID
	// Status is set only on the closing event of an already-terminal request.
	// For an "updated" event it is empty: fetch the request to see the change.
	Status RequestStatus
}

// Terminal reports whether the stream has nothing more to say.
func (e Event) Terminal() bool { return e.Status != "" && e.Status.IsTerminal() }

// Events subscribes to a request's server-sent events. The sequence ends when
// the server closes the stream (the request was already terminal), when ctx
// is done, or on a connection error, which is yielded last. A server with
// streaming disabled yields a 404 Error immediately.
func (c *Client) Events(ctx context.Context, id uuid.UUID) iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/requests/"+id.String()+"/events", nil)
		if err != nil {
			yield(Event{}, fmt.Errorf("quorum: building stream request: %w", err))
			return
		}
		c.prepare(req)
		req.Header.Set("Accept", "text/event-stream")

		// Streams outlive any sensible request timeout.
		hc := *c.http
		hc.Timeout = 0
		resp, err := hc.Do(req)
		if err != nil {
			yield(Event{}, fmt.Errorf("quorum: opening stream: %w", err))
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			yield(Event{}, newError(resp))
			return
		}

		var name string
		var data strings.Builder
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case line == "":
				if name != "" || data.Len() > 0 {
					ev, err := parseEvent(name, data.String())
					name, data = "", strings.Builder{}
					if err != nil {
						if !yield(Event{}, err) {
							return
						}
						continue
					}
					if !yield(ev, nil) {
						return
					}
				}
			case strings.HasPrefix(line, ":"):
				// keepalive comment
			case strings.HasPrefix(line, "event:"):
				name = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			case strings.HasPrefix(line, "data:"):
				data.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			}
		}
		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			yield(Event{}, fmt.Errorf("quorum: stream closed: %w", err))
		}
	}
}

func parseEvent(name, data string) (Event, error) {
	var body struct {
		RequestID uuid.UUID     `json:"request_id"`
		Status    RequestStatus `json:"status"`
	}
	if err := json.Unmarshal([]byte(data), &body); err != nil {
		return Event{}, fmt.Errorf("quorum: malformed %q event: %w", name, err)
	}
	return Event{RequestID: body.RequestID, Status: body.Status}, nil
}

// WaitOptions tune WaitForDecision.
type WaitOptions struct {
	// PollInterval is how often to re-fetch the request when the event
	// stream is unavailable. Default 5s.
	PollInterval time.Duration
}

// WaitForDecision blocks until the request reaches a terminal status and
// returns it. It listens on the event stream and falls back to polling when
// streaming is unavailable or drops. Bound the wait with ctx.
func (c *Client) WaitForDecision(ctx context.Context, id uuid.UUID, opts ...WaitOptions) (*Request, error) {
	var o WaitOptions
	if len(opts) > 0 {
		o = opts[0]
	}
	if o.PollInterval <= 0 {
		o.PollInterval = 5 * time.Second
	}

	for {
		// A fresh read first: the request may already be decided, and this
		// catches anything that happened before the stream opened.
		req, err := c.GetRequest(ctx, id)
		if err != nil {
			return nil, err
		}
		if req.Status.IsTerminal() {
			return req, nil
		}

		streamErr := c.followStream(ctx, id)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if streamErr == nil {
			// The stream closed cleanly after an update; loop to re-read.
			continue
		}

		// Streaming is unavailable: poll instead.
		req, err = c.poll(ctx, id, o.PollInterval)
		if err != nil {
			return nil, err
		}
		return req, nil
	}
}

// followStream consumes events until the request looks decided or the
// stream ends. It returns nil when it is worth re-reading the request and an
// error when streaming should be abandoned.
func (c *Client) followStream(ctx context.Context, id uuid.UUID) error {
	for ev, err := range c.Events(ctx, id) {
		if err != nil {
			return err
		}
		if ev.Terminal() {
			return nil
		}
		// An "updated" event: see whether it was the deciding one.
		req, err := c.GetRequest(ctx, id)
		if err != nil {
			return err
		}
		if req.Status.IsTerminal() {
			return nil
		}
	}
	return nil
}

func (c *Client) poll(ctx context.Context, id uuid.UUID, every time.Duration) (*Request, error) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			req, err := c.GetRequest(ctx, id)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return nil, err
				}
				continue // transient; keep polling
			}
			if req.Status.IsTerminal() {
				return req, nil
			}
		}
	}
}
