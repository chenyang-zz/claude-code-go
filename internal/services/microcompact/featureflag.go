package microcompact

import (
	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
)

// IsMicroCompactEnabled reports whether the time-based microcompact service is
// enabled. Default is off — set CLAUDE_FEATURE_MICRO_COMPACT=1 to opt in.
func IsMicroCompactEnabled() bool {
	return featureflag.IsEnabled(featureflag.FlagMicroCompact)
}
