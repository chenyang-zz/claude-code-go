package plugin

import (
	"encoding/json"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/hook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHooksRegistrar_RegisterHooks_Success(t *testing.T) {
	registrar := NewHooksRegistrar()
	plugins := []*LoadedPlugin{
		{
			Name: "test-plugin",
			Path: "/plugins/test",
			HooksConfig: &HooksConfig{
				Events: map[string][]HookMatcherEntry{
					"PreToolUse": {
						{
							Matcher: "test-tool",
							Hooks: []HookCommand{
								{Command: "echo pre", Timeout: 5000},
							},
						},
					},
				},
			},
		},
	}

	merged, errs := registrar.RegisterHooks(plugins, nil)
	require.Len(t, errs, 0)
	require.Len(t, merged[hook.EventPreToolUse], 1)
	assert.Equal(t, "test-tool", merged[hook.EventPreToolUse][0].Matcher)
}

func TestHooksRegistrar_RegisterHooks_MergeWithBase(t *testing.T) {
	registrar := NewHooksRegistrar()

	base := hook.HooksConfig{
		hook.EventPreToolUse: []hook.HookMatcher{
			{Matcher: "base-tool"},
		},
	}

	plugins := []*LoadedPlugin{
		{
			Name: "test-plugin",
			Path: "/plugins/test",
			HooksConfig: &HooksConfig{
				Events: map[string][]HookMatcherEntry{
					"PreToolUse": {
						{
							Matcher: "plugin-tool",
							Hooks:   []HookCommand{{Command: "echo"}},
						},
					},
				},
			},
		},
	}

	merged, errs := registrar.RegisterHooks(plugins, base)
	require.Len(t, errs, 0)
	require.Len(t, merged[hook.EventPreToolUse], 2)
	assert.Equal(t, "base-tool", merged[hook.EventPreToolUse][0].Matcher)
	assert.Equal(t, "plugin-tool", merged[hook.EventPreToolUse][1].Matcher)
}

func TestHooksRegistrar_RegisterHooks_InvalidEvent(t *testing.T) {
	registrar := NewHooksRegistrar()
	plugins := []*LoadedPlugin{
		{
			Name: "test-plugin",
			Path: "/plugins/test",
			HooksConfig: &HooksConfig{
				Events: map[string][]HookMatcherEntry{
					"UnknownEvent": {
						{
							Matcher: "test",
							Hooks:   []HookCommand{{Command: "echo"}},
						},
					},
				},
			},
		},
	}

	merged, errs := registrar.RegisterHooks(plugins, nil)
	require.Len(t, errs, 0)
	assert.Len(t, merged, 0)
}

func TestHooksRegistrar_RegisterHooks_NilRegistrar(t *testing.T) {
	var registrar *HooksRegistrar
	base := hook.HooksConfig{hook.EventPreToolUse: []hook.HookMatcher{{Matcher: "base"}}}
	merged, errs := registrar.RegisterHooks(nil, base)
	require.Len(t, errs, 0)
	assert.Equal(t, base, merged)
}

func TestHooksRegistrar_RegisterHooks_EmptyPlugins(t *testing.T) {
	registrar := NewHooksRegistrar()
	base := hook.HooksConfig{hook.EventPreToolUse: []hook.HookMatcher{{Matcher: "base"}}}
	merged, errs := registrar.RegisterHooks([]*LoadedPlugin{}, base)
	require.Len(t, errs, 0)
	assert.Equal(t, base, merged)
}

func TestHooksRegistrar_RegisterHooks_NilPlugin(t *testing.T) {
	registrar := NewHooksRegistrar()
	plugins := []*LoadedPlugin{nil}
	merged, errs := registrar.RegisterHooks(plugins, nil)
	require.Len(t, errs, 0)
	assert.Len(t, merged, 0)
}

func TestConvertHookMatcher(t *testing.T) {
	entry := HookMatcherEntry{
		Matcher: "my-tool",
		Hooks: []HookCommand{
			{Command: "echo hello", Timeout: 3000},
		},
	}

	hm, err := convertHookMatcher(entry, "/plugins/test", "my-plugin")
	require.NoError(t, err)
	assert.Equal(t, "my-tool", hm.Matcher)
	require.Len(t, hm.Hooks, 1)

	// Verify the If field stores plugin context.
	var cmdHook hook.CommandHook
	err = json.Unmarshal(hm.Hooks[0], &cmdHook)
	require.NoError(t, err)
	assert.Equal(t, hook.TypeCommand, cmdHook.Type)
	assert.Equal(t, "echo hello", cmdHook.Command)
	assert.Equal(t, 3, cmdHook.Timeout) // 3000ms -> 3s
	assert.Equal(t, "plugin:my-plugin:/plugins/test", cmdHook.If)
}

func TestConvertHookMatcher_NoPluginRoot(t *testing.T) {
	entry := HookMatcherEntry{
		Matcher: "tool",
		Hooks:   []HookCommand{{Command: "echo"}},
	}

	hm, err := convertHookMatcher(entry, "", "plugin")
	require.NoError(t, err)

	var cmdHook hook.CommandHook
	err = json.Unmarshal(hm.Hooks[0], &cmdHook)
	require.NoError(t, err)
	assert.Empty(t, cmdHook.If)
}
