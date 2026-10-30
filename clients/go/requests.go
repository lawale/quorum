package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/google/uuid"
	"github.com/lawale/quorum/internal/model"
)

// CreateRequest describes a change to submit for approval.
type CreateRequest struct {
	// Type selects the policy.
	Type string
	// Payload is the change itself: any value that marshals to a non-empty
	// JSON object, or a json.RawMessage holding one.
	Payload any
	// EligibleReviewers, when set, is a per-request allowlist of checker IDs.
	EligibleReviewers []string
	// Metadata is free-form data handed back to you. Set metadata.display to
	// control what the console and widgets show.
	Metadata any
	// IdempotencyKey makes retries safe: a repeated key returns the request
	// created the first time.
	IdempotencyKey string
}

// CreateRequest submits a request as the client's identity, the maker.
func (c *Client) CreateRequest(ctx context.Context, in CreateRequest) (*Request, error) {
	payload, err := toRaw(in.Payload)
	if err != nil {
		return nil, fmt.Errorf("quorum: payload: %w", err)
	}
	metadata, err := toRaw(in.Metadata)
	if err != nil {
		return nil, fmt.Errorf("quorum: metadata: %w", err)
	}
	body := model.CreateRequestInput{
		Type:              in.Type,
		Payload:           payload,
		EligibleReviewers: in.EligibleReviewers,
		Metadata:          metadata,
	}
	var extra http.Header
	if in.IdempotencyKey != "" {
		extra = http.Header{"Idempotency-Key": {in.IdempotencyKey}}
	}
	var out Request
	if err := c.do(ctx, http.MethodPost, "/api/v1/requests", nil, extra, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetRequest fetches a request with its approvals. ViewerCanAct on the result
// says whether the client's identity may decide it right now.
func (c *Client) GetRequest(ctx context.Context, id uuid.UUID) (*Request, error) {
	var out Request
	if err := c.do(ctx, http.MethodGet, "/api/v1/requests/"+id.String(), nil, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListRequests filters the tenant's requests. Zero-value fields are not
// applied.
type ListRequests struct {
	Status  RequestStatus
	Type    string
	MakerID string
	// Search matches the payload and the request ID.
	Search  string
	Page    int
	PerPage int // capped at 100 by the server
}

// ListRequests returns one page of requests, newest first.
func (c *Client) ListRequests(ctx context.Context, f ListRequests) (*Page[Request], error) {
	q := url.Values{}
	setQuery(q, "status", string(f.Status))
	setQuery(q, "type", f.Type)
	setQuery(q, "maker_id", f.MakerID)
	setQuery(q, "search", f.Search)
	setPaging(q, f.Page, f.PerPage)
	var out Page[Request]
	if err := c.do(ctx, http.MethodGet, "/api/v1/requests", q, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Approve records the client's identity approving the request at its current
// stage. comment may be empty.
func (c *Client) Approve(ctx context.Context, id uuid.UUID, comment string) (*Request, error) {
	return c.decide(ctx, id, "approve", comment)
}

// Reject records the client's identity rejecting the request at its current
// stage. comment may be empty.
func (c *Client) Reject(ctx context.Context, id uuid.UUID, comment string) (*Request, error) {
	return c.decide(ctx, id, "reject", comment)
}

func (c *Client) decide(ctx context.Context, id uuid.UUID, verb, comment string) (*Request, error) {
	var body any
	if comment != "" {
		body = model.DecisionInput{Comment: &comment}
	}
	var out Request
	if err := c.do(ctx, http.MethodPost, "/api/v1/requests/"+id.String()+"/"+verb, nil, nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Cancel withdraws a pending request. Only its maker may do so.
func (c *Client) Cancel(ctx context.Context, id uuid.UUID) (*Request, error) {
	var out Request
	if err := c.do(ctx, http.MethodPost, "/api/v1/requests/"+id.String()+"/cancel", nil, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Audit returns every recorded action on a request, oldest first.
func (c *Client) Audit(ctx context.Context, id uuid.UUID) ([]AuditLog, error) {
	var out model.List[AuditLog]
	if err := c.do(ctx, http.MethodGet, "/api/v1/requests/"+id.String()+"/audit", nil, nil, nil, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// toRaw marshals v unless it is already raw JSON. nil stays nil.
func toRaw(v any) (json.RawMessage, error) {
	switch x := v.(type) {
	case nil:
		return nil, nil
	case json.RawMessage:
		return x, nil
	case []byte:
		return json.RawMessage(x), nil
	default:
		return json.Marshal(v)
	}
}

func setQuery(q url.Values, key, value string) {
	if value != "" {
		q.Set(key, value)
	}
}

func setPaging(q url.Values, page, perPage int) {
	if page > 0 {
		q.Set("page", strconv.Itoa(page))
	}
	if perPage > 0 {
		q.Set("per_page", strconv.Itoa(perPage))
	}
}
