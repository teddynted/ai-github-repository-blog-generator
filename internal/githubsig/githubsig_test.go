package githubsig

import "testing"

func TestSignAndVerifyRoundTrip(t *testing.T) {
	secret := "whsecret"
	body := []byte(`{"hello":"world"}`)
	sig := Sign(secret, body)
	if !Verify(secret, body, sig) {
		t.Fatalf("valid signature rejected: %s", sig)
	}
}

func TestVerifyRejectsTampering(t *testing.T) {
	secret := "whsecret"
	body := []byte(`{"hello":"world"}`)
	sig := Sign(secret, body)

	if Verify(secret, []byte(`{"hello":"mars"}`), sig) {
		t.Error("modified body should fail verification")
	}
	if Verify("wrong-secret", body, sig) {
		t.Error("wrong secret should fail verification")
	}
}

func TestVerifyRejectsMalformedHeaders(t *testing.T) {
	body := []byte("x")
	for _, h := range []string{"", "sha1=abc", "deadbeef", "sha256="} {
		if Verify("s", body, h) {
			t.Errorf("malformed header %q should not verify", h)
		}
	}
	if Verify("", body, Sign("s", body)) {
		t.Error("empty secret must not verify")
	}
}

// Known-answer test: HMAC-SHA256("It's a Secret to Everybody", "Hello, World!")
// is the value GitHub's documentation uses.
func TestKnownAnswer(t *testing.T) {
	got := Sign("It's a Secret to Everybody", []byte("Hello, World!"))
	want := "sha256=757107ea0eb2509fc211221cce984b8a37570b6d7586c22c46f4379c8b043e17"
	if got != want {
		t.Errorf("Sign = %s, want %s", got, want)
	}
}
