package bash

import (
	"regexp"
	"strings"
)

// SedEditInfo stores the extracted components of a sed in-place edit command.
type SedEditInfo struct {
	// FilePath is the file being edited.
	FilePath string
	// Pattern is the search regex.
	Pattern string
	// Replacement is the replacement string.
	Replacement string
	// Flags are the substitution flags (g, i, etc.).
	Flags string
	// ExtendedRegex is true when -E or -r flag is present.
	ExtendedRegex bool
}

// isSedInPlaceEdit reports whether command is a simple sed -i substitution.
func isSedInPlaceEdit(command string) bool {
	return parseSedEditCommand(command) != nil
}

// parseSedEditCommand parses a sed edit command and returns the edit information.
// Returns nil if the command is not a valid sed in-place edit.
func parseSedEditCommand(command string) *SedEditInfo {
	trimmed := strings.TrimSpace(command)

	// Must start with "sed" followed by whitespace.
	if !strings.HasPrefix(trimmed, "sed") {
		return nil
	}
	rest := trimmed[len("sed"):]
	if len(rest) == 0 || (rest[0] != ' ' && rest[0] != '\t') {
		return nil
	}

	withoutSed := strings.TrimSpace(rest)
	tokens, ok := tokenizeShellCommand(withoutSed)
	if !ok {
		return nil
	}

	var hasInPlaceFlag bool
	var extendedRegex bool
	var expression string
	var hasExpression bool
	var filePath string
	var hasFilePath bool

	i := 0
	for i < len(tokens) {
		arg := tokens[i]

		// Handle -i flag (with or without backup suffix)
		if arg == "-i" || arg == "--in-place" {
			hasInPlaceFlag = true
			i++
			// On macOS, -i requires a suffix argument (even if empty string).
			// Check if next arg looks like a backup suffix.
			if i < len(tokens) {
				nextArg := tokens[i]
				if nextArg != "" && !strings.HasPrefix(nextArg, "-") &&
					(nextArg == "" || strings.HasPrefix(nextArg, ".")) {
					i++ // Skip the backup suffix
				}
			}
			continue
		}
		if strings.HasPrefix(arg, "-i") && arg != "-i" {
			// -i.bak or similar (inline suffix)
			hasInPlaceFlag = true
			i++
			continue
		}

		// Handle extended regex flags
		if arg == "-E" || arg == "-r" || arg == "--regexp-extended" {
			extendedRegex = true
			i++
			continue
		}

		// Handle -e flag with expression
		if arg == "-e" || arg == "--expression" {
			if i+1 < len(tokens) {
				if hasExpression {
					return nil // Only support single expression
				}
				expression = tokens[i+1]
				hasExpression = true
				i += 2
				continue
			}
			return nil
		}
		if strings.HasPrefix(arg, "--expression=") {
			if hasExpression {
				return nil
			}
			expression = strings.TrimPrefix(arg, "--expression=")
			hasExpression = true
			i++
			continue
		}

		// Unknown flag — not safe to parse
		if strings.HasPrefix(arg, "-") {
			return nil
		}

		// Non-flag argument
		if !hasExpression {
			expression = arg
			hasExpression = true
		} else if !hasFilePath {
			filePath = arg
			hasFilePath = true
		} else {
			// More than one file — not supported
			return nil
		}
		i++
	}

	// Must have -i flag, expression, and file path
	if !hasInPlaceFlag || !hasExpression || !hasFilePath {
		return nil
	}

	// Parse the substitution expression: s/pattern/replacement/flags
	// Only support / as delimiter
	if !strings.HasPrefix(expression, "s/") {
		return nil
	}

	exprBody := expression[2:] // Skip "s/"

	var pattern strings.Builder
	var replacement strings.Builder
	var flags strings.Builder
	state := "pattern" // "pattern" | "replacement" | "flags"

	j := 0
	for j < len(exprBody) {
		char := exprBody[j]

		if char == '\\' && j+1 < len(exprBody) {
			escSeq := exprBody[j : j+2]
			switch state {
			case "pattern":
				pattern.WriteString(escSeq)
			case "replacement":
				replacement.WriteString(escSeq)
			default:
				flags.WriteString(escSeq)
			}
			j += 2
			continue
		}

		if char == '/' {
			switch state {
			case "pattern":
				state = "replacement"
			case "replacement":
				state = "flags"
			default:
				// Extra delimiter in flags — unexpected
				return nil
			}
			j++
			continue
		}

		switch state {
		case "pattern":
			pattern.WriteByte(char)
		case "replacement":
			replacement.WriteByte(char)
		default:
			flags.WriteByte(char)
		}
		j++
	}

	// Must have found all three parts
	if state != "flags" {
		return nil
	}

	// Validate flags — only allow safe substitution flags
	validFlagsPattern := "^[gpimIM1-9]*$"
	matched, err := matchString(validFlagsPattern, flags.String())
	if err != nil || !matched {
		return nil
	}

	return &SedEditInfo{
		FilePath:      filePath,
		Pattern:       pattern.String(),
		Replacement:   replacement.String(),
		Flags:         flags.String(),
		ExtendedRegex: extendedRegex,
	}
}

// tokenizeShellCommand performs a lightweight shell tokenization sufficient
// for common sed command shapes. It handles single quotes, double quotes,
// backslash escapes, and whitespace separation. Returns false if quotes are
// unbalanced.
func tokenizeShellCommand(input string) ([]string, bool) {
	var tokens []string
	var current strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false

	for i := 0; i < len(input); i++ {
		char := input[i]

		if escaped {
			current.WriteByte(char)
			escaped = false
			continue
		}

		if char == '\\' && !inSingleQuote {
			escaped = true
			continue
		}

		if char == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			continue
		}

		if char == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			continue
		}

		if isShellWhitespace(char) {
			if inSingleQuote || inDoubleQuote {
				current.WriteByte(char)
			} else {
				if current.Len() > 0 {
					tokens = append(tokens, current.String())
					current.Reset()
				}
			}
			continue
		}

		current.WriteByte(char)
	}

	if inSingleQuote || inDoubleQuote {
		return nil, false
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens, true
}

// isShellWhitespace reports whether b is a shell token separator.
func isShellWhitespace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// matchString is a thin wrapper for regexp pattern matching.
func matchString(pattern, s string) (bool, error) {
	return regexpMatchString(pattern, s)
}

// regexpMatchString is the actual regexp implementation, extracted as a
// variable so tests can stub it if needed.
var regexpMatchString = func(pattern, s string) (bool, error) {
	return regexp.MatchString(pattern, s)
}
