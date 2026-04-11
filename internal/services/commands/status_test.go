package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
)

// TestStatusCommandMetadata verifies /status exposes stable metadata.
func TestStatusCommandMetadata(t *testing.T) {
	meta := StatusCommand{}.Metadata()
	if meta.Name != "status" {
		t.Fatalf("Metadata().Name = %q, want status", meta.Name)
	}
	if meta.Description != "Show Claude Code status including version, model, account, API connectivity, and tool statuses" {
		t.Fatalf("Metadata().Description = %q, want stable status description", meta.Description)
	}
	if meta.Usage != "/status" {
		t.Fatalf("Metadata().Usage = %q, want /status", meta.Usage)
	}
}

// TestStatusCommandExecute verifies /status reports the current Go host summary and stable fallback boundaries.
func TestStatusCommandExecute(t *testing.T) {
	result, err := StatusCommand{
		Config: coreconfig.Config{
			Provider:      "anthropic",
			Model:         "claude-sonnet-4-5",
			ProjectPath:   "/repo/project",
			ApprovalMode:  "default",
			SessionDBPath: "/tmp/sessions.db",
			APIKey:        "test-key",
		},
	}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Status summary:\n- Provider: anthropic\n- Model: claude-sonnet-4-5\n- Project path: /repo/project\n- Approval mode: default\n- Session storage: configured (/tmp/sessions.db)\n- Account auth: API key configured; interactive account status is not available\n- API base URL: default\n- API connectivity check: not available in Claude Code Go yet\n- Tool status checks: not available in Claude Code Go yet\n- Settings status UI: not available in Claude Code Go yet"
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}

// TestStatusCommandExecuteWithoutAPIKey verifies /status keeps the missing-account fallback stable.
func TestStatusCommandExecuteWithoutAPIKey(t *testing.T) {
	result, err := StatusCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Status summary:\n- Provider: (not set)\n- Model: (not set)\n- Project path: (not set)\n- Approval mode: (not set)\n- Session storage: not configured\n- Account auth: missing API key; interactive account status is not available\n- API base URL: default\n- API connectivity check: not available in Claude Code Go yet\n- Tool status checks: not available in Claude Code Go yet\n- Settings status UI: not available in Claude Code Go yet"
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}
