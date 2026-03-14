package auth

import (
	"errors"
	"net/http"
	"strings"
)

// TrustProvider trusts identity headers from the host app.
type TrustProvider struct {
	UserIDHeader   string
	RolesHeader    string
	TenantIDHeader string
}

func NewTrustProvider(userIDHeader, rolesHeader, tenantIDHeader string) *TrustProvider {
	return &TrustProvider{
		UserIDHeader:   userIDHeader,
		RolesHeader:    rolesHeader,
		TenantIDHeader: tenantIDHeader,
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

	var roles []string
	if rolesHeader := r.Header.Get(p.RolesHeader); rolesHeader != "" {
		for _, role := range strings.Split(rolesHeader, ",") {
			role = strings.TrimSpace(role)
			if role != "" {
				roles = append(roles, role)
			}
		}
	}

	return &Identity{
		UserID:   userID,
		Roles:    roles,
		TenantID: tenantID,
	}, nil
}
