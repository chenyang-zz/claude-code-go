package teammemsync

import (
	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
)

// IsFeatureEnabled reports whether the team memory sync feature flag is
// enabled. This is the flag-only check used by consumers that have already
// validated the higher-level IsTeamMemoryEnabled gate.
func IsFeatureEnabled() bool {
	return featureflag.IsEnabled(featureflag.FlagTeamMemorySync)
}
