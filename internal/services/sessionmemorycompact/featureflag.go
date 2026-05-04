package sessionmemorycompact

import (
	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
)

// IsSessionMemoryCompactEnabled reports whether the session memory compaction
// feature flag is enabled.
func IsSessionMemoryCompactEnabled() bool {
	return featureflag.IsEnabled(featureflag.FlagSessionMemoryCompact)
}
