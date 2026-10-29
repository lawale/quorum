package client

import (
	"net/http"
	"strings"
)

// Credentials put an identity on an outgoing request. Use Identity against a
// server in trust mode and BearerToken against one in verify mode.
type Credentials interface {
	apply(h http.Header, names HeaderNames, defaultTenant string)
}

// Identity is a trusted identity, sent as gateway headers. It is only safe to
// use when the client itself is the trusted gateway, or sits behind one that
// strips these headers from the outside world.
type Identity struct {
	UserID      string
	TenantID    string // falls back to the client's WithTenant
	Roles       []string
	Permissions []string
	// Attributes are named identity facts such as branch_id. Each one is sent
	// in the header the server is configured to read it from; see
	// HeaderNames.Attributes.
	Attributes map[string]string
}

func (id Identity) apply(h http.Header, names HeaderNames, defaultTenant string) {
	if id.UserID != "" {
		h.Set(names.UserID, id.UserID)
	}
	tenant := id.TenantID
	if tenant == "" {
		tenant = defaultTenant
	}
	if tenant != "" {
		h.Set(names.TenantID, tenant)
	}
	if len(id.Roles) > 0 {
		h.Set(names.Roles, strings.Join(id.Roles, ","))
	}
	if len(id.Permissions) > 0 {
		h.Set(names.Permissions, strings.Join(id.Permissions, ","))
	}
	for attr, value := range id.Attributes {
		if header, ok := names.Attributes[attr]; ok && value != "" {
			h.Set(header, value)
		}
	}
}

// BearerToken is a JWT for a server in verify mode.
type BearerToken string

func (t BearerToken) apply(h http.Header, _ HeaderNames, _ string) {
	h.Set("Authorization", "Bearer "+string(t))
}

// HeaderNames are the headers a trust-mode server reads identity from. They
// must match the server's auth.trust configuration.
type HeaderNames struct {
	UserID      string
	Roles       string
	Permissions string
	TenantID    string
	// Attributes maps an attribute name to its header, e.g. {"branch_id": "X-User-Branch"}.
	Attributes map[string]string
}

// DefaultHeaderNames are the server's defaults.
func DefaultHeaderNames() HeaderNames {
	return HeaderNames{
		UserID:      "X-User-ID",
		Roles:       "X-User-Roles",
		Permissions: "X-User-Permissions",
		TenantID:    "X-Tenant-ID",
	}
}

func (n HeaderNames) merge(o HeaderNames) HeaderNames {
	if o.UserID != "" {
		n.UserID = o.UserID
	}
	if o.Roles != "" {
		n.Roles = o.Roles
	}
	if o.Permissions != "" {
		n.Permissions = o.Permissions
	}
	if o.TenantID != "" {
		n.TenantID = o.TenantID
	}
	if len(o.Attributes) > 0 {
		merged := make(map[string]string, len(n.Attributes)+len(o.Attributes))
		for k, v := range n.Attributes {
			merged[k] = v
		}
		for k, v := range o.Attributes {
			merged[k] = v
		}
		n.Attributes = merged
	}
	return n
}
