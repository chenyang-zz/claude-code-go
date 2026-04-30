package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/hook"
	"github.com/sheepzhao/claude-code-go/internal/platform/plugin"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// DefaultHookTimeout is the default timeout for hook command execution (10 minutes).
	DefaultHookTimeout = 10 * time.Minute
)

// Runner executes command-type hooks and returns their results.
type Runner struct {
	// Environ returns the base environment inherited by child processes.
	Environ func() []string
}

// NewRunner creates a hook runner using the host environment.
func NewRunner() *Runner {
	return &Runner{
		Environ: os.Environ,
	}
}

// RunCommand executes one command-type hook, piping the hook input as JSON via stdin.
// Plugin variables in the command string are substituted, and plugin-specific
// environment variables (CLAUDE_PLUGIN_ROOT, CLAUDE_PLUGIN_DATA,
// CLAUDE_PROJECT_DIR) are injected into the child process.
func (r *Runner) RunCommand(ctx context.Context, cmdHook hook.CommandHook, input any, cwd string) (hook.HookResult, error) {
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return hook.HookResult{}, fmt.Errorf("marshal hook input: %w", err)
	}

	timeout := DefaultHookTimeout
	if cmdHook.Timeout > 0 {
		timeout = time.Duration(cmdHook.Timeout) * time.Second
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Extract plugin context from the If field.
	pluginName, pluginRoot, pluginSource := extractPluginInfo(cmdHook.If)

	// Substitute plugin variables in the command string.
	command := cmdHook.Command
	if pluginRoot != "" {
		command = strings.ReplaceAll(command, "${CLAUDE_PLUGIN_ROOT}", filepath.ToSlash(pluginRoot))
	}
	if pluginSource != "" {
		if dataDir, err := plugin.GetPluginDataDir(pluginSource); err == nil {
			command = strings.ReplaceAll(command, "${CLAUDE_PLUGIN_DATA}", filepath.ToSlash(dataDir))
		}
	}

	shellPath, shellArgs := resolveShell(cmdHook.Shell)
	args := append(shellArgs, command)

	cmd := exec.CommandContext(runCtx, shellPath, args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	cmd.Env = r.injectPluginEnv(r.environ(), pluginRoot, pluginSource, cwd)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = bytes.NewReader(append(inputJSON, '\n'))

	logger.DebugCF("hook_runner", "executing hook command", map[string]any{
		"command":     command,
		"original":    cmdHook.Command,
		"plugin":      pluginName,
		"shell":       shellPath,
		"timeout_sec": int(timeout.Seconds()),
		"cwd":         cwd,
	})

	err = cmd.Run()
	result := hook.HookResult{
		ExitCode: 0,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}

	if err == nil {
		// Parse stdout JSON for structured output.
		parsed := hook.ParseHookOutput(result.Stdout)
		result.ParsedOutput = parsed
		if parsed != nil && parsed.Continue != nil && !*parsed.Continue {
			result.PreventContinuation = true
		}
		logger.DebugCF("hook_runner", "hook command succeeded", map[string]any{
			"command":    command,
			"stdout_len": len(result.Stdout),
		})
		return result, nil
	}

	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		result.TimedOut = true
		result.ExitCode = -1
		logger.DebugCF("hook_runner", "hook command timed out", map[string]any{
			"command":     command,
			"timeout_sec": int(timeout.Seconds()),
		})
		return result, nil
	}

	if errors.Is(runCtx.Err(), context.Canceled) {
		result.ExitCode = -1
		logger.DebugCF("hook_runner", "hook command canceled", map[string]any{
			"command": command,
		})
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		logger.DebugCF("hook_runner", "hook command exited with failure", map[string]any{
			"command":   command,
			"exit_code": result.ExitCode,
			"stderr":    truncateForLog(result.Stderr, 200),
		})
		return result, nil
	}

	return hook.HookResult{}, fmt.Errorf("run hook command %q: %w", command, err)
}

// RunStopHooks executes all command hooks for a stop-type event.
// This method satisfies the engine.HookRunner interface.
func (r *Runner) RunStopHooks(ctx context.Context, config hook.HooksConfig, event hook.HookEvent, input any, cwd string) []hook.HookResult {
	return r.RunHooksForEventWithQuery(ctx, config, event, input, cwd, MatchQuery{})
}

// RunHooksForTool executes command hooks filtered by tool name for tool lifecycle events.
// This method satisfies the engine.HookRunner interface.
func (r *Runner) RunHooksForTool(ctx context.Context, config hook.HooksConfig, event hook.HookEvent, input any, cwd string, toolName string) []hook.HookResult {
	cmdHooks := MatchHooks(config, event, MatchQuery{ToolName: toolName})
	if len(cmdHooks) == 0 {
		return nil
	}

	results := make([]hook.HookResult, 0, len(cmdHooks))
	for _, cmdHook := range cmdHooks {
		result, err := r.RunCommand(ctx, cmdHook, input, cwd)
		if err != nil {
			logger.DebugCF("hook_runner", "hook execution failed", map[string]any{
				"event":     string(event),
				"tool_name": toolName,
				"command":   cmdHook.Command,
				"error":     err.Error(),
			})
			results = append(results, hook.HookResult{ExitCode: -1, Stderr: err.Error()})
			continue
		}
		results = append(results, result)
	}
	return results
}

