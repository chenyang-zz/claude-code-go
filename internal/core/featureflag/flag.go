package featureflag

import "os"

// Well-known feature flag names. Each flag is controlled by the environment
// variable CLAUDE_FEATURE_{NAME} (e.g. CLAUDE_FEATURE_TOKEN_BUDGET).
const (
	// FlagTokenBudget gates the +500k token budget input parsing and
	// automatic budget continuation in the engine loop.
	FlagTokenBudget = "TOKEN_BUDGET"
)

// envPrefix is the environment variable prefix used for all feature flags.
const envPrefix = "CLAUDE_FEATURE_"

// IsEnabled reports whether the named feature flag is enabled.
// A flag is enabled when the environment variable CLAUDE_FEATURE_{NAME}
// is set to exactly "1". All other values (including unset) mean disabled.
func IsEnabled(name string) bool {
	return os.Getenv(envPrefix+name) == "1"
}
