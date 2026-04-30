package plugin

import (
	"encoding/json"
	"fmt"

	"github.com/sheepzhao/claude-code-go/internal/core/hook"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// HooksRegistrar converts plugin hook configurations into the core
// hook.HooksConfig format used by the engine runtime.
type HooksRegistrar struct{}

// NewHooksRegistrar creates a new hooks registrar.
func NewHooksRegistrar() *HooksRegistrar {
	return &HooksRegistrar{}
}

// RegisterHooks converts a slice of enabled plugins' hook configurations into a
// single hook.HooksConfig and merges it with the provided base configuration.
// The returned config contains both base hooks and plugin hooks; plugin hooks
// for the same event are appended after base hooks.
func (r *HooksRegistrar) RegisterHooks(plugins []*LoadedPlugin, base hook.HooksConfig) (hook.HooksConfig, []*PluginError) {
	if r == nil {
		return base, nil
	}

	pluginHooks := make(hook.HooksConfig)

	for _, p := range plugins {
		if p == nil || p.HooksConfig == nil || len(p.HooksConfig.Events) == 0 {
			continue
		}

		for eventName, matchers := range p.HooksConfig.Events {
			event := hook.HookEvent(eventName)
			if !event.IsValid() {
				logger.DebugCF("plugin.hooks_registrar", "skipping unknown hook event", map[string]any{
					"event":  eventName,
					"plugin": p.Name,
				})
				continue
			}

			for _, m := range matchers {
				hm, err := convertHookMatcher(m, p.Path, p.Name)
				if err != nil {
					logger.WarnCF("plugin.hooks_registrar", "failed to convert hook matcher", map[string]any{
						"event":  eventName,
						"plugin": p.Name,
						"error":  err.Error(),
					})
					continue
				}
				pluginHooks[event] = append(pluginHooks[event], hm)
			}
		}
	}

	// Merge plugin hooks after base hooks.  Plugin hooks for the same
	// event are appended after base hooks so both execute.
	merged := make(hook.HooksConfig, len(base))
	for k, v := range base {
		merged[k] = v
	}
	for event, matchers := range pluginHooks {
		merged[event] = append(merged[event], matchers...)
	}

	logger.DebugCF("plugin.hooks_registrar", "registered plugin hooks", map[string]any{
		"event_count": len(merged),
	})

	return merged, nil
}

// convertHookMatcher converts a plugin HookMatcherEntry and its plugin context
// into a core hook.HookMatcher.  The plugin root path is preserved in the
// CommandHook's execution context via the If field (used by the hook runner
// for CWD resolution).
func convertHookMatcher(entry HookMatcherEntry, pluginRoot, pluginName string) (hook.HookMatcher, error) {
	var rawHooks []json.RawMessage

	for _, hc := range entry.Hooks {
		cmdHook := hook.CommandHook{
			Type:    hook.TypeCommand,
			Command: hc.Command,
		}
		if hc.Timeout > 0 {
			cmdHook.Timeout = hc.Timeout / 1000 // Plugin stores ms, core uses seconds
		}
		if pluginRoot != "" {
			// Store plugin root in the If field so the hook runner can derive
			// the correct working directory.  This is a pragmatic use of an
			// otherwise unused field for plugin hook CWD tracking.
			cmdHook.If = fmt.Sprintf("plugin:%s:%s", pluginName, pluginRoot)
		}

		raw, err := json.Marshal(cmdHook)
		if err != nil {
			return hook.HookMatcher{}, fmt.Errorf("marshal hook command: %w", err)
		}
		rawHooks = append(rawHooks, raw)
	}

	return hook.HookMatcher{
		Matcher: entry.Matcher,
		Hooks:   rawHooks,
	}, nil
}
