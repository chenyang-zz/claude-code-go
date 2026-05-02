package tips

import (
	"os"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
)

// isEnvTruthy reports whether a string is a truthy value.
func isEnvTruthy(s string) bool {
	v := strings.ToLower(s)
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

// isEnvFalsy reports whether a string is a falsy value.
func isEnvFalsy(s string) bool {
	v := strings.ToLower(s)
	return v == "0" || v == "false" || v == "no" || v == "off"
}

// IsSpinnerTipsEnabled implements the gate chain for tip display.
//   1. env CLAUDE_CODE_ENABLE_SPINNER_TIPS override (truthy→true, falsy→false)
//   2. feature flag CLAUDE_FEATURE_SPINNER_TIPS == "1"
//   3. Defaults to true for interactive REPL/CLI sessions.
func IsSpinnerTipsEnabled() bool {
	if v := os.Getenv("CLAUDE_CODE_ENABLE_SPINNER_TIPS"); v != "" {
		if isEnvFalsy(v) {
			return false
		}
		if isEnvTruthy(v) {
			return true
		}
	}

	if featureflag.IsEnabled(featureflag.FlagSpinnerTips) {
		return true
	}

	// Default to enabled for the current REPL/CLI-first runtime.
	return true
}
