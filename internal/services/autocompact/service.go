package autocompact

import (
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// modelContextWindows maps known model identifiers to their context window
// sizes. Unknown models default to 200K.
var modelContextWindows = map[string]int{
	"claude-sonnet-4-20250514":      200_000,
	"claude-sonnet-4-6-20250610":    200_000,
	"claude-opus-4-20250514":        200_000,
	"claude-opus-4-6-20250610":      200_000,
	"claude-haiku-4-20250514":       200_000,
	"claude-sonnet-3-5-20241022":    200_000,
	"claude-sonnet-3-5-20240620":    200_000,
	"claude-opus-3-5-20240620":      200_000,
	"claude-haiku-3-5-20241022":     200_000,
}

// defaultContextWindow is the fallback when the model is not in the known map.
const defaultContextWindow = 200_000

// GetEffectiveContextWindowSize returns the context window size minus the
// reserved output tokens for summary generation. An optional env override
// (CLAUDE_CODE_AUTO_COMPACT_WINDOW) can further restrict the window.
func GetEffectiveContextWindowSize(model string) int {
	window, ok := modelContextWindows[model]
	if !ok {
		window = defaultContextWindow
	}

	// Apply optional env override
	if v := os.Getenv("CLAUDE_CODE_AUTO_COMPACT_WINDOW"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			if parsed < window {
				window = parsed
			}
		}
	}

	output := GetMaxOutputTokensForModel(model)
	reserved := min(output, MAX_OUTPUT_TOKENS_FOR_SUMMARY)
	result := window - reserved
	if result < 0 {
		result = 0
	}
	return result
}

// GetMaxOutputTokensForModel returns the maximum output tokens for the given
// model. Unknown models default to 8192.
func GetMaxOutputTokensForModel(model string) int {
	// Most Claude models support 8192 output tokens. Specific models may
	// differ, but the TS side uses a simple heuristic based on the model name.
	if strings.Contains(model, "haiku") {
		return 8192
	}
	if strings.Contains(model, "sonnet") {
		return 8192
	}
	if strings.Contains(model, "opus") {
		return 8192
	}
	return 8192
}

// GetAutoCompactThreshold returns the token threshold at which auto-compact
// should trigger. An env override (CLAUDE_AUTOCOMPACT_PCT_OVERRIDE) can set
// it as a percentage of the effective context window.
func GetAutoCompactThreshold(model string) int {
	effectiveWindow := GetEffectiveContextWindowSize(model)
	threshold := effectiveWindow - AUTOCOMPACT_BUFFER_TOKENS

	// Override for easier testing of autocompact
	if v := os.Getenv("CLAUDE_AUTOCOMPACT_PCT_OVERRIDE"); v != "" {
		if pct, err := strconv.ParseFloat(v, 64); err == nil && pct > 0 && pct <= 100 {
			pctThreshold := int(math.Floor(float64(effectiveWindow) * (pct / 100.0)))
			if pctThreshold < threshold {
				threshold = pctThreshold
			}
		}
	}

	return threshold
}

// CalculateTokenWarningState computes the token warning state for the given
// token usage and model. Returns percentage remaining and threshold flags.
func CalculateTokenWarningState(tokenUsage int, model string) TokenWarningState {
	threshold := GetAutoCompactThreshold(model)
	effectiveWindow := GetEffectiveContextWindowSize(model)

	// When auto-compact is disabled, use the effective window as the threshold
	var cmpThreshold int
	if IsAutoCompactEnabled() {
		cmpThreshold = threshold
	} else {
		cmpThreshold = effectiveWindow
	}

	percentLeft := 0
	if cmpThreshold > 0 {
		percentLeft = max(0, int(math.Round(float64(cmpThreshold-tokenUsage)/float64(cmpThreshold)*100)))
	}

	warningThreshold := threshold - WARNING_THRESHOLD_BUFFER_TOKENS
	errorThreshold := threshold - ERROR_THRESHOLD_BUFFER_TOKENS

	// Blocking limit
	blockingLimit := effectiveWindow - MANUAL_COMPACT_BUFFER_TOKENS
	if v := os.Getenv("CLAUDE_CODE_BLOCKING_LIMIT_OVERRIDE"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			blockingLimit = parsed
		}
	}

	return TokenWarningState{
		PercentLeft:               percentLeft,
		IsAboveWarningThreshold:   tokenUsage >= warningThreshold,
		IsAboveErrorThreshold:     tokenUsage >= errorThreshold,
		IsAboveAutoCompactThreshold: IsAutoCompactEnabled() && tokenUsage >= threshold,
		IsAtBlockingLimit:         tokenUsage >= blockingLimit,
	}
}

// IsAutoCompactEnabled reports whether the auto-compact feature is enabled.
// Checks env overrides (DISABLE_COMPACT, DISABLE_AUTO_COMPACT) and the
// feature flag. The feature flag is off by default.
func IsAutoCompactEnabled() bool {
	if isEnvTruthy("DISABLE_COMPACT") {
		return false
	}
	if isEnvTruthy("DISABLE_AUTO_COMPACT") {
		return false
	}
	return featureflag.IsEnabled(featureflag.FlagAutoCompact)
}

// ShouldAutoCompact determines whether auto-compact should trigger for the
// given messages and model. Returns true when token usage exceeds the
// auto-compact threshold and all guard conditions pass.
//
// Simplified from TS: recursion guards for session_memory/compact querySources
// are stubbed; context-collapse guards are skipped (the Go side doesn't have
// the context-collapse system yet); reactive-compact guards are skipped.
func ShouldAutoCompact(messagesCount int, estimatedTokens int, model string) bool {
	if !IsAutoCompactEnabled() {
		return false
	}

	threshold := GetAutoCompactThreshold(model)
	logger.DebugCF("autocompact", "shouldAutoCompact check", map[string]any{
		"estimated_tokens": estimatedTokens,
		"threshold":        threshold,
		"effective_window": GetEffectiveContextWindowSize(model),
	})

	return estimatedTokens >= threshold
}

// isEnvTruthy returns true if the named environment variable is set to a
// truthy value ("1", "true", "yes", case-insensitive).
func isEnvTruthy(name string) bool {
	v := os.Getenv(name)
	switch strings.ToLower(v) {
	case "1", "true", "yes":
		return true
	}
	return false
}

// min returns the smaller of two ints.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max returns the larger of two ints.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
