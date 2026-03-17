package auth

import (
	"bytes"
	"context"
	"crypto/hmac"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/lawale/quorum/internal/model"
	"github.com/lawale/quorum/internal/signing"
)

var (
	ErrAuthorizationDenied = fmt.Errorf("authorization denied by dynamic hook")
)

// AuthorizationHook calls the consuming system's dynamic authorization endpoint
// to determine if a checker is allowed to act on a request.
type AuthorizationHook struct {
	client *http.Client
}

func NewAuthorizationHook(timeout time.Duration) *AuthorizationHook {
	return &AuthorizationHook{
		client: &http.Client{Timeout: timeout},
	}
}

func (h *AuthorizationHook) Check(ctx context.Context, hookURL string, secret string, hookReq model.AuthorizationHookRequest) error {
	body, err := json.Marshal(hookReq)
	if err != nil {
		return fmt.Errorf("marshaling authorization hook request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, hookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating authorization hook request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "Quorum/1.0")

	if secret != "" {
		sig := signing.ComputeHMAC(body, secret)
		httpReq.Header.Set("X-Signature-256", "sha256="+sig)
	}

	resp, err := h.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("calling authorization hook endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authorization hook endpoint returned status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading authorization hook response: %w", err)
	}

	if secret != "" {
		respSig := resp.Header.Get("X-Signature-256")
		if respSig == "" {
			return fmt.Errorf("authorization hook response missing X-Signature-256 header")
		}
		expectedSig := "sha256=" + signing.ComputeHMAC(bodyBytes, secret)
		if !hmac.Equal([]byte(respSig), []byte(expectedSig)) {
			return fmt.Errorf("authorization hook response signature mismatch")
		}
	}

	var hookResp model.AuthorizationHookResponse
	if err := json.Unmarshal(bodyBytes, &hookResp); err != nil {
		return fmt.Errorf("decoding authorization hook response: %w", err)
	}

	if !hookResp.Allowed {
		if hookResp.Reason != "" {
			return fmt.Errorf("%w: %s", ErrAuthorizationDenied, hookResp.Reason)
		}
		return ErrAuthorizationDenied
	}

	return nil
}
