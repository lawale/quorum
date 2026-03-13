package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/lawale/quorum/internal/model"
)

var (
	ErrPermissionDenied = fmt.Errorf("permission denied by external check")
)

// PermissionChecker calls the consuming system's permission endpoint
// to determine if a checker is allowed to act on a request.
type PermissionChecker struct {
	client *http.Client
}

func NewPermissionChecker(timeout time.Duration) *PermissionChecker {
	return &PermissionChecker{
		client: &http.Client{Timeout: timeout},
	}
}

// Check calls the permission check URL and returns whether the checker is allowed.
// Returns (true, nil) if no permission URL is configured (skip check).
func (p *PermissionChecker) Check(ctx context.Context, permissionURL string, checkReq model.PermissionCheckRequest) error {
	body, err := json.Marshal(checkReq)
	if err != nil {
		return fmt.Errorf("marshaling permission check request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, permissionURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating permission check request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "Quorum/1.0")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("calling permission check endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("permission check endpoint returned status %d", resp.StatusCode)
	}

	var checkResp model.PermissionCheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&checkResp); err != nil {
		return fmt.Errorf("decoding permission check response: %w", err)
	}

	if !checkResp.Allowed {
		if checkResp.Reason != "" {
			return fmt.Errorf("%w: %s", ErrPermissionDenied, checkResp.Reason)
		}
		return ErrPermissionDenied
	}

	return nil
}
