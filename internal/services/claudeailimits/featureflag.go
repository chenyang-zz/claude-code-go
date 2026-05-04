package claudeailimits

import (
	"os"

	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
)

// IsClaudeAILimitsEnabled reports whether the Claude.ai rate limit
// observation system is active. It defaults to true and only returns false
// when the environment variable CLAUDE_FEATURE_CLAUDEAI_LIMITS is set to "0".
func IsClaudeAILimitsEnabled() bool {
	value := os.Getenv("CLAUDE_FEATURE_" + featureflag.FlagClaudeAILimits)
	if value == "0" {
		return false
	}
	return true
}
