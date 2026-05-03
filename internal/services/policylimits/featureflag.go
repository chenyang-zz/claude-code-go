package policylimits

import (
	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
)

// IsPolicyLimitsEnabled reports whether the policy limits system is active.
// Controlled by the CLAUDE_FEATURE_POLICY_LIMITS environment variable.
func IsPolicyLimitsEnabled() bool {
	return featureflag.IsEnabled(featureflag.FlagPolicyLimits)
}
