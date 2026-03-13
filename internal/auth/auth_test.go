package auth

import (
	"context"
	"testing"
)

func TestWithIdentity_UserIDFromContext(t *testing.T) {
	ctx := WithIdentity(context.Background(), &Identity{
		UserID: "user-123",
		Roles:  []string{"admin"},
	})

	got := UserIDFromContext(ctx)
	if got != "user-123" {
		t.Errorf("UserIDFromContext() = %q, want %q", got, "user-123")
	}
}

func TestWithIdentity_RolesFromContext(t *testing.T) {
	roles := []string{"admin", "manager"}
	ctx := WithIdentity(context.Background(), &Identity{
		UserID: "user-123",
		Roles:  roles,
	})

	got := RolesFromContext(ctx)
	if len(got) != 2 || got[0] != "admin" || got[1] != "manager" {
		t.Errorf("RolesFromContext() = %v, want %v", got, roles)
	}
}

func TestUserIDFromContext_EmptyContext(t *testing.T) {
	got := UserIDFromContext(context.Background())
	if got != "" {
		t.Errorf("UserIDFromContext(empty) = %q, want empty string", got)
	}
}

func TestRolesFromContext_EmptyContext(t *testing.T) {
	got := RolesFromContext(context.Background())
	if got != nil {
		t.Errorf("RolesFromContext(empty) = %v, want nil", got)
	}
}
