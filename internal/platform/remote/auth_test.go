package remote

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolveAuthTokenFromEnv verifies direct env-var token resolution.
func TestResolveAuthTokenFromEnv(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ACCESS_TOKEN", "token_from_env")
	// Ensure competing sources are absent.
	os.Unsetenv("CLAUDE_SESSION_INGRESS_TOKEN_FILE")

	got := ResolveAuthToken()
	if got != "token_from_env" {
		t.Fatalf("ResolveAuthToken() = %q, want token_from_env", got)
	}
}

// TestResolveAuthTokenFromFileEnv verifies token resolution from a file path in env.
func TestResolveAuthTokenFromFileEnv(t *testing.T) {
	os.Unsetenv("CLAUDE_CODE_SESSION_ACCESS_TOKEN")

	tmpDir := t.TempDir()
	tokenFile := filepath.Join(tmpDir, "token.txt")
	if err := os.WriteFile(tokenFile, []byte("token_from_file\n"), 0600); err != nil {
		t.Fatalf("write token file: %v", err)
	}

	t.Setenv("CLAUDE_SESSION_INGRESS_TOKEN_FILE", tokenFile)

	got := ResolveAuthToken()
	if got != "token_from_file" {
		t.Fatalf("ResolveAuthToken() = %q, want token_from_file", got)
	}
}

// TestResolveAuthTokenEmptyWhenNothingAvailable verifies empty result when no source exists.
func TestResolveAuthTokenEmptyWhenNothingAvailable(t *testing.T) {
	os.Unsetenv("CLAUDE_CODE_SESSION_ACCESS_TOKEN")
	os.Unsetenv("CLAUDE_SESSION_INGRESS_TOKEN_FILE")

	got := ResolveAuthToken()
	if got != "" {
		t.Fatalf("ResolveAuthToken() = %q, want empty string", got)
	}
}

// TestAuthHeadersWithToken verifies AuthHeaders includes Authorization when a token is present.
func TestAuthHeadersWithToken(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ACCESS_TOKEN", "my_token")

	headers := AuthHeaders()
	if got := headers["Authorization"]; got != "Bearer my_token" {
		t.Fatalf("Authorization = %q, want Bearer my_token", got)
	}
}

// TestAuthHeadersEmptyWhenNoToken verifies AuthHeaders is empty when no token is available.
func TestAuthHeadersEmptyWhenNoToken(t *testing.T) {
	os.Unsetenv("CLAUDE_CODE_SESSION_ACCESS_TOKEN")
	os.Unsetenv("CLAUDE_SESSION_INGRESS_TOKEN_FILE")

	headers := AuthHeaders()
	if len(headers) != 0 {
		t.Fatalf("AuthHeaders() = %v, want empty map", headers)
	}
}
