package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// TestMCPCommandMetadata verifies /mcp exposes stable metadata.
func TestMCPCommandMetadata(t *testing.T) {
	meta := MCPCommand{}.Metadata()
	if meta.Name != "mcp" {
		t.Fatalf("Metadata().Name = %q, want mcp", meta.Name)
	}
	if meta.Description != "Manage MCP servers" {
		t.Fatalf("Metadata().Description = %q, want stable mcp description", meta.Description)
	}
	if meta.Usage != "/mcp [enable|disable <server-name>]" {
		t.Fatalf("Metadata().Usage = %q, want mcp usage", meta.Usage)
	}
}

// TestMCPCommandExecuteReportsFallback verifies /mcp reports the current Go host fallback before MCP management is migrated.
func TestMCPCommandExecuteReportsFallback(t *testing.T) {
	result, err := MCPCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "MCP server management is not available in Claude Code Go yet. Configure MCP servers before startup instead."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}
