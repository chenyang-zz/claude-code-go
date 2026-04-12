package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// OutputStyleCommand exposes the deprecated /output-style entrypoint as a stable text-only notice.
type OutputStyleCommand struct{}

// Metadata returns the canonical slash descriptor for /output-style.
func (c OutputStyleCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "output-style",
		Description: "Deprecated: use /config to change output style",
		Usage:       "/output-style",
	}
}

// Execute reports the stable deprecation notice used by the source command.
func (c OutputStyleCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	return command.Result{
		Output: "/output-style has been deprecated. Use /config to change your output style, or set it in your settings file. Changes take effect on the next session.",
	}, nil
}
