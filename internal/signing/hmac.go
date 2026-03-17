package signing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// ComputeHMAC returns the hex-encoded HMAC-SHA256 of payload using the given secret.
func ComputeHMAC(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
