package bash

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"strings"
)

// placeholder constants for BRE→ERE conversion. They use null-byte prefixes
// that never appear in user input.
const (
	backslashPlaceholder = "\x00BACKSLASH\x00"
	plusPlaceholder      = "\x00PLUS\x00"
	questionPlaceholder  = "\x00QUESTION\x00"
	pipePlaceholder      = "\x00PIPE\x00"
	lparenPlaceholder    = "\x00LPAREN\x00"
	rparenPlaceholder    = "\x00RPAREN\x00"
)

// applySedSubstitution applies a sed substitution to file content.
// Returns the new content after applying the substitution.
// If the regex is invalid, returns the original content unchanged.
func applySedSubstitution(content string, sedInfo *SedEditInfo) string {
	// Convert sed pattern to Go regex pattern.
	goPattern := sedInfo.Pattern
	// Unescape \/ to /.
	goPattern = strings.ReplaceAll(goPattern, `\/`, "/")

	// In BRE mode (no -E flag), metacharacters have opposite escaping:
	// BRE: \+ means "one or more", + is literal.
	// ERE/Go: + means "one or more", \+ is literal.
	if !sedInfo.ExtendedRegex {
		goPattern = convertBREtoERE(goPattern)
	}

	// Build regex flags as Go prefix groups.
	var goFlags string
	if strings.Contains(sedInfo.Flags, "i") || strings.Contains(sedInfo.Flags, "I") {
		goFlags += "(?i)"
	}
	if strings.Contains(sedInfo.Flags, "m") || strings.Contains(sedInfo.Flags, "M") {
		goFlags += "(?m)"
	}
	fullPattern := goFlags + goPattern

	// Handle replacement string.
	// Generate a unique placeholder with random salt to prevent injection attacks.
	salt := make([]byte, 8)
	rand.Read(salt)
	saltStr := hex.EncodeToString(salt)
	escapedAmpPlaceholder := "___ESCAPED_AMPERSAND_" + saltStr + "___"

	replacement := sedInfo.Replacement
	// Unescape \/ to /.
	replacement = strings.ReplaceAll(replacement, `\/`, "/")
	// Escape \& to a placeholder.
	replacement = strings.ReplaceAll(replacement, `\&`, escapedAmpPlaceholder)
	// Convert & to $0 (full match in Go).
	replacement = strings.ReplaceAll(replacement, "&", "$0")
	// Convert placeholder back to literal &.
	replacement = strings.ReplaceAll(replacement, escapedAmpPlaceholder, "&")
	// Convert \n to newline, \t to tab.
	replacement = strings.ReplaceAll(replacement, `\n`, "\n")
	replacement = strings.ReplaceAll(replacement, `\t`, "\t")

	re, err := regexp.Compile(fullPattern)
	if err != nil {
		return content
	}

	if strings.Contains(sedInfo.Flags, "g") {
		return re.ReplaceAllString(content, replacement)
	}
	return replaceFirstString(re, content, replacement)
}

// convertBREtoERE converts a Basic Regular Expression pattern to Extended
// Regular Expression semantics for Go's regexp package.
func convertBREtoERE(pattern string) string {
	// Step 1: Protect literal backslashes (\\) first.
	result := strings.ReplaceAll(pattern, `\\`, backslashPlaceholder)
	// Step 2: Replace escaped metacharacters with placeholders.
	result = strings.ReplaceAll(result, `\+`, plusPlaceholder)
	result = strings.ReplaceAll(result, `\?`, questionPlaceholder)
	result = strings.ReplaceAll(result, `\|`, pipePlaceholder)
	result = strings.ReplaceAll(result, `\(`, lparenPlaceholder)
	result = strings.ReplaceAll(result, `\)`, rparenPlaceholder)
	// Step 3: Escape unescaped metacharacters (these are literal in BRE).
	result = strings.ReplaceAll(result, "+", `\+`)
	result = strings.ReplaceAll(result, "?", `\?`)
	result = strings.ReplaceAll(result, "|", `\|`)
	result = strings.ReplaceAll(result, "(", `\(`)
	result = strings.ReplaceAll(result, ")", `\)`)
	// Step 4: Replace placeholders with their unescaped ERE equivalents.
	result = strings.ReplaceAll(result, backslashPlaceholder, `\\`)
	result = strings.ReplaceAll(result, plusPlaceholder, `+`)
	result = strings.ReplaceAll(result, questionPlaceholder, `?`)
	result = strings.ReplaceAll(result, pipePlaceholder, `|`)
	result = strings.ReplaceAll(result, lparenPlaceholder, `(`)
	result = strings.ReplaceAll(result, rparenPlaceholder, `)`)
	return result
}

// replaceFirstString replaces only the first match of re in src with repl.
func replaceFirstString(re *regexp.Regexp, src, repl string) string {
	loc := re.FindStringIndex(src)
	if loc == nil {
		return src
	}
	return src[:loc[0]] + re.ReplaceAllString(src[loc[0]:loc[1]], repl) + src[loc[1]:]
}
