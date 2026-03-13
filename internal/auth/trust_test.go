package auth

import (
	"net/http/httptest"
	"testing"
)

func TestTrustProvider_Authenticate_Success(t *testing.T) {
	p := NewTrustProvider("X-User-ID", "X-User-Roles")
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-User-ID", "user-123")
	r.Header.Set("X-User-Roles", "admin,manager")

	id, err := p.Authenticate(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id.UserID != "user-123" {
		t.Errorf("UserID = %q, want %q", id.UserID, "user-123")
	}
	if len(id.Roles) != 2 || id.Roles[0] != "admin" || id.Roles[1] != "manager" {
		t.Errorf("Roles = %v, want [admin manager]", id.Roles)
	}
}

func TestTrustProvider_Authenticate_MissingUserID(t *testing.T) {
	p := NewTrustProvider("X-User-ID", "X-User-Roles")
	r := httptest.NewRequest("GET", "/", nil)

	_, err := p.Authenticate(r)
	if err == nil {
		t.Fatal("expected error for missing user ID header")
	}
}

func TestTrustProvider_Authenticate_NoRoles(t *testing.T) {
	p := NewTrustProvider("X-User-ID", "X-User-Roles")
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-User-ID", "user-123")

	id, err := p.Authenticate(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id.UserID != "user-123" {
		t.Errorf("UserID = %q, want %q", id.UserID, "user-123")
	}
	if len(id.Roles) != 0 {
		t.Errorf("Roles = %v, want empty", id.Roles)
	}
}

func TestTrustProvider_Authenticate_MultipleRoles(t *testing.T) {
	p := NewTrustProvider("X-User-ID", "X-User-Roles")
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-User-ID", "user-123")
	r.Header.Set("X-User-Roles", "admin, manager, viewer")

	id, err := p.Authenticate(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(id.Roles) != 3 {
		t.Fatalf("expected 3 roles, got %d: %v", len(id.Roles), id.Roles)
	}
	// Roles should be trimmed
	expected := []string{"admin", "manager", "viewer"}
	for i, role := range id.Roles {
		if role != expected[i] {
			t.Errorf("Roles[%d] = %q, want %q", i, role, expected[i])
		}
	}
}

func TestTrustProvider_Authenticate_EmptyRoleEntries(t *testing.T) {
	p := NewTrustProvider("X-User-ID", "X-User-Roles")
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-User-ID", "user-123")
	r.Header.Set("X-User-Roles", "admin,,viewer,  ")

	id, err := p.Authenticate(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Empty entries should be filtered out
	if len(id.Roles) != 2 {
		t.Fatalf("expected 2 roles (empty filtered), got %d: %v", len(id.Roles), id.Roles)
	}
	if id.Roles[0] != "admin" || id.Roles[1] != "viewer" {
		t.Errorf("Roles = %v, want [admin viewer]", id.Roles)
	}
}

func TestTrustProvider_Authenticate_CustomHeaders(t *testing.T) {
	p := NewTrustProvider("X-Custom-User", "X-Custom-Roles")
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Custom-User", "custom-user")
	r.Header.Set("X-Custom-Roles", "role1")

	id, err := p.Authenticate(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id.UserID != "custom-user" {
		t.Errorf("UserID = %q, want %q", id.UserID, "custom-user")
	}
	if len(id.Roles) != 1 || id.Roles[0] != "role1" {
		t.Errorf("Roles = %v, want [role1]", id.Roles)
	}
}
