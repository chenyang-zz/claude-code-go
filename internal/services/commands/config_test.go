package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
)

// TestConfigCommandMetadataExposesSettingsAlias verifies /config publishes the source-compatible /settings alias.
func TestConfigCommandMetadataExposesSettingsAlias(t *testing.T) {
	meta := ConfigCommand{}.Metadata()
	if meta.Name != "config" {
		t.Fatalf("Metadata().Name = %q, want config", meta.Name)
	}
	if len(meta.Aliases) != 1 || meta.Aliases[0] != "settings" {
		t.Fatalf("Metadata().Aliases = %#v, want []string{\"settings\"}", meta.Aliases)
	}
}

// TestConfigCommandExecuteRendersResolvedConfig verifies /config returns a stable text snapshot of the current runtime config.
func TestConfigCommandExecuteRendersResolvedConfig(t *testing.T) {
	result, err := ConfigCommand{
		Config: coreconfig.Config{
			Provider:      "anthropic",
			Model:         "claude-sonnet-4-5",
			ProjectPath:   "/repo",
			ApprovalMode:  "default",
			SessionDBPath: "/tmp/claude.db",
			APIKey:        "secret",
			APIBaseURL:    "https://example.invalid",
		},
	}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Current configuration:\n- Provider: anthropic\n- Model: claude-sonnet-4-5\n- Project path: /repo\n- Approval mode: default\n- Session DB path: /tmp/claude.db\n- API key: configured\n- API base URL: https://example.invalid"
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}
