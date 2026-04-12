package commands

import (
	"context"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const releaseNotesURL = "https://github.com/anthropics/claude-code/blob/main/CHANGELOG.md"

const releaseNotesCommandFallback = "Release notes fetching is not available in Claude Code Go yet. See the full changelog at: " + releaseNotesURL

// ReleaseNotesCommand exposes the minimum text-only /release-notes behavior available before changelog fetch and cache support exists.
type ReleaseNotesCommand struct{}

// Metadata returns the canonical slash descriptor for /release-notes.
func (c ReleaseNotesCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "release-notes",
		Description: "View release notes",
		Usage:       "/release-notes",
	}
}

// Execute reports the stable changelog fallback supported by the current Go host.
func (c ReleaseNotesCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	logger.DebugCF("commands", "rendered release-notes command fallback output", map[string]any{
		"release_notes_cached":  false,
		"release_notes_fetched": false,
	})

	return command.Result{
		Output: releaseNotesCommandFallback,
	}, nil
}
