package datetimeparser

import (
	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
)

// IsDateTimeParserEnabled reports whether the date/time parser helper is
// enabled. Default is off — set CLAUDE_FEATURE_DATETIME_PARSER=1 to opt in.
func IsDateTimeParserEnabled() bool {
	return featureflag.IsEnabled(featureflag.FlagDateTimeParser)
}
