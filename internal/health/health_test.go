package health

import (
	"context"
	"errors"
	"testing"
)

type mockChecker struct {
	name string
	err  error
}

func (m *mockChecker) Name() string                   { return m.name }
func (m *mockChecker) Health(_ context.Context) error { return m.err }

func TestCheck_AllHealthy(t *testing.T) {
	checkers := []HealthChecker{
		&mockChecker{name: "postgres", err: nil},
		&mockChecker{name: "redis", err: nil},
	}

	healthy, components := Check(context.Background(), checkers)
	if !healthy {
		t.Fatal("expected healthy")
	}
	if len(components) != 2 {
		t.Fatalf("expected 2 components, got %d", len(components))
	}
	if components["postgres"].Status != "healthy" {
		t.Errorf("expected postgres healthy, got %s", components["postgres"].Status)
	}
	if components["redis"].Status != "healthy" {
		t.Errorf("expected redis healthy, got %s", components["redis"].Status)
	}
}

func TestCheck_OneUnhealthy(t *testing.T) {
	checkers := []HealthChecker{
		&mockChecker{name: "postgres", err: nil},
		&mockChecker{name: "redis", err: errors.New("connection refused")},
	}

	healthy, components := Check(context.Background(), checkers)
	if healthy {
		t.Fatal("expected unhealthy")
	}
	if components["postgres"].Status != "healthy" {
		t.Errorf("expected postgres healthy, got %s", components["postgres"].Status)
	}
	if components["redis"].Status != "unhealthy" {
		t.Errorf("expected redis unhealthy, got %s", components["redis"].Status)
	}
	if components["redis"].Error != "connection refused" {
		t.Errorf("expected error message, got %q", components["redis"].Error)
	}
}

func TestCheck_NoCheckers(t *testing.T) {
	healthy, components := Check(context.Background(), nil)
	if !healthy {
		t.Fatal("expected healthy with no checkers")
	}
	if len(components) != 0 {
		t.Fatalf("expected 0 components, got %d", len(components))
	}
}
