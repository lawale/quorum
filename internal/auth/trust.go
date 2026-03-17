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
}

func NewTrustProvider(userIDHeader, rolesHeader, permissionsHeader, tenantIDHeader string) *TrustProvider {
	return &TrustProvider{
		UserIDHeader:      userIDHeader,
		RolesHeader:       rolesHeader,
		PermissionsHeader: permissionsHeader,
		TenantIDHeader:    tenantIDHeader,
	}
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

	return &Identity{
		UserID:      userID,
		Roles:       roles,
		Permissions: permissions,
		TenantID:    tenantID,
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
