package sessiontitle

import (
	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
)

// IsSessionTitleEnabled reports whether the session title helper is
// enabled. Default is off — set CLAUDE_FEATURE_SESSION_TITLE=1 to opt in.
func IsSessionTitleEnabled() bool {
	return featureflag.IsEnabled(featureflag.FlagSessionTitle)
}
