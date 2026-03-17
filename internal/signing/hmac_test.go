package signing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestComputeHMAC(t *testing.T) {
	payload := []byte(`{"request_id":"abc","checker_id":"user-1"}`)
	secret := "my-secret-key"

	result := ComputeHMAC(payload, secret)

	// Verify independently
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))

	if result != expected {
		t.Errorf("ComputeHMAC = %q, want %q", result, expected)
	}
}

func TestComputeHMAC_DifferentSecrets_DifferentResults(t *testing.T) {
	payload := []byte(`{"data":"test"}`)

	sig1 := ComputeHMAC(payload, "secret-1")
	sig2 := ComputeHMAC(payload, "secret-2")

	if sig1 == sig2 {
		t.Error("expected different signatures for different secrets")
	}
}

func TestComputeHMAC_DifferentPayloads_DifferentResults(t *testing.T) {
	secret := "shared-secret"

	sig1 := ComputeHMAC([]byte(`{"a":1}`), secret)
	sig2 := ComputeHMAC([]byte(`{"a":2}`), secret)

	if sig1 == sig2 {
		t.Error("expected different signatures for different payloads")
	}
}

func TestComputeHMAC_Deterministic(t *testing.T) {
	payload := []byte(`{"data":"test"}`)
	secret := "key"

	sig1 := ComputeHMAC(payload, secret)
	sig2 := ComputeHMAC(payload, secret)

	if sig1 != sig2 {
		t.Error("expected identical signatures for same inputs")
	}
}
