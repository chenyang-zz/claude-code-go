package preventsleep

import (
	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
)

// IsPreventSleepEnabled reports whether the prevent-sleep feature is enabled
// via the FlagPreventSleep flag (CLAUDE_FEATURE_PREVENT_SLEEP=1).
func IsPreventSleepEnabled() bool {
	return featureflag.IsEnabled(featureflag.FlagPreventSleep)
}
