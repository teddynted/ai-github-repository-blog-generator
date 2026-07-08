// Package githubsig verifies GitHub webhook signatures. GitHub signs each
// delivery with HMAC-SHA256 over the raw request body using the webhook's
// secret, sending the result in the X-Hub-Signature-256 header as
// "sha256=<hex>". Verification uses a constant-time comparison.
package githubsig

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const prefix = "sha256="

// Verify reports whether signatureHeader is a valid HMAC-SHA256 signature of
// body under secret. It is constant-time and returns false for a missing or
// malformed header or an empty secret.
func Verify(secret string, body []byte, signatureHeader string) bool {
	if secret == "" || !strings.HasPrefix(signatureHeader, prefix) {
		return false
	}
	want := signatureHeader[len(prefix):]

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	got := hex.EncodeToString(mac.Sum(nil))

	// hmac.Equal is constant-time for equal-length inputs.
	return hmac.Equal([]byte(got), []byte(want))
}

// Sign produces the header value for a body/secret. Primarily for tests and
// local tooling.
func Sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return prefix + hex.EncodeToString(mac.Sum(nil))
}
