package repl

import "strings"

// IsSlashCommand reports whether the input starts with a slash-prefixed command token.
func IsSlashCommand(input string) bool {
	return strings.HasPrefix(strings.TrimSpace(input), "/")
}
