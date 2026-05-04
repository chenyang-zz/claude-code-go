package webfetchsummary

import (
	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
)

// IsWebFetchSummaryEnabled reports whether the web fetch summarization
// helper is enabled. Default is off — set CLAUDE_FEATURE_WEB_FETCH_SUMMARY=1
// to opt in.
func IsWebFetchSummaryEnabled() bool {
	return featureflag.IsEnabled(featureflag.FlagWebFetchSummary)
}
