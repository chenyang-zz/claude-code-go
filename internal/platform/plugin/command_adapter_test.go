package plugin

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/stretchr/testify/assert"
)

func TestCommandAdapter_Metadata(t *testing.T) {
	pc := &PluginCommand{
		Name:          "test-cmd",
		Description:   "A test command",
		WhenToUse:     "when testing",
		UserInvocable: true,
		PluginName:    "test-plugin",
	}
	adapter := NewCommandAdapter(pc)

	meta := adapter.Metadata()
	assert.Equal(t, "test-cmd", meta.Name)
	assert.Equal(t, "A test command", meta.Description)
	assert.Equal(t, "/test-cmd [args]", meta.Usage)
	assert.False(t, meta.Hidden)
}

func TestCommandAdapter_Metadata_FallbackDescription(t *testing.T) {
	pc := &PluginCommand{
		Name:       "test-cmd",
		WhenToUse:  "when testing",
		PluginName: "test-plugin",
	}
	adapter := NewCommandAdapter(pc)

	meta := adapter.Metadata()
	assert.Equal(t, "when testing", meta.Description)
}

func TestCommandAdapter_Metadata_FallbackPluginName(t *testing.T) {
	pc := &PluginCommand{
		Name:       "test-cmd",
		PluginName: "test-plugin",
	}
	adapter := NewCommandAdapter(pc)

	meta := adapter.Metadata()
	assert.Equal(t, "Plugin command from test-plugin", meta.Description)
}

func TestCommandAdapter_Metadata_Hidden(t *testing.T) {
	pc := &PluginCommand{
		Name:          "test-cmd",
		UserInvocable: false,
	}
	adapter := NewCommandAdapter(pc)

	meta := adapter.Metadata()
	assert.True(t, meta.Hidden)
}

func TestCommandAdapter_Metadata_Nil(t *testing.T) {
	var adapter *CommandAdapter
	meta := adapter.Metadata()
	assert.Empty(t, meta.Name)
}

func TestCommandAdapter_Execute(t *testing.T) {
	pc := &PluginCommand{
		Name:       "test-cmd",
		RawContent: "Hello from plugin",
		PluginName: "test-plugin",
	}
	adapter := NewCommandAdapter(pc)

	result, err := adapter.Execute(context.Background(), command.Args{})
	assert.NoError(t, err)
	assert.Equal(t, "Hello from plugin", result.Output)
}

func TestCommandAdapter_Execute_ArgSubstitution(t *testing.T) {
	pc := &PluginCommand{
		Name:       "test-cmd",
		RawContent: "First: ${1}, Second: ${2}",
		PluginName: "test-plugin",
	}
	adapter := NewCommandAdapter(pc)

	result, err := adapter.Execute(context.Background(), command.Args{
		Raw: []string{"alpha", "beta"},
	})
	assert.NoError(t, err)
	assert.Equal(t, "First: alpha, Second: beta", result.Output)
}

func TestCommandAdapter_Execute_EmptyContentFallback(t *testing.T) {
	pc := &PluginCommand{
		Name:        "test-cmd",
		Description: "Test description",
		PluginName:  "test-plugin",
	}
	adapter := NewCommandAdapter(pc)

	result, err := adapter.Execute(context.Background(), command.Args{})
	assert.NoError(t, err)
	assert.Contains(t, result.Output, "/test-cmd")
	assert.Contains(t, result.Output, "test-plugin")
	assert.Contains(t, result.Output, "Test description")
}

func TestCommandAdapter_Execute_Nil(t *testing.T) {
	var adapter *CommandAdapter
	_, err := adapter.Execute(context.Background(), command.Args{})
	assert.Error(t, err)
}

func TestCommandAdapter_Metadata_ArgumentHint(t *testing.T) {
	pc := &PluginCommand{
		Name:          "test-cmd",
		Description:   "A test command",
		ArgumentHint:  "[file] [line]",
		UserInvocable: true,
	}
	adapter := NewCommandAdapter(pc)

	meta := adapter.Metadata()
	assert.Equal(t, "/test-cmd [file] [line]", meta.Usage)
}

