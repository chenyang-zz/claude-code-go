package agentsummary

import "github.com/sheepzhao/claude-code-go/internal/core/featureflag"

// IsAgentSummaryEnabled reports whether the agent summary feature is enabled
// via the AGENT_SUMMARY feature flag.
func IsAgentSummaryEnabled() bool {
	return featureflag.IsEnabled(featureflag.FlagAgentSummary)
}
