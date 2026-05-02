package autodream

import (
	"os"
	"strconv"

	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
)

// AutoDreamConfig holds the scheduling knobs for autoDream consolidation.
type AutoDreamConfig struct {
	// MinHours is the minimum hours since last consolidation before a new
	// consolidation can be triggered.
	MinHours int
	// MinSessions is the minimum number of sessions touched since last
	// consolidation required to trigger a new consolidation.
	MinSessions int
}

const (
	// defaultMinHours is the default minimum hours between consolidations.
	defaultMinHours = 24
	// defaultMinSessions is the default minimum session count to trigger.
	defaultMinSessions = 5
)

// getConfig returns the autoDream scheduling configuration.
// Defaults: minHours=24, minSessions=5.
// Overrides via env: CLAUDE_CODE_AUTO_DREAM_MIN_HOURS, CLAUDE_CODE_AUTO_DREAM_MIN_SESSIONS.
// GrowthBook feature flag integration is a stub — replace env reads with
// GrowthBook SDK calls when the SDK is migrated.
func getConfig() AutoDreamConfig {
	cfg := AutoDreamConfig{
		MinHours:    defaultMinHours,
		MinSessions: defaultMinSessions,
	}

	if v := os.Getenv("CLAUDE_CODE_AUTO_DREAM_MIN_HOURS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MinHours = n
		}
	}

	if v := os.Getenv("CLAUDE_CODE_AUTO_DREAM_MIN_SESSIONS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MinSessions = n
		}
	}

	return cfg
}

// isAutoDreamEnabled checks whether autoDream consolidation should run.
// Gate chain: CLAUDE_FEATURE_AUTO_DREAM=1 must be set (off-by-default).
// GrowthBook fallback is a stub — always returns false.
func isAutoDreamEnabled() bool {
	return featureflag.IsEnabled(featureflag.FlagAutoDream)
}
