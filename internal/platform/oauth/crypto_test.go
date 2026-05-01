package oauth

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestGenerateCodeVerifier_Length(t *testing.T) {
	verifier, err := GenerateCodeVerifier()
	if err != nil {
		t.Fatalf("GenerateCodeVerifier returned error: %v", err)
	}
	// 32 random bytes, base64url-encoded without padding => ceil(32 * 4 / 3) = 43.
	if len(verifier) != 43 {
		t.Fatalf("verifier length = %d, want 43", len(verifier))
	}
}

func TestGenerateCodeVerifier_CharsetIsBase64URL(t *testing.T) {
	verifier, err := GenerateCodeVerifier()
	if err != nil {
		t.Fatalf("GenerateCodeVerifier returned error: %v", err)
	}
	if strings.ContainsAny(verifier, "+/=") {
		t.Fatalf("verifier %q contains forbidden base64-standard characters", verifier)
	}
	if _, err := base64.RawURLEncoding.DecodeString(verifier); err != nil {
		t.Fatalf("verifier is not valid base64url: %v", err)
	}
}

func TestGenerateCodeVerifier_Random(t *testing.T) {
	a, err := GenerateCodeVerifier()
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	b, err := GenerateCodeVerifier()
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if a == b {
		t.Fatalf("two consecutive calls returned the same verifier %q", a)
	}
}

func TestGenerateCodeChallenge_Deterministic(t *testing.T) {
	const verifier = "test-verifier-string-1234567890"
	c1 := GenerateCodeChallenge(verifier)
	c2 := GenerateCodeChallenge(verifier)
	if c1 != c2 {
		t.Fatalf("challenge for same verifier differed: %q vs %q", c1, c2)
	}
}

func TestGenerateCodeChallenge_Length(t *testing.T) {
	verifier, err := GenerateCodeVerifier()
	if err != nil {
		t.Fatalf("GenerateCodeVerifier returned error: %v", err)
	}
	challenge := GenerateCodeChallenge(verifier)
	// sha256 produces 32 bytes; base64url without padding => 43 chars.
	if len(challenge) != 43 {
		t.Fatalf("challenge length = %d, want 43", len(challenge))
	}
	if strings.ContainsAny(challenge, "+/=") {
		t.Fatalf("challenge %q contains forbidden base64-standard characters", challenge)
	}
}

// TestGenerateCodeChallenge_RFC7636Vector verifies the challenge derivation
// against the test vector in RFC 7636 Appendix B.
func TestGenerateCodeChallenge_RFC7636Vector(t *testing.T) {
	const verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	const want = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	got := GenerateCodeChallenge(verifier)
	if got != want {
		t.Fatalf("RFC 7636 challenge mismatch:\n  got:  %s\n  want: %s", got, want)
	}
}

func TestGenerateState_Length(t *testing.T) {
	state, err := GenerateState()
	if err != nil {
		t.Fatalf("GenerateState returned error: %v", err)
	}
	if len(state) != 43 {
		t.Fatalf("state length = %d, want 43", len(state))
	}
	if strings.ContainsAny(state, "+/=") {
		t.Fatalf("state %q contains forbidden base64-standard characters", state)
	}
}

func TestGenerateState_Random(t *testing.T) {
	a, err := GenerateState()
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	b, err := GenerateState()
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if a == b {
		t.Fatalf("two consecutive calls returned the same state %q", a)
	}
}

func TestGenerateState_VerifierIndependence(t *testing.T) {
	verifier, err := GenerateCodeVerifier()
	if err != nil {
		t.Fatalf("GenerateCodeVerifier returned error: %v", err)
	}
	state, err := GenerateState()
	if err != nil {
		t.Fatalf("GenerateState returned error: %v", err)
	}
	if verifier == state {
		t.Fatalf("verifier and state should not collide, both = %q", verifier)
	}
}
