package model

import "encoding/json"

// This file holds the bodies the public API accepts and the envelopes it
// returns. Handlers decode straight into these types and the Go client
// encodes them, so a field added here reaches both sides at once.

// CreateRequestInput is the body of POST /api/v1/requests.
type CreateRequestInput struct {
	Type              string          `json:"type"`
	Payload           json.RawMessage `json:"payload"`
	EligibleReviewers []string        `json:"eligible_reviewers,omitempty"`
	Metadata          json.RawMessage `json:"metadata,omitempty"`
}

// DecisionInput is the optional body of the approve and reject endpoints.
type DecisionInput struct {
	Comment *string `json:"comment,omitempty"`
}

// PolicyInput is the body of POST /api/v1/policies.
type PolicyInput struct {
	Name                       string          `json:"name"`
	RequestType                string          `json:"request_type"`
	Stages                     []ApprovalStage `json:"stages"`
	IdentityFields             []string        `json:"identity_fields,omitempty"`
	DynamicAuthorizationURL    *string         `json:"dynamic_authorization_url,omitempty"`
	DynamicAuthorizationSecret *string         `json:"dynamic_authorization_secret,omitempty"`
	AutoExpireDuration         *Duration       `json:"auto_expire_duration,omitempty"`
	DisplayTemplate            json.RawMessage `json:"display_template,omitempty"`
	BypassRules                []BypassRule    `json:"bypass_rules,omitempty"`
}

// Policy builds the policy the input describes. Identity, tenant and
// timestamps are left for the service to assign.
func (in PolicyInput) Policy() *Policy {
	return &Policy{
		Name:                       in.Name,
		RequestType:                in.RequestType,
		Stages:                     in.Stages,
		IdentityFields:             in.IdentityFields,
		DynamicAuthorizationURL:    in.DynamicAuthorizationURL,
		DynamicAuthorizationSecret: in.DynamicAuthorizationSecret,
		AutoExpireDuration:         in.AutoExpireDuration,
		DisplayTemplate:            in.DisplayTemplate,
		BypassRules:                in.BypassRules,
	}
}

// PolicyUpdate is the body of PUT /api/v1/policies/{id}. A field that is
// absent keeps its stored value; send an empty list to clear a list field.
type PolicyUpdate struct {
	Name                       string          `json:"name,omitempty"`
	Stages                     []ApprovalStage `json:"stages,omitempty"`
	IdentityFields             []string        `json:"identity_fields,omitempty"`
	DynamicAuthorizationURL    *string         `json:"dynamic_authorization_url,omitempty"`
	DynamicAuthorizationSecret *string         `json:"dynamic_authorization_secret,omitempty"`
	AutoExpireDuration         *Duration       `json:"auto_expire_duration,omitempty"`
	DisplayTemplate            json.RawMessage `json:"display_template,omitempty"`
	BypassRules                []BypassRule    `json:"bypass_rules,omitempty"`
}

// MarshalJSON sends a list field only when it is set, so that an empty list
// clears the stored value while a nil one leaves it alone. Plain omitempty
// would drop both.
func (u PolicyUpdate) MarshalJSON() ([]byte, error) {
	out := map[string]any{}
	if u.Name != "" {
		out["name"] = u.Name
	}
	if u.Stages != nil {
		out["stages"] = u.Stages
	}
	if u.IdentityFields != nil {
		out["identity_fields"] = u.IdentityFields
	}
	if u.DynamicAuthorizationURL != nil {
		out["dynamic_authorization_url"] = u.DynamicAuthorizationURL
	}
	if u.DynamicAuthorizationSecret != nil {
		out["dynamic_authorization_secret"] = u.DynamicAuthorizationSecret
	}
	if u.AutoExpireDuration != nil {
		out["auto_expire_duration"] = u.AutoExpireDuration
	}
	if u.DisplayTemplate != nil {
		out["display_template"] = u.DisplayTemplate
	}
	if u.BypassRules != nil {
		out["bypass_rules"] = u.BypassRules
	}
	return json.Marshal(out)
}

// ApplyTo copies every field the update sets onto p.
func (u PolicyUpdate) ApplyTo(p *Policy) {
	if u.Name != "" {
		p.Name = u.Name
	}
	if u.Stages != nil {
		p.Stages = u.Stages
	}
	if u.IdentityFields != nil {
		p.IdentityFields = u.IdentityFields
	}
	if u.DynamicAuthorizationURL != nil {
		p.DynamicAuthorizationURL = u.DynamicAuthorizationURL
	}
	if u.DynamicAuthorizationSecret != nil {
		p.DynamicAuthorizationSecret = u.DynamicAuthorizationSecret
	}
	if u.AutoExpireDuration != nil {
		p.AutoExpireDuration = u.AutoExpireDuration
	}
	if u.DisplayTemplate != nil {
		p.DisplayTemplate = u.DisplayTemplate
	}
	if u.BypassRules != nil {
		p.BypassRules = u.BypassRules
	}
}

// WebhookInput is the body of POST /api/v1/webhooks.
type WebhookInput struct {
	URL         string   `json:"url"`
	Events      []string `json:"events"`
	Secret      string   `json:"secret"`
	RequestType *string  `json:"request_type,omitempty"`
}

// Webhook builds the subscription the input describes.
func (in WebhookInput) Webhook() *Webhook {
	return &Webhook{
		URL:         in.URL,
		Events:      in.Events,
		Secret:      in.Secret,
		RequestType: in.RequestType,
	}
}

// Page is the envelope every paginated list endpoint returns.
type Page[T any] struct {
	Data  []T `json:"data"`
	Total int `json:"total"`
	Page  int `json:"page"`
}

// NewPage builds a Page whose Data is never null on the wire.
func NewPage[T any](data []T, total, page int) Page[T] {
	if data == nil {
		data = []T{}
	}
	return Page[T]{Data: data, Total: total, Page: page}
}

// List is the envelope of an unpaginated list such as a request's audit trail.
type List[T any] struct {
	Data []T `json:"data"`
}

// NewList builds a List whose Data is never null on the wire.
func NewList[T any](data []T) List[T] {
	if data == nil {
		data = []T{}
	}
	return List[T]{Data: data}
}

// ErrorResponse is the body of every non-2xx API response.
type ErrorResponse struct {
	Error string `json:"error"`
}
