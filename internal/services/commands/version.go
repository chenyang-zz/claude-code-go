package commands

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const unknownVersion = "unknown"

// VersionCommand exposes the minimum text-only /version behavior available in the Go host.
type VersionCommand struct{}

// Metadata returns the canonical slash descriptor for /version.
func (c VersionCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "version",
		Description: "Print the version this session is running (not what autoupdate downloaded)",
		Usage:       "/version",
	}
}

// Execute reports the current binary version derived from Go build metadata when available.
func (c VersionCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	_ = ctx
	_ = args

	version, buildTime := currentBuildVersion()
	logger.DebugCF("commands", "rendered version command output", map[string]any{
		"version":          version,
		"build_time_known": buildTime != "",
	})

	if buildTime != "" {
		return command.Result{
			Output: fmt.Sprintf("%s (built %s)", version, buildTime),
		}, nil
	}

	return command.Result{
		Output: version,
	}, nil
}

// currentBuildVersion reads the current binary version and optional build time from Go build info.
func currentBuildVersion() (string, string) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return unknownVersion, ""
	}

	version := strings.TrimSpace(info.Main.Version)
	if version == "" {
		version = unknownVersion
	}

	var buildTime string
	for _, setting := range info.Settings {
		if setting.Key == "vcs.time" {
			buildTime = strings.TrimSpace(setting.Value)
			break
		}
	}

	return version, buildTime
}
