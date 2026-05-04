package rename

import (
	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
)

// IsRenameSuggestionEnabled reports whether the rename suggestion helper is
// enabled. Default is off — set CLAUDE_FEATURE_RENAME_SUGGESTION=1 to opt in.
func IsRenameSuggestionEnabled() bool {
	return featureflag.IsEnabled(featureflag.FlagRenameSuggestion)
}
