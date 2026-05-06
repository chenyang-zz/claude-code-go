package feedback

import (
	"regexp"
	"strings"
)

// redactPatterns contains precompiled regular expressions for sensitive
// information redaction. Uses only RE2-compatible patterns (no lookahead/lookbehind).
var redactPatterns = []struct {
	re   *regexp.Regexp
	repl string
}{
	// Anthropic API keys with quotes
	{regexp.MustCompile(`"(sk-ant[A-Za-z0-9]{24,})"`), `"[REDACTED_API_KEY]"`},
	// Anthropic API keys without quotes (word-boundary anchored)
	{regexp.MustCompile(`\bsk-ant-?[A-Za-z0-9_-]{10,}\b`), "[REDACTED_API_KEY]"},
	// AWS keys in quoted format
	{regexp.MustCompile(`AWS key: "(AWS[A-Z0-9]{20,})"`), `AWS key: "[REDACTED_AWS_KEY]"`},
	// AWS AKIAXXX keys (word-boundary anchored)
	{regexp.MustCompile(`\b(AKIA[A-Z0-9]{16})\b`), "[REDACTED_AWS_KEY]"},
	// Google Cloud keys (word-boundary anchored)
	{regexp.MustCompile(`\bAIza[A-Za-z0-9_-]{35,}\b`), "[REDACTED_GCP_KEY]"},
	// Vertex AI service account keys
	{regexp.MustCompile(`\b[a-z0-9-]+@[a-z0-9-]+\.iam\.gserviceaccount\.com\b`), "[REDACTED_GCP_SERVICE_ACCOUNT]"},
	// Generic API keys in headers (case-insensitive match on x-api-key)
	{regexp.MustCompile(`(?i)(["']?x-api-key["']?\s*[:=]\s*["']?)[^"',\s)}\]]+`), "${1}[REDACTED_API_KEY]"},
	// Authorization headers and Bearer tokens
	{regexp.MustCompile(`(?i)(["']?authorization["']?\s*[:=]\s*["']?(bearer\s+)?)[^"',\s)}\]]+`), "${1}[REDACTED_TOKEN]"},
	// AWS environment variables
	{regexp.MustCompile(`(?i)(AWS[_-][A-Za-z0-9_]+\s*[=:]\s*)["']?[^"',\s)}\]]+["']?`), "${1}[REDACTED_AWS_VALUE]"},
	// GCP environment variables
	{regexp.MustCompile(`(?i)(GOOGLE[_-][A-Za-z0-9_]+\s*[=:]\s*)["']?[^"',\s)}\]]+["']?`), "${1}[REDACTED_GCP_VALUE]"},
	// Environment variables with sensitive names
	{regexp.MustCompile(`(?i)((API[-_]?KEY|TOKEN|SECRET|PASSWORD)\s*[=:]\s*)["']?[^"',\s)}\]]+["']?`), "${1}[REDACTED]"},
}

// RedactSensitiveInfo removes API keys, tokens, and other sensitive
// information from text. This is a critical security component that must be
// called before including any user-provided or system-generated text in
// feedback submissions.
func RedactSensitiveInfo(text string) string {
	result := text
	for _, p := range redactPatterns {
		result = p.re.ReplaceAllString(result, p.repl)
	}
	return result
}

// SanitizeErrors applies sensitive information redaction to a slice of
// error entries, returning a new slice with redacted content.
func SanitizeErrors(errors []ErrorInfo) []ErrorInfo {
	if len(errors) == 0 {
		return nil
	}
	out := make([]ErrorInfo, len(errors))
	for i, e := range errors {
		out[i] = ErrorInfo{
			Error:     RedactSensitiveInfo(e.Error),
			Timestamp: e.Timestamp,
		}
	}
	return out
}

// CreateFallbackTitle generates a safe fallback title from the feedback
// description when Haiku title generation fails.
func CreateFallbackTitle(description string) string {
	firstLine := strings.SplitN(description, "\n", 2)[0]
	if len(firstLine) <= 60 && len(firstLine) > 5 {
		return firstLine
	}
	if len(firstLine) > 60 {
		truncated := firstLine[:60]
		if lastSpace := strings.LastIndex(truncated, " "); lastSpace > 30 {
			truncated = truncated[:lastSpace]
		}
		truncated += "..."
		if len(truncated) < 10 {
			return "Bug Report"
		}
		return truncated
	}
	return "Bug Report"
}
