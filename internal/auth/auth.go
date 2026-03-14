package auth

import (
	"context"
	"net/http"
)

type contextKey string

const (
	userIDKey   contextKey = "user_id"
	rolesKey    contextKey = "user_roles"
	tenantIDKey contextKey = "tenant_id"
)

// Identity represents the authenticated user extracted from the request.
type Identity struct {
	UserID   string
	Roles    []string
	TenantID string
}

// Provider extracts identity from an HTTP request.
type Provider interface {
	Authenticate(r *http.Request) (*Identity, error)
}

// WithIdentity stores the identity in the context.
func WithIdentity(ctx context.Context, id *Identity) context.Context {
	ctx = context.WithValue(ctx, userIDKey, id.UserID)
	ctx = context.WithValue(ctx, rolesKey, id.Roles)
	ctx = context.WithValue(ctx, tenantIDKey, id.TenantID)
	return ctx
}

// WithTenantID stores a tenant ID in the context (useful for background workers).
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantIDKey, tenantID)
}

// TenantIDFromContext extracts the tenant ID from the context.
func TenantIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(tenantIDKey).(string)
	return v
}

// UserIDFromContext extracts the user ID from the context.
func UserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(userIDKey).(string)
	return v
}

// RolesFromContext extracts the roles from the context.
func RolesFromContext(ctx context.Context) []string {
	v, _ := ctx.Value(rolesKey).([]string)
	return v
}
