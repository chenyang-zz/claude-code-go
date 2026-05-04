package shellprefix

import (
	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
)

// IsShellPrefixEnabled reports whether the shell prefix extraction helper is
// enabled. Default is off — set CLAUDE_FEATURE_SHELL_PREFIX=1 to opt in.
func IsShellPrefixEnabled() bool {
	return featureflag.IsEnabled(featureflag.FlagShellPrefix)
}
