// Package oauth implements the Anthropic / Claude.ai account OAuth login flow.
// It mirrors the TypeScript src/services/oauth/ package: PKCE crypto helpers,
// a localhost callback listener, an authorization-code exchange client, profile
// retrieval, and a service that orchestrates the end-to-end login flow.
package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// codeVerifierBytes is the random byte length used by the PKCE code verifier
// and OAuth state parameter. 32 bytes (256 bits) matches the TypeScript
// implementation in src/services/oauth/crypto.ts.
const codeVerifierBytes = 32

// GenerateCodeVerifier returns a fresh PKCE code verifier as defined in
// RFC 7636 §4.1: 32 random bytes encoded as base64url without padding.
func GenerateCodeVerifier() (string, error) {
	b := make([]byte, codeVerifierBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate code verifier: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// GenerateCodeChallenge derives the PKCE code challenge for the supplied
// verifier using the S256 method (RFC 7636 §4.2): sha256(verifier) encoded as
// base64url without padding. The verifier must already be url-safe; passing an
// empty string produces a deterministic challenge for the empty input.
func GenerateCodeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// GenerateState returns a fresh OAuth state parameter used for CSRF protection.
// 32 random bytes encoded as base64url without padding, matching the
// TypeScript reference implementation.
func GenerateState() (string, error) {
	b := make([]byte, codeVerifierBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
