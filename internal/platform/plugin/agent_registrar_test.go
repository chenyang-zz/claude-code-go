package plugin

import (
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentRegistrar_RegisterAgents_Success(t *testing.T) {
	registry := agent.NewInMemoryRegistry()
	registrar := NewAgentRegistrar(registry)

	defs := []*AgentDefinition{
		{
			AgentType:   "test-agent",
			WhenToUse:   "when testing",
			Description: "A test agent",
			Tools:       "read,write",
			Skills:      "test-skill",
			Model:       "claude-sonnet-4-6",
			Effort:      "high",
			MaxTurns:    10,
			Background:  true,
			Memory:      "project",
			Isolation:   "worktree",
			PluginName:  "test-plugin",
			SourcePath:  "/plugins/test/test-agent.yaml",
			RawContent:  "You are a test agent.",
			Color:       "#ff0000",
		},
	}

	registered, errs := registrar.RegisterAgents(defs)
	require.Len(t, errs, 0)
	assert.Equal(t, 1, registered)

	def, ok := registry.Get("test-agent")
	require.True(t, ok)
	assert.Equal(t, "test-agent", def.AgentType)
	assert.Equal(t, "when testing", def.WhenToUse)
	assert.Equal(t, "plugin", def.Source)
	assert.Equal(t, []string{"read", "write"}, def.Tools)
	assert.Equal(t, []string{"test-skill"}, def.Skills)
	assert.Equal(t, "claude-sonnet-4-6", def.Model)
	assert.Equal(t, "high", def.Effort)
	assert.Equal(t, 10, def.MaxTurns)
	assert.True(t, def.Background)
	assert.Equal(t, "project", def.Memory)
	assert.Equal(t, "worktree", def.Isolation)
	assert.Equal(t, "test-plugin", def.Plugin)
	assert.Equal(t, "You are a test agent.", def.SystemPrompt)
	assert.Equal(t, "#ff0000", def.Color)
	assert.Equal(t, "test-agent", def.Filename)
	assert.Equal(t, "/plugins/test", def.BaseDir)
}

func TestAgentRegistrar_RegisterAgents_Override(t *testing.T) {
	registry := agent.NewInMemoryRegistry()
	registrar := NewAgentRegistrar(registry)

	// Register first agent.
	first := &AgentDefinition{AgentType: "agent-1", WhenToUse: "first", PluginName: "p1"}
	registered, errs := registrar.RegisterAgents([]*AgentDefinition{first})
	require.Len(t, errs, 0)
	assert.Equal(t, 1, registered)

	def, ok := registry.Get("agent-1")
	require.True(t, ok)
	assert.Equal(t, "first", def.WhenToUse)

	// Register second agent with same type — should override.
	second := &AgentDefinition{AgentType: "agent-1", WhenToUse: "second", PluginName: "p2"}
	registered, errs = registrar.RegisterAgents([]*AgentDefinition{second})
	require.Len(t, errs, 0)
	assert.Equal(t, 1, registered)

	def, ok = registry.Get("agent-1")
	require.True(t, ok)
	assert.Equal(t, "second", def.WhenToUse)
}

func TestAgentRegistrar_RegisterAgents_Nil(t *testing.T) {
	var registrar *AgentRegistrar
	registered, errs := registrar.RegisterAgents([]*AgentDefinition{{AgentType: "a"}})
	assert.Equal(t, 0, registered)
	require.Len(t, errs, 1)
	assert.Equal(t, "registration-error", errs[0].Type)
}

func TestAgentRegistrar_RegisterAgents_NilRegistry(t *testing.T) {
	registrar := NewAgentRegistrar(nil)
	registered, errs := registrar.RegisterAgents([]*AgentDefinition{{AgentType: "a"}})
	assert.Equal(t, 0, registered)
	require.Len(t, errs, 1)
	assert.Equal(t, "registration-error", errs[0].Type)
}

func TestAgentRegistrar_RegisterAgents_SkipsNilDefs(t *testing.T) {
	registry := agent.NewInMemoryRegistry()
	registrar := NewAgentRegistrar(registry)

	defs := []*AgentDefinition{
		{AgentType: "valid", PluginName: "p"},
		nil,
	}

	registered, errs := registrar.RegisterAgents(defs)
	require.Len(t, errs, 0)
	assert.Equal(t, 1, registered)
}

func TestToCoreAgentDefinition(t *testing.T) {
	def := &AgentDefinition{
		AgentType:    "explore",
		WhenToUse:    "",
		Description:  "fallback description",
		Tools:        "*",
		DisallowedTools: "",
		Skills:       "",
		SourcePath:   "/path/to/agent.yaml",
		PluginName:   "my-plugin",
	}

	core := toCoreAgentDefinition(def)
	assert.Equal(t, "explore", core.AgentType)
	assert.Equal(t, "fallback description", core.WhenToUse)
	assert.Nil(t, core.Tools)
	assert.Nil(t, core.DisallowedTools)
	assert.Nil(t, core.Skills)
	assert.Equal(t, "plugin", core.Source)
	assert.Equal(t, "my-plugin", core.Plugin)
	assert.Equal(t, "agent", core.Filename)
	assert.Equal(t, "/path/to", core.BaseDir)
}

func TestParseCommaSeparated(t *testing.T) {
	assert.Nil(t, parseCommaSeparated(""))
	assert.Nil(t, parseCommaSeparated("*"))
	assert.Nil(t, parseCommaSeparated("  "))
	assert.Equal(t, []string{"a", "b", "c"}, parseCommaSeparated("a, b, c"))
	assert.Equal(t, []string{"a"}, parseCommaSeparated(" a "))
}
