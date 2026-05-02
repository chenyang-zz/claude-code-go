package claudeailimits

import "os"

// envFlag is the environment variable that toggles the Claude.ai limits
// system. The system is enabled by default; setting the variable to the
// literal string "0" disables it.
const envFlag = "CLAUDE_FEATURE_CLAUDEAI_LIMITS"

// IsClaudeAILimitsEnabled reports whether the Claude.ai rate limit
// observation system is active. It defaults to true and only returns false
// when the environment variable CLAUDE_FEATURE_CLAUDEAI_LIMITS is set to "0".
func IsClaudeAILimitsEnabled() bool {
	value := os.Getenv(envFlag)
	if value == "0" {
		return false
	}
	return true
}
