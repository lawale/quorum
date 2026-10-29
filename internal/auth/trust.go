package auth

import (
	"errors"
	"net/http"
	"strings"
)

// TrustProvider trusts identity headers from the host app.
type TrustProvider struct {
	UserIDHeader      string
	RolesHeader       string
	PermissionsHeader string
	TenantIDHeader    string
	// AttributeHeaders maps an attribute name to the header that carries it,
	// e.g. {"branch_id": "X-User-Branch"}. Absent headers are skipped.
	AttributeHeaders map[string]string
}

func NewTrustProvider(userIDHeader, rolesHeader, permissionsHeader, tenantIDHeader string) *TrustProvider {
	return &TrustProvider{
		UserIDHeader:      userIDHeader,
		RolesHeader:       rolesHeader,
		PermissionsHeader: permissionsHeader,
		TenantIDHeader:    tenantIDHeader,
	}
}

// WithAttributeHeaders configures the headers read into Identity.Attributes.
func (p *TrustProvider) WithAttributeHeaders(headers map[string]string) *TrustProvider {
	p.AttributeHeaders = headers
	return p
}

func (p *TrustProvider) Authenticate(r *http.Request) (*Identity, error) {
	userID := r.Header.Get(p.UserIDHeader)
	if userID == "" {
		return nil, errors.New("missing user identity header")
	}

	tenantID := r.Header.Get(p.TenantIDHeader)
	if tenantID == "" {
		return nil, errors.New("missing tenant identity header")
	}

	roles := parseCommaSeparated(r.Header.Get(p.RolesHeader))
	permissions := parseCommaSeparated(r.Header.Get(p.PermissionsHeader))

	var attrs map[string]string
	for name, header := range p.AttributeHeaders {
		if v := strings.TrimSpace(r.Header.Get(header)); v != "" {
			if attrs == nil {
				attrs = make(map[string]string, len(p.AttributeHeaders))
			}
			attrs[name] = v
		}
	}

	return &Identity{
		UserID:      userID,
		Roles:       roles,
		Permissions: permissions,
		TenantID:    tenantID,
		Attributes:  attrs,
	}, nil
}

func parseCommaSeparated(header string) []string {
	if header == "" {
		return nil
	}
	var result []string
	for _, v := range strings.Split(header, ",") {
		v = strings.TrimSpace(v)
		if v != "" {
			result = append(result, v)
		}
	}
	return result
}
