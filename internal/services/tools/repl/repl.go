package repl

import (
	"os"
	"strings"
)

// Name is the stable registry identifier for the REPL tool.
const Name = "REPL"

// REPL_ONLY_TOOLS lists the tools that are only accessible when REPL mode
// is active. When REPL mode is on, these tools are hidden from the model
// for direct invocation, forcing the model to use the REPL for batch
// operations. In the Go CLI-first runtime, REPL mode is always on, so
// this constant documents the concept for downstream consumers.
//
// The values use the registered Go tool Name constants (e.g. "Read" not
// "ReadTool") to match the tool registry's stable identifiers.
var REPL_ONLY_TOOLS = map[string]struct{}{
	"Read":          {},
	"Write":         {},
	"Edit":          {},
	"Glob":          {},
	"Grep":          {},
	"Bash":          {},
	"NotebookEdit":  {},
	"Agent":         {},
}

// IsReplModeEnabled reports whether the current process is running in
// REPL mode. REPL mode is the default for CLI entrypoints where
// USER_TYPE is "ant". SDK entrypoints are NOT defaulted on because SDK
// consumers script direct tool calls and REPL mode hides those tools.
//
// Env var control:
//   - CLAUDE_CODE_REPL=0 / false / no: explicitly disables REPL mode
//   - CLAUDE_REPL_MODE=1: explicitly enables REPL mode (legacy env)
//   - Default: enabled when USER_TYPE=ant AND CLAUDE_CODE_ENTRYPOINT=cli
func IsReplModeEnabled() bool {
	if isEnvDefinedFalsy("CLAUDE_CODE_REPL") {
		return false
	}
	if isEnvTruthy("CLAUDE_REPL_MODE") {
		return true
	}
	return os.Getenv("USER_TYPE") == "ant" &&
		os.Getenv("CLAUDE_CODE_ENTRYPOINT") == "cli"
}

// isEnvDefinedFalsy returns true when the named env var is set to a
// falsy or disabled value: "0", "false", "no" (case-insensitive).
func isEnvDefinedFalsy(name string) bool {
	val := os.Getenv(name)
	if val == "" {
		return false
	}
	lower := strings.ToLower(val)
	return lower == "0" || lower == "false" || lower == "no"
}

// isEnvTruthy returns true when the named env var is set to a truthy
// value: "1", "true", "yes" (case-insensitive).
func isEnvTruthy(name string) bool {
	lower := strings.ToLower(os.Getenv(name))
	return lower == "1" || lower == "true" || lower == "yes"
}
