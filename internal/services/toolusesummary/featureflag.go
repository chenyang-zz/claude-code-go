package toolusesummary

import (
	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
)

// IsToolUseSummaryEnabled reports whether the tool use summary helper is
// enabled. Default is off — set CLAUDE_FEATURE_TOOL_USE_SUMMARY=1 to opt in.
func IsToolUseSummaryEnabled() bool {
	return featureflag.IsEnabled(featureflag.FlagToolUseSummary)
}