func TestCommandAdapter_Execute_PluginVariableSubstitution(t *testing.T) {
	pc := &PluginCommand{
		Name:         "test-cmd",
		RawContent:   "Root: ${CLAUDE_PLUGIN_ROOT}",
		PluginPath:   "/path/to/plugin",
		PluginName:   "test-plugin",
		PluginSource: "test-plugin",
	}
	adapter := NewCommandAdapter(pc)

	result, err := adapter.Execute(context.Background(), command.Args{})
	assert.NoError(t, err)
	assert.Equal(t, "Root: /path/to/plugin", result.Output)
}

func TestCommandAdapter_Execute_SkillDirSubstitution(t *testing.T) {
	pc := &PluginCommand{
		Name:         "test-skill",
		RawContent:   "Skill dir: ${CLAUDE_SKILL_DIR}",
		PluginPath:   "/path/to/plugin",
		SourcePath:   "/path/to/plugin/skills/build/SKILL.md",
		IsSkill:      true,
		PluginName:   "test-plugin",
		PluginSource: "test-plugin",
	}
	adapter := NewCommandAdapter(pc)

	result, err := adapter.Execute(context.Background(), command.Args{})
	assert.NoError(t, err)
	assert.Equal(t, "Skill dir: /path/to/plugin/skills/build", result.Output)
}

func TestCommandAdapter_Execute_ArgumentsSubstitution(t *testing.T) {
	pc := &PluginCommand{
		Name:         "test-cmd",
		RawContent:   "All: $ARGUMENTS, First: $1",
		PluginName:   "test-plugin",
		PluginSource: "test-plugin",
	}
	adapter := NewCommandAdapter(pc)

	result, err := adapter.Execute(context.Background(), command.Args{
		Raw: []string{"alpha", "beta"},
	})
	assert.NoError(t, err)
	assert.Equal(t, "All: alpha beta, First: alpha", result.Output)
}

func TestCommandAdapter_Execute_NamedArgumentSubstitution(t *testing.T) {
	pc := &PluginCommand{
		Name:          "test-cmd",
		RawContent:    "Hello $name",
		PluginName:    "test-plugin",
		PluginSource:  "test-plugin",
		ArgumentNames: []string{"name"},
	}
	adapter := NewCommandAdapter(pc)

	result, err := adapter.Execute(context.Background(), command.Args{
		Raw: []string{"Alice"},
	})
	assert.NoError(t, err)
	assert.Equal(t, "Hello Alice", result.Output)
}

func TestCommandAdapter_Execute_AppendIfNoPlaceholder(t *testing.T) {
	pc := &PluginCommand{
		Name:         "test-cmd",
		RawContent:   "Plain text",
		PluginName:   "test-plugin",
		PluginSource: "test-plugin",
	}
	adapter := NewCommandAdapter(pc)

	result, err := adapter.Execute(context.Background(), command.Args{
		Raw: []string{"extra"},
	})
	assert.NoError(t, err)
	// With appendIfNoPlaceholder=false in Execute, arguments are not appended
	// when no placeholder is present (substituteSimpleArgs handles ${n} patterns).
	assert.Equal(t, "Plain text", result.Output)
}

func TestCommandAdapter_ParsedAllowedTools(t *testing.T) {
	pc := &PluginCommand{
		Name:         "test-cmd",
		AllowedTools: "read, write, edit",
		PluginName:   "test-plugin",
	}
	adapter := NewCommandAdapter(pc)

	tools := adapter.ParsedAllowedTools()
	assert.Equal(t, []string{"read", "write", "edit"}, tools)
}

func TestCommandAdapter_ParsedAllowedTools_Empty(t *testing.T) {
	pc := &PluginCommand{
		Name:       "test-cmd",
		PluginName: "test-plugin",
	}
	adapter := NewCommandAdapter(pc)

	tools := adapter.ParsedAllowedTools()
	assert.Nil(t, tools)
}

func TestSubstituteSimpleArgs(t *testing.T) {
	assert.Equal(t, "a b", substituteSimpleArgs("${1} ${2}", []string{"a", "b"}))
	assert.Equal(t, "hello", substituteSimpleArgs("${1}", []string{"hello"}))
	assert.Equal(t, "${1}", substituteSimpleArgs("${1}", []string{}))
	assert.Equal(t, "no placeholders", substituteSimpleArgs("no placeholders", []string{"a", "b"}))
}
