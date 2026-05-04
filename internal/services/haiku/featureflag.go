// Package haiku provides a thin Haiku single-prompt query helper used by
// downstream services such as toolusesummary. It wraps the existing
// model.Client (typically *anthropic.Client) and aggregates the streaming
// response into a single QueryResult.
//
// The package is gated by FlagHaikuQuery (CLAUDE_FEATURE_HAIKU). Unlike most
// flags in core/featureflag, FlagHaikuQuery uses reverse-default semantics:
// the helper is on by default because it is treated as foundational
// infrastructure shared by many downstream call sites. Set
// CLAUDE_FEATURE_HAIKU=0 (or "false") to opt out.
package haiku

import (
	"os"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
)

// IsHaikuEnabled reports whether the haiku query helper is enabled.
//
// Reverse-default semantics: returns true unless CLAUDE_FEATURE_HAIKU is
// explicitly set to "0" or "false" (case-insensitive). This intentionally
// differs from featureflag.IsEnabled, whose generic reading treats unset as
// disabled.
func IsHaikuEnabled() bool {
	val := os.Getenv("CLAUDE_FEATURE_" + featureflag.FlagHaikuQuery)
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "0", "false":
		return false
	}
	return true
}
