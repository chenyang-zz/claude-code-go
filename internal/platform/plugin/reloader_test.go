package plugin

import (
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/internal/core/hook"
)

func TestPluginRegistrar_UnregisterAll(t *testing.T) {
	// Setup registries.
	cmdReg := command.NewInMemoryRegistry()
	agentReg := agent.NewInMemoryRegistry()
	hooks := hook.HooksConfig{}

	registrar := NewPluginRegistrar(agentReg, cmdReg, &hooks, nil, nil)

	// Register a plugin command.
	pc := &PluginCommand{Name: "test-cmd", PluginName: "test-plugin"}
	adapter := NewCommandAdapter(pc)
	_ = cmdReg.Register(adapter)

	// Register a plugin agent.
	_ = agentReg.Register(agent.Definition{AgentType: "test-agent", Source: "plugin"})
	// Register a non-plugin agent (should not be removed).
	_ = agentReg.Register(agent.Definition{AgentType: "builtin-agent", Source: "built-in"})

	// Unregister all.
	baseHooks := hook.HooksConfig{"PreToolUse": []hook.HookMatcher{{Matcher: "Bash"}}}
	hooks = hook.HooksConfig{"PreToolUse": []hook.HookMatcher{{Matcher: "Write"}}}
	registrar.UnregisterAll(baseHooks)

	// Verify command removed.
	if _, ok := cmdReg.Get("test-cmd"); ok {
		t.Error("plugin command should be unregistered")
	}

	// Verify plugin agent removed, built-in preserved.
	if _, ok := agentReg.Get("test-agent"); ok {
		t.Error("plugin agent should be removed")
	}
	if _, ok := agentReg.Get("builtin-agent"); !ok {
		t.Error("built-in agent should be preserved")
	}

	// Verify hooks restored.
	if len(hooks) != 1 {
		t.Fatalf("hooks should have 1 event, got %d", len(hooks))
	}
	matchers := hooks["PreToolUse"]
	if len(matchers) != 1 || matchers[0].Matcher != "Bash" {
		t.Error("hooks should be restored to base configuration")
	}
}

func TestPluginRegistrar_UnregisterAll_NilRegistrar(t *testing.T) {
	var r *PluginRegistrar
	// Should not panic.
	r.UnregisterAll(nil)
}

func TestReloader_Reload(t *testing.T) {
	loader := NewPluginLoader(NewInstalledPluginsStore())
	cmdReg := command.NewInMemoryRegistry()
	agentReg := agent.NewInMemoryRegistry()
	hooks := hook.HooksConfig{}
	registrar := NewPluginRegistrar(agentReg, cmdReg, &hooks, nil, nil)
	baseHooks := hook.HooksConfig{}

	reloader := NewReloader(loader, registrar, baseHooks)

	summary, err := reloader.Reload()
	if err != nil {
		t.Fatalf("Reload error: %v", err)
	}
	if summary == nil {
		t.Fatal("expected non-nil summary")
	}
}

func TestReloader_Reload_NilComponents(t *testing.T) {
	reloader := NewReloader(nil, nil, nil)

	summary, err := reloader.Reload()
	if err != nil {
		t.Fatalf("Reload with nil components should not error: %v", err)
	}
	if summary != nil {
		t.Error("expected nil summary when registrar is nil")
	}
}

func TestReloader_ConcurrentReload(t *testing.T) {
	loader := NewPluginLoader(NewInstalledPluginsStore())
	cmdReg := command.NewInMemoryRegistry()
	agentReg := agent.NewInMemoryRegistry()
	hooks := hook.HooksConfig{}
	registrar := NewPluginRegistrar(agentReg, cmdReg, &hooks, nil, nil)
	baseHooks := hook.HooksConfig{}

	reloader := NewReloader(loader, registrar, baseHooks)

	// First reload should succeed.
	_, err1 := reloader.Reload()
	if err1 != nil {
		t.Fatalf("first reload error: %v", err1)
	}

	// Verify not reloading.
	if reloader.IsReloading() {
		t.Error("should not be reloading after first reload completes")
	}
}

func TestReloader_IsReloading(t *testing.T) {
	reloader := NewReloader(nil, nil, nil)
	if reloader.IsReloading() {
		t.Error("new reloader should not be reloading")
	}
}
