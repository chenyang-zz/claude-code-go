package repl

import (
	"fmt"
	"strings"
)

// ParsedInput carries the minimum REPL parse result needed for one CLI turn.
type ParsedInput struct {
	// Raw preserves the original joined input line.
	Raw string
	// IsSlashCommand marks slash-prefixed input that should bypass the engine for now.
	IsSlashCommand bool
	// Command stores the slash command name without the leading slash.
	Command string
	// Body stores the text body for either the slash command tail or a normal prompt.
	Body string
}

// ParseArgs converts CLI arguments into a single prompt or slash-command placeholder.
func ParseArgs(args []string) (ParsedInput, error) {
	raw := strings.TrimSpace(strings.Join(args, " "))
	if raw == "" {
		return ParsedInput{}, fmt.Errorf("missing input: provide a prompt or slash command")
	}

	if IsSlashCommand(raw) {
		command, body := splitSlashCommand(raw)
		return ParsedInput{
			Raw:            raw,
			IsSlashCommand: true,
			Command:        command,
			Body:           body,
		}, nil
	}

	return ParsedInput{
		Raw:  raw,
		Body: raw,
	}, nil
}

// splitSlashCommand extracts the slash command name and any trailing body text.
func splitSlashCommand(input string) (string, string) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(input, "/"))
	if trimmed == "" {
		return "", ""
	}

	parts := strings.SplitN(trimmed, " ", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}

	return parts[0], strings.TrimSpace(parts[1])
}
