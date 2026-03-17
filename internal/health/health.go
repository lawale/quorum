package health

import "context"

// HealthChecker is implemented by any dependency that can report its health.
// Database backends, caches, and external services implement this interface
// and register with the health aggregator via the server config.
type HealthChecker interface {
	Name() string
	Health(ctx context.Context) error
}

// ComponentStatus represents the health of a single dependency.
type ComponentStatus struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// Check runs all registered health checkers and returns an aggregate result.
// Returns healthy=true only if every checker passes. An empty checkers list
// is considered healthy (vacuous truth).
func Check(ctx context.Context, checkers []HealthChecker) (healthy bool, components map[string]ComponentStatus) {
	components = make(map[string]ComponentStatus, len(checkers))
	healthy = true

	for _, c := range checkers {
		if err := c.Health(ctx); err != nil {
			components[c.Name()] = ComponentStatus{
				Status: "unhealthy",
				Error:  err.Error(),
			}
			healthy = false
		} else {
			components[c.Name()] = ComponentStatus{
				Status: "healthy",
			}
		}
	}

	return healthy, components
}
