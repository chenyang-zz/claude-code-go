package repl

import (
	"fmt"
	"strings"
)

const (
	continueUsageMessage = "Continue command requires a prompt: use --continue <prompt>."
	forkUsageMessage     = "Fork session requires --continue or /resume."
)

// ParsedInput carries the minimum REPL parse result needed for one CLI turn.
type ParsedInput struct {
	// Raw preserves the original joined input line.
	Raw string
	// IsSlashCommand marks slash-prefixed input that should bypass the engine for now.
	IsSlashCommand bool
	// ContinueLatest requests an explicit latest-session recovery instead of the implicit restore-or-create flow.
	ContinueLatest bool
	// ForkSession requests that restored history is cloned into a new session before executing the prompt.
	ForkSession bool
	// Command stores the slash command name without the leading slash.
	Command string
	// Body stores the text body for either the slash command tail or a normal prompt.
	Body string
}

// ParseArgs converts CLI arguments into a single prompt or slash-command placeholder.
func ParseArgs(args []string) (ParsedInput, error) {
	flags, tail := consumeFlags(args)
	raw := strings.TrimSpace(strings.Join(tail, " "))
	if flags.ForkSession && !flags.ContinueLatest && !IsSlashCommand(raw) {
		return ParsedInput{}, fmt.Errorf(forkUsageMessage)
	}
	if flags.ContinueLatest && raw == "" {
		return ParsedInput{}, fmt.Errorf(continueUsageMessage)
	}
	if raw == "" {
		return ParsedInput{}, fmt.Errorf("missing input: provide a prompt or slash command")
	}

	if IsSlashCommand(raw) {
		command, body := splitSlashCommand(raw)
		return ParsedInput{
			Raw:            raw,
			IsSlashCommand: true,
			ContinueLatest: flags.ContinueLatest,
			ForkSession:    flags.ForkSession,
			Command:        command,
			Body:           body,
		}, nil
	}

	return ParsedInput{
		Raw:            raw,
		ContinueLatest: flags.ContinueLatest,
		ForkSession:    flags.ForkSession,
		Body:           raw,
	}, nil
}

type inputFlags struct {
	ContinueLatest bool
	ForkSession    bool
}

func consumeFlags(args []string) (inputFlags, []string) {
	var flags inputFlags
	index := 0
	for index < len(args) {
		switch strings.TrimSpace(args[index]) {
		case "--continue":
			flags.ContinueLatest = true
		case "--fork-session":
			flags.ForkSession = true
		default:
			return flags, args[index:]
		}
		index++
	}
	return flags, nil
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
