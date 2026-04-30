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

func TestSubstituteSimpleArgs(t *testing.T) {
	assert.Equal(t, "a b", substituteSimpleArgs("${1} ${2}", []string{"a", "b"}))
	assert.Equal(t, "hello", substituteSimpleArgs("${1}", []string{"hello"}))
	assert.Equal(t, "${1}", substituteSimpleArgs("${1}", []string{}))
	assert.Equal(t, "no placeholders", substituteSimpleArgs("no placeholders", []string{"a", "b"}))
}
