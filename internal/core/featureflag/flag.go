package featureflag

import (
	"os"
	"strings"
)

// Well-known feature flag names. Each flag is controlled by the environment
// variable CLAUDE_FEATURE_{NAME} (e.g. CLAUDE_FEATURE_TOKEN_BUDGET).
const (
	// FlagTokenBudget gates the +500k token budget input parsing and
	// automatic budget continuation in the engine loop.
	FlagTokenBudget = "TOKEN_BUDGET"
	// FlagTodoV2 gates the TodoV2 task tools (TaskCreate, TaskGet, TaskList,
	// TaskUpdate, TaskClaim, TaskReset). When disabled, the task toolset is
	// hidden from the provider tool catalog.
	FlagTodoV2 = "TODO_V2"
)

// envPrefix is the environment variable prefix used for all feature flags.
const envPrefix = "CLAUDE_FEATURE_"

// IsEnabled reports whether the named feature flag is enabled.
// A flag is enabled when the environment variable CLAUDE_FEATURE_{NAME}
// is set to exactly "1". All other values (including unset) mean disabled.
func IsEnabled(name string) bool {
	return os.Getenv(envPrefix+name) == "1"
}

// IsTodoV2Enabled reports whether the TodoV2 task tools should be exposed.
// It checks the TS-compatible CLAUDE_CODE_ENABLE_TASKS variable (any truthy
// value) and the Go-native CLAUDE_FEATURE_TODO_V2 flag.
//
// When neither variable is set, the default is true because Go's
// claude-code-go is currently REPL/CLI-first (inherently interactive).
// When non-interactive SDK mode is supported, this default should become
// conditional on the session type.
func IsTodoV2Enabled() bool {
	if isEnvTruthy("CLAUDE_CODE_ENABLE_TASKS") {
		return true
	}
	if IsEnabled(FlagTodoV2) {
		return true
	}
	// Default to enabled for the current REPL/CLI-first runtime.
	return true
}

// isEnvTruthy returns true if the environment variable is set to a truthy
// value (1, true, yes). Case-insensitive.
func isEnvTruthy(key string) bool {
	val := strings.ToLower(os.Getenv(key))
	return val == "1" || val == "true" || val == "yes"
}
