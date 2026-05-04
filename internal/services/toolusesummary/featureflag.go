package toolusesummary

import (
	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
)

// FlagToolUseSummary is the feature flag environment-variable suffix for
// the tool use summary helper. Mirrors the constant declared in
// internal/core/featureflag for symmetry.
const FlagToolUseSummary = "TOOL_USE_SUMMARY"

// IsToolUseSummaryEnabled reports whether the tool use summary helper is
// enabled. Default is off — set CLAUDE_FEATURE_TOOL_USE_SUMMARY=1 to opt in.
func IsToolUseSummaryEnabled() bool {
	return featureflag.IsEnabled(featureflag.FlagToolUseSummary)
}