// RunHooksForEvent executes all matching command hooks for an event and returns results.
func (r *Runner) RunHooksForEvent(ctx context.Context, config hook.HooksConfig, event hook.HookEvent, input any, cwd string) []hook.HookResult {
	return r.RunHooksForEventWithQuery(ctx, config, event, input, cwd, MatchQuery{})
}

// RunHooksForEventWithQuery executes hooks for an event using an explicit matcher query.
func (r *Runner) RunHooksForEventWithQuery(ctx context.Context, config hook.HooksConfig, event hook.HookEvent, input any, cwd string, query MatchQuery) []hook.HookResult {
	cmdHooks := config.CommandHooks(event)
	if len(cmdHooks) == 0 {
		return nil
	}

	if query.ToolName != "" || query.Matcher != "" {
		cmdHooks = MatchHooks(config, event, query)
	}

	results := make([]hook.HookResult, 0, len(cmdHooks))
	for _, cmdHook := range cmdHooks {
		result, err := r.RunCommand(ctx, cmdHook, input, cwd)
		if err != nil {
			logger.DebugCF("hook_runner", "hook execution failed", map[string]any{
				"event":   string(event),
				"command": cmdHook.Command,
				"error":   err.Error(),
			})
			results = append(results, hook.HookResult{ExitCode: -1, Stderr: err.Error()})
			continue
		}
		results = append(results, result)
	}
	return results
}

// environ returns the base child-process environment.
func (r *Runner) environ() []string {
	if r != nil && r.Environ != nil {
		return append([]string{}, r.Environ()...)
	}
	return append([]string{}, os.Environ()...)
}

// resolveShell selects the shell executable and argument prefix based on the hook's shell setting.
func resolveShell(shellType string) (string, []string) {
	if shellType == "powershell" {
		return "pwsh", []string{"-NoProfile", "-NonInteractive", "-Command"}
	}
	if runtime.GOOS == "windows" {
		return "bash", []string{"-c"}
	}
	return "bash", []string{"-c"}
}

// truncateForLog limits a string for safe inclusion in log output.
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// HasBlockingResult reports whether any hook result indicates a blocking error (exit code 2).
func HasBlockingResult(results []hook.HookResult) bool {
	for _, r := range results {
		if r.IsBlocking() {
			return true
		}
	}
	return false
}

// HasErrorResult reports whether any hook result indicates a non-blocking error.
func HasErrorResult(results []hook.HookResult) bool {
	for _, r := range results {
		if r.IsError() {
			return true
		}
	}
	return false
}

// BlockingErrors collects stderr from all blocking results (exit code 2).
func BlockingErrors(results []hook.HookResult) []string {
	var errs []string
	for _, r := range results {
		if r.IsBlocking() && strings.TrimSpace(r.Stderr) != "" {
			errs = append(errs, r.Stderr)
		}
	}
	return errs
}

// ErrorMessages collects stderr from all error results (non-zero, non-2).
func ErrorMessages(results []hook.HookResult) []string {
	var msgs []string
	for _, r := range results {
		if r.IsError() && strings.TrimSpace(r.Stderr) != "" {
			msgs = append(msgs, r.Stderr)
		}
	}
	return msgs
}

// extractPluginInfo parses the plugin context encoded in the If field.
// The expected format is "plugin:{name}:{root}:{source}".
// If the field does not start with "plugin:", empty strings are returned
// and the If value is treated as a normal condition.
func extractPluginInfo(ifField string) (name, root, source string) {
	if !strings.HasPrefix(ifField, "plugin:") {
		return "", "", ""
	}
	parts := strings.SplitN(ifField, ":", 4)
	if len(parts) >= 2 {
		name = parts[1]
	}
	if len(parts) >= 3 {
		root = parts[2]
	}
	if len(parts) >= 4 {
		source = parts[3]
	}
	return name, root, source
}

// injectPluginEnv injects plugin-specific environment variables into the base
// environment. Variables are only added when their corresponding values are
// non-empty and not already present in the base environment.
func (r *Runner) injectPluginEnv(base []string, pluginRoot, pluginSource, projectDir string) []string {
	env := make(map[string]string)
	for _, e := range base {
		if k, v, ok := strings.Cut(e, "="); ok {
			env[k] = v
		}
	}

	if projectDir != "" {
		if _, ok := env["CLAUDE_PROJECT_DIR"]; !ok {
			env["CLAUDE_PROJECT_DIR"] = projectDir
		}
	}
	if pluginRoot != "" {
		if _, ok := env["CLAUDE_PLUGIN_ROOT"]; !ok {
			env["CLAUDE_PLUGIN_ROOT"] = filepath.ToSlash(pluginRoot)
		}
	}
	if pluginSource != "" {
		if dataDir, err := plugin.GetPluginDataDir(pluginSource); err == nil {
			if _, ok := env["CLAUDE_PLUGIN_DATA"]; !ok {
				env["CLAUDE_PLUGIN_DATA"] = filepath.ToSlash(dataDir)
			}
		}
	}

	result := make([]string, 0, len(env))
	for k, v := range env {
		result = append(result, k+"="+v)
	}
	return result
}
