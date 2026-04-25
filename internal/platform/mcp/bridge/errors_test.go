package bridge

import (
	"errors"
	"os"
	"testing"
	"time"
)

func TestGetMcpToolTimeoutDefault(t *testing.T) {
	// Ensure env is clean.
	os.Unsetenv(envMcpToolTimeout)
	got := getMcpToolTimeout()
	if got != defaultMcpToolTimeout {
		t.Fatalf("default timeout = %v, want %v", got, defaultMcpToolTimeout)
	}
}

func TestGetMcpToolTimeoutEnvOverride(t *testing.T) {
	os.Setenv(envMcpToolTimeout, "5000")
	defer os.Unsetenv(envMcpToolTimeout)
	got := getMcpToolTimeout()
	if got != 5*time.Second {
		t.Fatalf("env timeout = %v, want 5s", got)
	}
}

func TestGetMcpToolTimeoutEnvInvalid(t *testing.T) {
	os.Setenv(envMcpToolTimeout, "not-a-number")
	defer os.Unsetenv(envMcpToolTimeout)
	got := getMcpToolTimeout()
	if got != defaultMcpToolTimeout {
		t.Fatalf("invalid env timeout = %v, want default %v", got, defaultMcpToolTimeout)
	}
}

func TestMcpAuthError(t *testing.T) {
	e := &McpAuthError{ServerName: "srv", Message: "token expired"}
	if e.Error() != "mcp auth error [srv]: token expired" {
		t.Fatalf("error message = %q", e.Error())
	}
	if !e.IsAuthError() {
		t.Fatal("expected IsAuthError() = true")
	}
	if e.IsSessionExpired() {
		t.Fatal("expected IsSessionExpired() = false")
	}
}

func TestMcpSessionExpiredError(t *testing.T) {
	e := &McpSessionExpiredError{ServerName: "srv"}
	if e.Error() != "mcp session expired [srv]" {
		t.Fatalf("error message = %q", e.Error())
	}
	if e.IsAuthError() {
		t.Fatal("expected IsAuthError() = false")
	}
	if !e.IsSessionExpired() {
		t.Fatal("expected IsSessionExpired() = true")
	}
}

func TestMcpToolCallError(t *testing.T) {
	e := &McpToolCallError{ServerName: "srv", ToolName: "tool", Message: "bad args"}
	if e.Error() != "mcp tool call error [srv/tool]: bad args" {
		t.Fatalf("error message = %q", e.Error())
	}
	if e.IsAuthError() {
		t.Fatal("expected IsAuthError() = false")
	}
	if e.IsSessionExpired() {
		t.Fatal("expected IsSessionExpired() = false")
	}
}

func TestClassifyMcpErrorNil(t *testing.T) {
	got := classifyMcpError("srv", "tool", nil)
	if got != nil {
		t.Fatalf("classify nil = %v, want nil", got)
	}
}

func TestClassifyMcpErrorAuth(t *testing.T) {
	cases := []string{
		"server returned 401",
		"Unauthorized request",
		"token expired",
		"authentication failed",
	}
	for _, c := range cases {
		err := errors.New(c)
		got := classifyMcpError("srv", "tool", err)
		authErr, ok := got.(*McpAuthError)
		if !ok {
			t.Fatalf("classify %q = %T, want *McpAuthError", c, got)
		}
		if authErr.ServerName != "srv" {
			t.Fatalf("auth error server = %q, want srv", authErr.ServerName)
		}
	}
}

func TestClassifyMcpErrorSession(t *testing.T) {
	cases := []string{
		"connection closed",
		"server returned 404",
		"session expired",
		"session not found",
	}
	for _, c := range cases {
		err := errors.New(c)
		got := classifyMcpError("srv", "tool", err)
		sessErr, ok := got.(*McpSessionExpiredError)
		if !ok {
			t.Fatalf("classify %q = %T, want *McpSessionExpiredError", c, got)
		}
		if sessErr.ServerName != "srv" {
			t.Fatalf("session error server = %q, want srv", sessErr.ServerName)
		}
	}
}

func TestClassifyMcpErrorUnchanged(t *testing.T) {
	original := errors.New("generic failure")
	got := classifyMcpError("srv", "tool", original)
	if got != original {
		t.Fatal("expected unclassified error to be returned unchanged")
	}
}
