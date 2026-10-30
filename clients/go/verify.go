package client

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// SignatureHeader carries the HMAC-SHA256 of a webhook delivery's body.
const SignatureHeader = "X-Signature-256"

// MaxWebhookBody is the largest delivery ParseWebhook will read.
const MaxWebhookBody = 4 << 20

// ErrInvalidSignature means a delivery did not come from a Quorum holding the
// subscription's secret, or was altered in transit.
var ErrInvalidSignature = errors.New("quorum: webhook signature invalid")

// ParseWebhook reads a delivery from an incoming request, verifies its
// signature against secret and decodes it. Reject the request when it returns
// an error; respond 2xx once you have durably recorded the event, otherwise
// Quorum retries.
func ParseWebhook(r *http.Request, secret string) (*WebhookPayload, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, MaxWebhookBody+1))
	if err != nil {
		return nil, fmt.Errorf("quorum: reading webhook body: %w", err)
	}
	if len(body) > MaxWebhookBody {
		return nil, errors.New("quorum: webhook body too large")
	}
	return ParseWebhookBytes(body, r.Header.Get(SignatureHeader), secret)
}

// ParseWebhookBytes is ParseWebhook for a body and signature header you have
// already read, for frameworks that consume the request themselves.
func ParseWebhookBytes(body []byte, signature, secret string) (*WebhookPayload, error) {
	if !VerifySignature(body, signature, secret) {
		return nil, ErrInvalidSignature
	}
	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("quorum: decoding webhook: %w", err)
	}
	return &payload, nil
}

// VerifySignature reports whether signature, in the form "sha256=<hex>", is
// the HMAC-SHA256 of body under secret.
func VerifySignature(body []byte, signature, secret string) bool {
	hexSig, ok := strings.CutPrefix(signature, "sha256=")
	if !ok || secret == "" {
		return false
	}
	got, err := hex.DecodeString(hexSig)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hmac.Equal(got, mac.Sum(nil))
}

// Sign produces the signature header value Quorum would send for body. It is
// for tests and for proxies that re-sign deliveries.
func Sign(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
