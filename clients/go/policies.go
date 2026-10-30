package client

import (
	"context"
	"net/http"
	"net/url"

	"github.com/google/uuid"
)

// Paging selects one page of a list.
type Paging struct {
	Page    int
	PerPage int // capped at 100 by the server
}

// CreatePolicy registers the approval policy for a request type. Warnings on
// the result are advisory problems that did not block the save.
func (c *Client) CreatePolicy(ctx context.Context, in PolicyInput) (*Policy, error) {
	var out Policy
	if err := c.do(ctx, http.MethodPost, "/api/v1/policies", nil, nil, in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetPolicy fetches a policy by ID.
func (c *Client) GetPolicy(ctx context.Context, id uuid.UUID) (*Policy, error) {
	var out Policy
	if err := c.do(ctx, http.MethodGet, "/api/v1/policies/"+id.String(), nil, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListPolicies returns one page of the tenant's policies, newest first.
func (c *Client) ListPolicies(ctx context.Context, p Paging) (*Page[Policy], error) {
	q := url.Values{}
	setPaging(q, p.Page, p.PerPage)
	var out Page[Policy]
	if err := c.do(ctx, http.MethodGet, "/api/v1/policies", q, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdatePolicy changes the fields set on u and leaves the rest alone. Send an
// empty, non-nil slice to clear a list field such as BypassRules.
func (c *Client) UpdatePolicy(ctx context.Context, id uuid.UUID, u PolicyUpdate) (*Policy, error) {
	var out Policy
	if err := c.do(ctx, http.MethodPut, "/api/v1/policies/"+id.String(), nil, nil, u, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeletePolicy removes a policy. Requests already created under it keep
// their state.
func (c *Client) DeletePolicy(ctx context.Context, id uuid.UUID) error {
	return c.do(ctx, http.MethodDelete, "/api/v1/policies/"+id.String(), nil, nil, nil, nil)
}
