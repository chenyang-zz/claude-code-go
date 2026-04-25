package prompts

import (
	"context"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/platform/mcp/client"
	mcpregistry "github.com/sheepzhao/claude-code-go/internal/platform/mcp/registry"
)

// TestMCPInstructionsSection_Compute verifies server instructions are omitted when the registry has no guidance.
func TestMCPInstructionsSection_Compute(t *testing.T) {
	registry := mcpregistry.NewServerRegistry()
	registry.LoadConfigs(map[string]client.ServerConfig{
		"files": {Command: "echo"},
	})
	mcpregistry.SetLastRegistry(registry)

	section := MCPInstructionsSection{}
	got, err := section.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if got != "" {
		t.Fatalf("Compute() = %q, want empty string when no connected registry is present", got)
	}
}

// TestMCPInstructionsSection_ComputeWithRegistry verifies instructions are emitted from the last registry.
func TestMCPInstructionsSection_ComputeWithRegistry(t *testing.T) {
	registry := mcpregistry.NewServerRegistry()
	registry.LoadConfigs(map[string]client.ServerConfig{
		"files": {Command: "echo"},
	})
	if len(registry.List()) != 1 {
		t.Fatalf("expected one registry entry, got %d", len(registry.List()))
	}
	registry.SetInstructions("files", "Use this server for workspace file lookups.")
	// Reinstall the mutated snapshot so the section can read it.
	mcpregistry.SetLastRegistry(registry)

	section := MCPInstructionsSection{}
	got, err := section.Compute(context.Background())
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if !strings.Contains(got, "# MCP Server Instructions") {
		t.Fatalf("Compute() = %q, want MCP instructions header", got)
	}
	if !strings.Contains(got, "tools, resources, and prompts") {
		t.Fatalf("Compute() = %q, want updated capability wording", got)
	}
	if !strings.Contains(got, "Use this server for workspace file lookups.") {
		t.Fatalf("Compute() = %q, want server instructions", got)
	}
}
