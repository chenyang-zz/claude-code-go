package command

// Args carries the minimum parsed slash-command input needed by command handlers.
type Args struct {
	// Raw stores the original shell-split argument tail passed to the command.
	Raw []string
	// Flags preserves any future structured switches parsed for the command.
	Flags map[string]string
	// RawLine stores the original command body before any command-specific parsing.
	RawLine string
}
