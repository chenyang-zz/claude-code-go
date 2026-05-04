package notifier

import "github.com/sheepzhao/claude-code-go/internal/core/featureflag"

// IsNotifierEnabled reports whether notifier dispatch is enabled. Returns
// true when CLAUDE_FEATURE_NOTIFIER=1 (see featureflag.FlagNotifier).
func IsNotifierEnabled() bool {
	return featureflag.IsEnabled(featureflag.FlagNotifier)
}
