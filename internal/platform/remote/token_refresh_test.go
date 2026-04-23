package remote

import (
	"net/http"
	"os"
	"testing"
)

func TestEnvTokenProvider_Token(t *testing.T) {
	// Ensure no env var is set initially
	t.Setenv("CLAUDE_CODE_SESSION_ACCESS_TOKEN", "")
	t.Setenv("CLAUDE_SESSION_INGRESS_TOKEN_FILE", "")

	provider := NewEnvTokenProvider()
	if got := provider.Token(); got != "" {
		t.Fatalf("expected empty token, got %q", got)
	}
}

func TestEnvTokenProvider_Token_FromEnv(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ACCESS_TOKEN", "test-token-123")

	provider := NewEnvTokenProvider()
	if got := provider.Token(); got != "test-token-123" {
		t.Fatalf("expected token from env, got %q", got)
	}

	state := provider.AuthState()
	if state.Source != "env" {
		t.Fatalf("expected source=env, got %q", state.Source)
	}
	if state.Token != "test-token-123" {
		t.Fatalf("expected token in state, got %q", state.Token)
	}
}

func TestEnvTokenProvider_Refresh(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ACCESS_TOKEN", "old-token")

	provider := NewEnvTokenProvider()
	if got := provider.Token(); got != "old-token" {
		t.Fatalf("expected initial token, got %q", got)
	}

	// Update env var to simulate parent process injecting a new token
	t.Setenv("CLAUDE_CODE_SESSION_ACCESS_TOKEN", "new-token")

	newToken, err := provider.Refresh()
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if newToken != "new-token" {
		t.Fatalf("expected new token after refresh, got %q", newToken)
	}
	if provider.Token() != "new-token" {
		t.Fatalf("expected cached token updated, got %q", provider.Token())
	}

	state := provider.AuthState()
	if state.RefreshCount != 1 {
		t.Fatalf("expected refresh count 1, got %d", state.RefreshCount)
	}
	if !state.RefreshedAt.IsZero() {
		// RefreshedAt should be set
	} else {
		t.Fatal("expected RefreshedAt to be set")
	}
}

func TestEnvTokenProvider_Refresh_NoToken(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ACCESS_TOKEN", "")
	t.Setenv("CLAUDE_SESSION_INGRESS_TOKEN_FILE", "")

	provider := NewEnvTokenProvider()
	_, err := provider.Refresh()
	if err == nil {
		t.Fatal("expected error when no token available")
	}

	state := provider.AuthState()
	if state.RefreshCount != 1 {
		t.Fatalf("expected refresh count 1 even on failure, got %d", state.RefreshCount)
	}
}

func TestEnvTokenProvider_Refresh_SameToken(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ACCESS_TOKEN", "same-token")

	provider := NewEnvTokenProvider()
	newToken, err := provider.Refresh()
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if newToken != "same-token" {
		t.Fatalf("expected same token, got %q", newToken)
	}
}

func TestEnvTokenProvider_Refresh_FromFile(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ACCESS_TOKEN", "")

	f := createTempTokenFile(t, "file-token")
	t.Setenv("CLAUDE_SESSION_INGRESS_TOKEN_FILE", f)

	provider := NewEnvTokenProvider()
	if got := provider.Token(); got != "file-token" {
		t.Fatalf("expected token from file, got %q", got)
	}

	state := provider.AuthState()
	if state.Source != "file" {
		t.Fatalf("expected source=file, got %q", state.Source)
	}
}

func TestApplyAuthHeader_Bearer(t *testing.T) {
	req, _ := http.NewRequest("GET", "/test", nil)
	applyAuthHeader(req, "jwt-token-123")
	if got := req.Header.Get("Authorization"); got != "Bearer jwt-token-123" {
		t.Fatalf("expected Bearer auth, got %q", got)
	}
}

func TestApplyAuthHeader_SessionKey(t *testing.T) {
	req, _ := http.NewRequest("GET", "/test", nil)
	t.Setenv("CLAUDE_CODE_ORGANIZATION_UUID", "org-uuid-123")
	applyAuthHeader(req, "sk-ant-sid01-test")
	if got := req.Header.Get("Cookie"); got != "sessionKey=sk-ant-sid01-test" {
		t.Fatalf("expected Cookie auth, got %q", got)
	}
	if got := req.Header.Get("X-Organization-Uuid"); got != "org-uuid-123" {
		t.Fatalf("expected org UUID header, got %q", got)
	}
}

func TestApplyAuthHeader_Empty(t *testing.T) {
	req, _ := http.NewRequest("GET", "/test", nil)
	applyAuthHeader(req, "")
	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("expected no auth header, got %q", got)
	}
}

func TestResolveAuthTokenWithSource(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ACCESS_TOKEN", "env-token")
	t.Setenv("CLAUDE_SESSION_INGRESS_TOKEN_FILE", "")

	token, source := resolveAuthTokenWithSource()
	if token != "env-token" {
		t.Fatalf("expected env token, got %q", token)
	}
	if source != "env" {
		t.Fatalf("expected source=env, got %q", source)
	}
}

func createTempTokenFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "token-*.txt")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return f.Name()
}
