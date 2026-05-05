package coordinator

import (
	"strings"
	"testing"
)

func TestIsCoordinatorMode_Disabled(t *testing.T) {
	t.Setenv("CLAUDE_CODE_COORDINATOR_MODE", "")
	if IsCoordinatorMode() {
		t.Fatalf("IsCoordinatorMode() = true, want false")
	}
}

func TestIsCoordinatorMode_Enabled(t *testing.T) {
	t.Setenv("CLAUDE_CODE_COORDINATOR_MODE", "1")
	if !IsCoordinatorMode() {
		t.Fatalf("IsCoordinatorMode() = false, want true")
	}
}

func TestGetCoordinatorSystemPrompt_FullContent(t *testing.T) {
	prompt := GetCoordinatorSystemPrompt("Bash, Read", "", "")

	// Verify key sections exist
	sections := []string{
		"## 1. Your Role",
		"## 2. Your Tools",
		"## 3. Workers",
		"## 4. Task Workflow",
		"## 5. Writing Worker Prompts",
	}
	for _, section := range sections {
		if !strings.Contains(prompt, section) {
			t.Fatalf("missing section %q in prompt", section)
		}
	}

	// Verify worker tools are embedded
	if !strings.Contains(prompt, "Workers spawned via the Agent tool have access to these tools: Bash, Read") {
		t.Fatalf("missing worker tools in prompt")
	}
}

func TestGetCoordinatorSystemPrompt_WithMCPServers(t *testing.T) {
	prompt := GetCoordinatorSystemPrompt("Bash", "github, jira", "")
	if !strings.Contains(prompt, "MCP tools from connected MCP servers: github, jira") {
		t.Fatalf("missing MCP server list")
	}
}

func TestGetCoordinatorSystemPrompt_WithScratchpad(t *testing.T) {
	prompt := GetCoordinatorSystemPrompt("Bash", "", "/tmp/scratch")
	if !strings.Contains(prompt, "Scratchpad directory: /tmp/scratch") {
		t.Fatalf("missing scratchpad")
	}
}

func TestGetCoordinatorSystemPrompt_DefaultWorkerTools(t *testing.T) {
	prompt := GetCoordinatorSystemPrompt("", "", "")
	if !strings.Contains(prompt, "the standard tools available in this session") {
		t.Fatalf("missing default worker tools fallback")
	}
}

func TestRenderWorkerToolsSummary_FullMode(t *testing.T) {
	tools := map[string]struct{}{
		"Bash":          {},
		"Read":          {},
		"Agent":         {},
		"SendMessage":   {},
		"SyntheticOutput": {},
	}
	result := RenderWorkerToolsSummary(tools, false)
	if strings.Contains(result, "Agent") {
		t.Fatalf("Agent should be excluded from worker tools")
	}
	if strings.Contains(result, "SendMessage") {
		t.Fatalf("SendMessage should be excluded from worker tools")
	}
	if !strings.Contains(result, "Bash") {
		t.Fatalf("Bash should be in worker tools")
	}
	if !strings.Contains(result, "Read") {
		t.Fatalf("Read should be in worker tools")
	}
}

func TestRenderWorkerToolsSummary_SimpleMode(t *testing.T) {
	result := RenderWorkerToolsSummary(nil, true)
	if result != "Bash, Read, Edit" {
		t.Fatalf("simple mode = %q, want Bash, Read, Edit", result)
	}
}

func TestRenderWorkerToolsSummary_Empty(t *testing.T) {
	result := RenderWorkerToolsSummary(map[string]struct{}{}, false)
	if result != "" {
		t.Fatalf("empty tools = %q, want empty", result)
	}
}
