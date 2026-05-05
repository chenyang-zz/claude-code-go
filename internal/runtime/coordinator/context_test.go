package coordinator

import (
	"strings"
	"testing"
)

func TestBuildCoordinatorUserContext_Empty(t *testing.T) {
	ctx := BuildCoordinatorUserContext(nil, nil, "")
	if len(ctx.MCPServerNames) != 0 {
		t.Fatalf("MCPServerNames = %v, want empty", ctx.MCPServerNames)
	}
	if ctx.ScratchpadDir != "" {
		t.Fatalf("ScratchpadDir = %q, want empty", ctx.ScratchpadDir)
	}
}

func TestBuildCoordinatorUserContext_WithScratchpad(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_COORDINATOR_SCRATCHPAD", "1")
	ctx := BuildCoordinatorUserContext(nil, nil, "/tmp/scratch")
	if ctx.ScratchpadDir != "/tmp/scratch" {
		t.Fatalf("ScratchpadDir = %q, want /tmp/scratch", ctx.ScratchpadDir)
	}
}

func TestBuildCoordinatorUserContext_ScratchpadDisabled(t *testing.T) {
	t.Setenv("CLAUDE_FEATURE_COORDINATOR_SCRATCHPAD", "0")
	ctx := BuildCoordinatorUserContext(nil, nil, "/tmp/scratch")
	if ctx.ScratchpadDir != "" {
		t.Fatalf("ScratchpadDir = %q, want empty when flag disabled", ctx.ScratchpadDir)
	}
}

func TestRenderWorkerToolsContext_FullMode(t *testing.T) {
	ctx := CoordinatorUserContext{
		EnabledToolNames: map[string]struct{}{"Bash": {}, "Read": {}},
	}
	result := RenderWorkerToolsContext(ctx, false)
	if !strings.Contains(result, "Workers spawned via the Agent tool have access to these tools:") {
		t.Fatalf("missing worker tools header")
	}
	if !strings.Contains(result, "Bash") {
		t.Fatalf("missing Bash in tools")
	}
}

func TestRenderWorkerToolsContext_SimpleMode(t *testing.T) {
	ctx := CoordinatorUserContext{}
	result := RenderWorkerToolsContext(ctx, true)
	if !strings.Contains(result, "Bash, Read, Edit") {
		t.Fatalf("simple mode = %q, want Bash, Read, Edit", result)
	}
}

func TestRenderWorkerToolsContext_WithMCPServers(t *testing.T) {
	ctx := CoordinatorUserContext{
		MCPServerNames: []string{"github", "jira"},
	}
	result := RenderWorkerToolsContext(ctx, false)
	if !strings.Contains(result, "MCP tools from connected MCP servers: github, jira") {
		t.Fatalf("missing MCP server list in: %s", result)
	}
}

func TestRenderWorkerToolsContext_WithScratchpad(t *testing.T) {
	ctx := CoordinatorUserContext{
		ScratchpadDir: "/tmp/scratch",
	}
	result := RenderWorkerToolsContext(ctx, false)
	if !strings.Contains(result, "Scratchpad directory: /tmp/scratch") {
		t.Fatalf("missing scratchpad in: %s", result)
	}
}

func TestIsSimpleMode_Disabled(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SIMPLE", "")
	if IsSimpleMode() {
		t.Fatalf("IsSimpleMode() = true, want false")
	}
}

func TestIsSimpleMode_Enabled(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SIMPLE", "1")
	if !IsSimpleMode() {
		t.Fatalf("IsSimpleMode() = false, want true")
	}
}
