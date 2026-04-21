package engine

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// budgetMultipliers maps token budget suffixes to their multiplier values.
var budgetMultipliers = map[string]int{
	"k": 1_000,
	"m": 1_000_000,
	"b": 1_000_000_000,
}

// Shorthand patterns anchored to start/end to avoid false positives in
// natural language. Verbose (use/spend 2M tokens) matches anywhere.
var (
	// shorthandStartRE matches "+500k" at the beginning of the text.
	shorthandStartRE = regexp.MustCompile(`(?i)^\s*\+(\d+(?:\.\d+)?)\s*(k|m|b)\b`)
	// shorthandEndRE matches "+500k" near the end of the text.
	// The leading whitespace is captured rather than using lookbehind to avoid
	// compatibility issues across regex engines.
	shorthandEndRE = regexp.MustCompile(`(?i)\s\+(\d+(?:\.\d+)?)\s*(k|m|b)\s*[.!?]?\s*$`)
	// verboseRE matches "use 2M tokens" or "spend 500k tokens" anywhere.
	verboseRE = regexp.MustCompile(`(?i)\b(?:use|spend)\s+(\d+(?:\.\d+)?)\s*(k|m|b)\s*tokens?\b`)
)

// ParseTokenBudget extracts a token budget from user input text.
// It recognises three formats:
//   - "+500k" shorthand at the start of the text
//   - "+1.5m" shorthand near the end of the text (preceded by whitespace)
//   - "use/spend 2M tokens" verbose form anywhere in the text
//
// Returns the parsed token count and true if a budget was found,
// or 0 and false if the text contains no budget directive.
func ParseTokenBudget(text string) (int, bool) {
	if match := shorthandStartRE.FindStringSubmatch(text); match != nil {
		return parseBudgetMatch(match[1], match[2]), true
	}
	if match := shorthandEndRE.FindStringSubmatch(text); match != nil {
		return parseBudgetMatch(match[1], match[2]), true
	}
	if match := verboseRE.FindStringSubmatch(text); match != nil {
		return parseBudgetMatch(match[1], match[2]), true
	}
	return 0, false
}

// parseBudgetMatch converts a numeric value string and suffix into a token count.
func parseBudgetMatch(value, suffix string) int {
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	multiplier := budgetMultipliers[strings.ToLower(suffix)]
	return int(f * float64(multiplier))
}

// FormatBudgetNudgeMessage generates a continuation nudge that tells the model
// how far along it is in the token budget and encourages it to keep producing.
// The message format matches the TypeScript implementation's
// getBudgetContinuationMessage: "Stopped at {pct}% of token target
// ({turnTokens} / {budget}). Keep working — do not summarize."
func FormatBudgetNudgeMessage(pct int, turnTokens int, budget int) string {
	return fmt.Sprintf(
		"Stopped at %d%% of token target (%s / %s). Keep working \u2014 do not summarize.",
		pct, formatNumber(turnTokens), formatNumber(budget),
	)
}

// formatNumber formats a positive integer with comma-thousand separators
// (e.g. 500000 → "500,000") matching the TS Intl.NumberFormat('en-US') output.
func formatNumber(n int) string {
	if n < 0 {
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return b.String()
}

// ComputeTaskBudgetRemaining decrements the remaining budget by the context
// window size represented by the given usage. It returns the updated remaining
// value. The TS implementation reads usage.iterations[-1] for the exact context
// window; Go falls back to InputTokens+OutputTokens which is the TS fallback
// path as well. Callers should pass the actual summary request usage for a
// compaction step rather than a locally estimated transcript size.
func ComputeTaskBudgetRemaining(currentRemaining int, totalBudget int, inputTokens int, outputTokens int) int {
	contextSize := inputTokens + outputTokens
	remaining := currentRemaining
	if remaining <= 0 {
		remaining = totalBudget
	}
	remaining -= contextSize
	if remaining < 0 {
		remaining = 0
	}
	return remaining
}
