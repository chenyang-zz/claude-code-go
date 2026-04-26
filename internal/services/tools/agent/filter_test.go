package agent

import (
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

func makeCatalog(names ...string) []model.ToolDefinition {
	defs := make([]model.ToolDefinition, len(names))
	for i, n := range names {
		defs[i] = model.ToolDefinition{Name: n}
	}
	return defs
}

func TestResolveAgentTools_Wildcard(t *testing.T) {
	catalog := makeCatalog("Read", "Write", "Edit", "Bash", "Glob")

	// nil allowlist → wildcard
	def := agent.Definition{Source: "built-in"}
	result := resolveAgentTools(def, catalog)
	if !result.HasWildcard {
		t.Errorf("expected HasWildcard=true for nil allowlist")
	}
	if len(result.Tools) != len(catalog) {
		t.Errorf("wildcard: expected %d tools, got %d", len(catalog), len(result.Tools))
	}

	// ["*"] allowlist → wildcard
	def = agent.Definition{Source: "built-in", Tools: []string{"*"}}
	result = resolveAgentTools(def, catalog)
	if !result.HasWildcard {
		t.Errorf("expected HasWildcard=true for [*] allowlist")
	}
}

func TestResolveAgentTools_Allowlist(t *testing.T) {
	catalog := makeCatalog("Read", "Write", "Edit", "Bash", "Glob")

	def := agent.Definition{
		Source: "built-in",
		Tools:  []string{"Read", "Glob"},
	}
	result := resolveAgentTools(def, catalog)
	if result.HasWildcard {
		t.Errorf("expected HasWildcard=false for explicit allowlist")
	}
	if len(result.Tools) != 2 {
		t.Errorf("allowlist: expected 2 tools, got %d", len(result.Tools))
	}
	names := toolNames(result.Tools)
	if !contains(names, "Read") || !contains(names, "Glob") {
		t.Errorf("allowlist: expected Read+Glob, got %v", names)
	}
	if len(result.ValidSpecs) != 2 {
		t.Errorf("allowlist: expected 2 valid specs, got %d", len(result.ValidSpecs))
	}
	if len(result.InvalidSpecs) != 0 {
		t.Errorf("allowlist: expected 0 invalid specs, got %d", len(result.InvalidSpecs))
	}
}

func TestResolveAgentTools_AllowlistWithInvalid(t *testing.T) {
	catalog := makeCatalog("Read", "Write")

	def := agent.Definition{
		Source: "built-in",
		Tools:  []string{"Read", "NonExistent", "Write"},
	}
	result := resolveAgentTools(def, catalog)
	if len(result.Tools) != 2 {
		t.Errorf("expected 2 resolved tools, got %d", len(result.Tools))
	}
	if len(result.ValidSpecs) != 2 {
		t.Errorf("expected 2 valid specs, got %d", len(result.ValidSpecs))
	}
	if len(result.InvalidSpecs) != 1 || result.InvalidSpecs[0] != "NonExistent" {
		t.Errorf("expected 1 invalid spec 'NonExistent', got %v", result.InvalidSpecs)
	}
}

func TestResolveAgentTools_Denylist(t *testing.T) {
	catalog := makeCatalog("Read", "Write", "Edit", "Bash")

	def := agent.Definition{
		Source:          "built-in",
		DisallowedTools: []string{"Write", "Bash"},
	}
	result := resolveAgentTools(def, catalog)
	if !result.HasWildcard {
		t.Errorf("expected HasWildcard=true for denylist-only (nil Tools = wildcard)")
	}
	names := toolNames(result.Tools)
	if contains(names, "Write") || contains(names, "Bash") {
		t.Errorf("denylist: Write and Bash should be removed, got %v", names)
	}
	if !contains(names, "Read") || !contains(names, "Edit") {
		t.Errorf("denylist: Read and Edit should remain, got %v", names)
	}
}

func TestResolveAgentTools_AllowlistAndDenylist(t *testing.T) {
	catalog := makeCatalog("Read", "Write", "Edit", "Bash")

	def := agent.Definition{
		Source:          "built-in",
		Tools:           []string{"Read", "Write", "Edit"},
		DisallowedTools: []string{"Write"},
	}
	result := resolveAgentTools(def, catalog)
	names := toolNames(result.Tools)
	// Allowlist restricts to Read/Write/Edit; denylist removes Write.
	if len(names) != 2 {
		t.Errorf("expected 2 tools, got %d: %v", len(names), names)
	}
	if contains(names, "Write") {
		t.Errorf("Write should be removed by denylist")
	}
	if !contains(names, "Read") || !contains(names, "Edit") {
		t.Errorf("Read and Edit should remain, got %v", names)
	}
}

func TestResolveAgentTools_EmptyAllowlist(t *testing.T) {
	catalog := makeCatalog("Read", "Write")

	def := agent.Definition{
		Source: "built-in",
		Tools:  []string{},
	}
	result := resolveAgentTools(def, catalog)
	if len(result.Tools) != 0 {
		t.Errorf("empty allowlist: expected 0 tools, got %d", len(result.Tools))
	}
}

func TestResolveAgentTools_BuiltInVsCustomDefaults(t *testing.T) {
	catalog := makeCatalog("Agent", "TaskStop", "Read", "Write")

	// Built-in agent: default disallowed set applies.
	builtIn := agent.Definition{Source: "built-in"}
	result := resolveAgentTools(builtIn, catalog)
	names := toolNames(result.Tools)
	if contains(names, "Agent") {
		t.Errorf("built-in: Agent should be disallowed by default")
	}
	if contains(names, "TaskStop") {
		t.Errorf("built-in: TaskStop should be disallowed by default")
	}
	if !contains(names, "Read") || !contains(names, "Write") {
		t.Errorf("built-in: Read and Write should remain, got %v", names)
	}

	// Custom agent: same default set currently (CUSTOM_AGENT_DISALLOWED_TOOLS == ALL_AGENT_DISALLOWED_TOOLS).
	custom := agent.Definition{Source: "userSettings"}
	result = resolveAgentTools(custom, catalog)
	names = toolNames(result.Tools)
	if contains(names, "Agent") || contains(names, "TaskStop") {
		t.Errorf("custom: Agent and TaskStop should be disallowed by default")
	}
}

func TestResolveAgentTools_MCPPassthrough(t *testing.T) {
	catalog := makeCatalog("Read", "Write", "mcp__server1__toolA", "mcp__server2__toolB")

	// Even with a denylist that would match MCP tools, they should remain.
	def := agent.Definition{
		Source:          "built-in",
		DisallowedTools: []string{"mcp__server1__toolA"},
	}
	result := resolveAgentTools(def, catalog)
	names := toolNames(result.Tools)
	if !contains(names, "mcp__server1__toolA") {
		t.Errorf("MCP tool mcp__server1__toolA should pass through denylist")
	}
	if !contains(names, "mcp__server2__toolB") {
		t.Errorf("MCP tool mcp__server2__toolB should remain, got %v", names)
	}
	if !contains(names, "Write") {
		t.Errorf("Write should remain (not in denylist)")
	}
}

func TestResolveAgentTools_MCPPassthroughWithAllowlist(t *testing.T) {
	catalog := makeCatalog("Read", "Write", "mcp__server1__toolA")

	// Allowlist only permits Read — but MCP tools should still pass through.
	def := agent.Definition{
		Source: "built-in",
		Tools:  []string{"Read"},
	}
	result := resolveAgentTools(def, catalog)
	names := toolNames(result.Tools)
	if !contains(names, "Read") {
		t.Errorf("Read should be allowed")
	}
	if !contains(names, "mcp__server1__toolA") {
		t.Errorf("MCP tool should pass through allowlist, got %v", names)
	}
	if contains(names, "Write") {
		t.Errorf("Write should not be in allowlist")
	}
}

func TestFormatToolList(t *testing.T) {
	tests := []struct {
		name string
		def  agent.Definition
		want string
	}{
		{
			name: "no restrictions",
			def:  agent.Definition{},
			want: "All tools",
		},
		{
			name: "allowlist only",
			def:  agent.Definition{Tools: []string{"Read", "Glob"}},
			want: "Read, Glob",
		},
		{
			name: "denylist only",
			def:  agent.Definition{DisallowedTools: []string{"Write", "Bash"}},
			want: "All tools except Write, Bash",
		},
		{
			name: "both",
			def: agent.Definition{
				Tools:           []string{"Read", "Write", "Glob"},
				DisallowedTools: []string{"Write"},
			},
			want: "Read, Glob",
		},
		{
			name: "both empty",
			def: agent.Definition{
				Tools:           []string{"Read", "Write"},
				DisallowedTools: []string{"Read", "Write"},
			},
			want: "None",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatToolList(tc.def)
			if got != tc.want {
				t.Errorf("formatToolList() = %q, want %q", got, tc.want)
			}
		})
	}
}

func toolNames(tools []model.ToolDefinition) []string {
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Name
	}
	return names
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
