package auth

import (
	"context"
	"net/http"
)

type contextKey string

const (
	userIDKey contextKey = "user_id"
	rolesKey  contextKey = "user_roles"
)

// Identity represents the authenticated user extracted from the request.
type Identity struct {
	UserID string
	Roles  []string
}

// Provider extracts identity from an HTTP request.
type Provider interface {
	Authenticate(r *http.Request) (*Identity, error)
}

// WithIdentity stores the identity in the context.
func WithIdentity(ctx context.Context, id *Identity) context.Context {
	ctx = context.WithValue(ctx, userIDKey, id.UserID)
	ctx = context.WithValue(ctx, rolesKey, id.Roles)
	return ctx
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
