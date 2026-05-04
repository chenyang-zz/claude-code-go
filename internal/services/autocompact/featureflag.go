package autocompact

import (
	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
)

// IsAutoCompactEnabledByFlag reports whether the auto-compact feature flag
// is enabled. Unlike IsAutoCompactEnabled (which also checks env overrides),
// this only checks the feature flag, useful for testing.
func IsAutoCompactEnabledByFlag() bool {
	return featureflag.IsEnabled(featureflag.FlagAutoCompact)
}
