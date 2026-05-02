package promptsuggestion

import (
	"os"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
)

// PromptVariant identifies the prompt strategy used for suggestion generation.
type PromptVariant string

const (
	// PromptVariantUserIntent uses the inferred user intent as the prompt.
	PromptVariantUserIntent PromptVariant = "user_intent"
	// PromptVariantStatedIntent uses the explicitly stated user intent as the prompt.
	PromptVariantStatedIntent PromptVariant = "stated_intent"
)

// isEnvTruthy returns true if the environment variable is set to a truthy
// value (1, true, yes, on). Case-insensitive.
func isEnvTruthy(s string) bool {
	val := strings.ToLower(s)
	return val == "1" || val == "true" || val == "yes" || val == "on"
}

// isEnvFalsy returns true if the environment variable is set to a falsy
// value (0, false, no, off). Case-insensitive.
func isEnvFalsy(s string) bool {
	val := strings.ToLower(s)
	return val == "0" || val == "false" || val == "no" || val == "off"
}

// isNonInteractive returns true if the session is non-interactive.
// It checks the CLAUDE_NON_INTERACTIVE environment variable.
func isNonInteractive() bool {
	return os.Getenv("CLAUDE_NON_INTERACTIVE") != ""
}

// isTeammate returns true if the current agent is a teammate (forked subagent).
// It checks the CLAUDE_AGENT_ID environment variable.
func isTeammate() bool {
	return os.Getenv("CLAUDE_AGENT_ID") != ""
}

// IsPromptSuggestionEnabled implements a 4-layer gate chain:
//  1. env CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION override (truthy→true, falsy→false)
//  2. feature flag CLAUDE_FEATURE_PROMPT_SUGGESTION == "1"
//  3. non-interactive session rejection
//  4. teammate (forked subagent) rejection
// Defaults to false.
func IsPromptSuggestionEnabled() bool {
	if v := os.Getenv("CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION"); v != "" {
		if isEnvFalsy(v) {
			return false
		}
		if isEnvTruthy(v) {
			// Even with env override, apply safety gates
			if isNonInteractive() {
				return false
			}
			if isTeammate() {
				return false
			}
			return true
		}
	}

	if !featureflag.IsEnabled(featureflag.FlagPromptSuggestion) {
		return false
	}

	if isNonInteractive() {
		return false
	}

	if isTeammate() {
		return false
	}

	return true
}

// IsSpeculationEnabled implements a 2-layer gate:
//  1. env CLAUDE_CODE_ENABLE_SPECULATION override (truthy→true, falsy→false)
//  2. feature flag CLAUDE_FEATURE_SPECULATION == "1"
// Defaults to false.
func IsSpeculationEnabled() bool {
	if v := os.Getenv("CLAUDE_CODE_ENABLE_SPECULATION"); v != "" {
		if isEnvTruthy(v) {
			return true
		}
		if isEnvFalsy(v) {
			return false
		}
	}

	return featureflag.IsEnabled(featureflag.FlagSpeculation)
}

// GetPromptVariant returns the current prompt variant.
// Currently fixed to user_intent.
func GetPromptVariant() PromptVariant {
	return PromptVariantUserIntent
}
