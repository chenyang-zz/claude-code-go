package commands

import (
	"context"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	mcpregistry "github.com/sheepzhao/claude-code-go/internal/platform/mcp/registry"
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
	if !strings.Contains(meta.Usage, "detail") {
		t.Fatalf("Metadata().Usage = %q, should contain detail subcommand", meta.Usage)
	}
}

// TestMCPCommandExecuteReportsFallback verifies /mcp reports the current Go host fallback before MCP management is migrated.
func TestMCPCommandExecuteReportsFallback(t *testing.T) {
	result, err := MCPCommand{}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "No MCP servers configured. Set CLAUDE_CODE_MCP_SERVERS to configure servers."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}

// TestMCPCommandExecuteDetailNotFound verifies /mcp detail returns an error for unknown servers.
func TestMCPCommandExecuteDetailNotFound(t *testing.T) {
	reg := mcpregistry.NewServerRegistry()
	mcpregistry.SetLastRegistry(reg)
	defer mcpregistry.SetLastRegistry(nil)

	cmd := MCPCommand{}
	result, err := cmd.Execute(context.Background(), command.Args{Raw: []string{"detail", "nonexistent"}})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(result.Output, "not found") {
		t.Fatalf("expected 'not found' in output, got: %q", result.Output)
	}
}

// TestMCPCommandExecuteDetailEmptyRegistry verifies /mcp detail with empty registry.
func TestMCPCommandExecuteDetailEmptyRegistry(t *testing.T) {
	reg := mcpregistry.NewServerRegistry()
	mcpregistry.SetLastRegistry(reg)
	defer mcpregistry.SetLastRegistry(nil)

	cmd := MCPCommand{}
	result, err := cmd.Execute(context.Background(), command.Args{Raw: []string{"detail", "missing"}})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(result.Output, "not found") {
		t.Fatalf("expected 'not found' in output, got: %q", result.Output)
	}
}

// TestMCPCommandExecuteListEmptyRegistry verifies /mcp with an empty registry.
func TestMCPCommandExecuteListEmptyRegistry(t *testing.T) {
	reg := mcpregistry.NewServerRegistry()
	mcpregistry.SetLastRegistry(reg)
	defer mcpregistry.SetLastRegistry(nil)

	cmd := MCPCommand{}
	result, err := cmd.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output != "No MCP servers configured." {
		t.Fatalf("Execute() output = %q, want 'No MCP servers configured.'", result.Output)
	}
}
