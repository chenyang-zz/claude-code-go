package bash

import (
	"strings"
)

// RedirectInfo stores parsed output redirection metadata from one Bash command.
type RedirectInfo struct {
	// StdoutFile is the target file path for stdout redirection (> or >>).
	StdoutFile string
	// StderrFile is the target file path for stderr redirection (2> or 2>>).
	StderrFile string
	// Append reports whether the redirection uses append mode (>> or 2>>).
	Append bool
	// Command is the command string with redirection tokens stripped.
	Command string
}

// parseOutputRedirection scans one shell command string for basic output
// redirection tokens (>, >>, 2>, 2>>) and returns the captured file paths
// together with the command stripped of those tokens.
//
// This is intentionally a minimal parser — it handles the common cases
// that the model generates. Complex heredocs, nested quotes, and
// fd-duplicate redirects (>&) are out of scope for the current batch.
func parseOutputRedirection(command string) RedirectInfo {
	info := RedirectInfo{Command: command}

	fields := splitCommandRespectingQuotes(command)
	var kept []string
	for i := 0; i < len(fields); i++ {
		tok := fields[i]
		switch tok {
		case ">>":
			info.Append = true
			if i+1 < len(fields) {
				info.StdoutFile = unquote(fields[i+1])
				i++
			}
		case ">":
			if i+1 < len(fields) {
				info.StdoutFile = unquote(fields[i+1])
				i++
			}
		case "2>>":
			info.Append = true
			if i+1 < len(fields) {
				info.StderrFile = unquote(fields[i+1])
				i++
			}
		case "2>":
			if i+1 < len(fields) {
				info.StderrFile = unquote(fields[i+1])
				i++
			}
		default:
			kept = append(kept, tok)
		}
	}

	info.Command = strings.Join(kept, " ")
	return info
}

// splitCommandRespectingQuotes splits a command string into tokens,
// preserving quoted substrings as single tokens. Redirection operators
// (>, >>, 2>, 2>>) are treated as separate tokens even when adjacent
// to other text.
func splitCommandRespectingQuotes(command string) []string {
	var tokens []string
	var current strings.Builder
	inQuote := rune(0)

	flush := func() {
		if current.Len() > 0 {
			tokens = append(tokens, current.String())
			current.Reset()
		}
	}

	runes := []rune(command)
	for i := 0; i < len(runes); i++ {
		r := runes[i]

		if inQuote != 0 {
			current.WriteRune(r)
			if r == inQuote {
				inQuote = 0
			}
			continue
		}

		switch r {
		case '"', '\'':
			flush()
			current.WriteRune(r)
			inQuote = r
		case ' ', '\t':
			flush()
		case '>':
			flush()
			// Check for >> or >
			if i+1 < len(runes) && runes[i+1] == '>' {
				tokens = append(tokens, ">>")
				i++
			} else {
				tokens = append(tokens, ">")
			}
		case '2':
			// Check for 2> or 2>>
			if i+1 < len(runes) && runes[i+1] == '>' {
				flush()
				if i+2 < len(runes) && runes[i+2] == '>' {
					tokens = append(tokens, "2>>")
					i += 2
				} else {
					tokens = append(tokens, "2>")
					i++
				}
			} else {
				current.WriteRune(r)
			}
		default:
			current.WriteRune(r)
		}
	}
	flush()
	return tokens
}

// unquote removes matching surrounding single or double quotes from a string.
func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
