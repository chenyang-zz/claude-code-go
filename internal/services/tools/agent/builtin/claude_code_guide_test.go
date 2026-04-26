package builtin

import (
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/tool"
)

func TestClaudeCodeGuideSystemPromptProvider_EmptySnapshot_ReturnsBasePrompt(t *testing.T) {
	provider := ClaudeCodeGuideSystemPromptProvider{}
	prompt := provider.GetSystemPrompt(tool.UseContext{})

	if prompt == "" {
		t.Fatal("expected non-empty base prompt")
	}
	if strings.Contains(prompt, "User's Current Configuration") {
		t.Error("empty snapshot should not include configuration section")
	}
	if !strings.Contains(prompt, "Claude guide agent") {
		t.Error("base prompt missing expected content")
	}
}

func TestClaudeCodeGuideSystemPromptProvider_WithSnapshot_AppendsConfiguration(t *testing.T) {
	provider := ClaudeCodeGuideSystemPromptProvider{}
	ctx := tool.UseContext{
		SessionConfig: tool.SessionConfigSnapshot{
			CustomAgents: []tool.AgentInfo{
				{AgentType: "my-agent", WhenToUse: "use me"},
			},
			MCPServers: []string{"server-a", "server-b"},
			UserSettings: map[string]any{
				"theme": "dark",
			},
		},
	}
	prompt := provider.GetSystemPrompt(ctx)

	if !strings.Contains(prompt, "User's Current Configuration") {
		t.Error("expected configuration section")
	}
	if !strings.Contains(prompt, "my-agent") {
		t.Error("expected custom agent in configuration")
	}
	if !strings.Contains(prompt, "server-a") {
		t.Error("expected MCP server in configuration")
	}
	if !strings.Contains(prompt, "theme") {
		t.Error("expected user settings in configuration")
	}
	if !strings.Contains(prompt, "Claude guide agent") {
		t.Error("base prompt should be preserved before configuration")
	}
}

func TestRenderCurrentConfiguration_CustomAgents(t *testing.T) {
	cfg := tool.SessionConfigSnapshot{
		CustomAgents: []tool.AgentInfo{
			{AgentType: "test-agent", WhenToUse: "when testing"},
		},
	}
	out := renderCurrentConfiguration(cfg)
	if !strings.Contains(out, "Custom agents") {
		t.Error("missing Custom agents header")
	}
	if !strings.Contains(out, "test-agent") {
		t.Error("missing agent type")
	}
	if !strings.Contains(out, "when testing") {
		t.Error("missing whenToUse")
	}
}

func TestRenderCurrentConfiguration_MCPServers(t *testing.T) {
	cfg := tool.SessionConfigSnapshot{
		MCPServers: []string{"fs", "git"},
	}
	out := renderCurrentConfiguration(cfg)
	if !strings.Contains(out, "MCP servers") {
		t.Error("missing MCP servers header")
	}
	if !strings.Contains(out, "fs") || !strings.Contains(out, "git") {
		t.Error("missing server names")
	}
}

func TestRenderCurrentConfiguration_CustomSkills(t *testing.T) {
	cfg := tool.SessionConfigSnapshot{
		CustomSkills: []tool.SkillInfo{
			{Name: "test-skill", Description: "a test skill"},
		},
	}
	out := renderCurrentConfiguration(cfg)
	if !strings.Contains(out, "Custom skills") {
		t.Error("missing Custom skills header")
	}
	if !strings.Contains(out, "/test-skill") {
		t.Error("missing skill name with slash prefix")
	}
}

func TestRenderCurrentConfiguration_PluginSkills(t *testing.T) {
	cfg := tool.SessionConfigSnapshot{
		PluginSkills: []tool.SkillInfo{
			{Name: "plugin-skill", Description: "a plugin skill"},
		},
	}
	out := renderCurrentConfiguration(cfg)
	if !strings.Contains(out, "Plugin skills") {
		t.Error("missing Plugin skills header")
	}
}

func TestRenderCurrentConfiguration_UserSettings(t *testing.T) {
	cfg := tool.SessionConfigSnapshot{
		UserSettings: map[string]any{
			"model":   "claude-sonnet",
			"theme":   "dark",
		},
	}
	out := renderCurrentConfiguration(cfg)
	if !strings.Contains(out, "User settings.json") {
		t.Error("missing User settings.json header")
	}
	if !strings.Contains(out, "claude-sonnet") {
		t.Error("missing settings value")
	}
}

func TestRenderCurrentConfiguration_EmptyOmitsSections(t *testing.T) {
	cfg := tool.SessionConfigSnapshot{}
	out := renderCurrentConfiguration(cfg)
	if strings.Contains(out, "Custom skills") {
		t.Error("should omit empty Custom skills section")
	}
	if strings.Contains(out, "Custom agents") {
		t.Error("should omit empty Custom agents section")
	}
	if strings.Contains(out, "MCP servers") {
		t.Error("should omit empty MCP servers section")
	}
	if strings.Contains(out, "Plugin skills") {
		t.Error("should omit empty Plugin skills section")
	}
	if strings.Contains(out, "User settings.json") {
		t.Error("should omit empty User settings section")
	}
}
