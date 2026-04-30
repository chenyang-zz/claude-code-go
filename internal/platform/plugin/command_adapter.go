package plugin

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// CommandAdapter adapts a plugin PluginCommand to the core command.Command
// interface.  The Execute method returns the command's raw content as text
// output; it does not trigger an engine query (the full prompt execution
// semantic is not yet migrated).
type CommandAdapter struct {
	pluginCmd *PluginCommand
}

// NewCommandAdapter wraps a PluginCommand for registration with a command
// registry.
func NewCommandAdapter(pc *PluginCommand) *CommandAdapter {
	return &CommandAdapter{pluginCmd: pc}
}

// Metadata returns the canonical slash descriptor for this plugin command.
func (a *CommandAdapter) Metadata() command.Metadata {
	if a == nil || a.pluginCmd == nil {
		return command.Metadata{}
	}

	pc := a.pluginCmd
	desc := pc.Description
	if desc == "" {
		desc = pc.WhenToUse
	}
	if desc == "" {
		desc = fmt.Sprintf("Plugin command from %s", pc.PluginName)
	}

	return command.Metadata{
		Name:        pc.Name,
		Description: desc,
		Usage:       fmt.Sprintf("/%s [args]", pc.Name),
		Hidden:      !pc.UserInvocable,
	}
}

// Execute returns the command's raw markdown content as text output.  This is
// a minimal passthrough that makes plugin commands callable without requiring
// the full engine query infrastructure.  Arguments are substituted into the
// content using simple ${arg} replacement when arg names are available.
func (a *CommandAdapter) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	if a == nil || a.pluginCmd == nil {
		return command.Result{}, fmt.Errorf("command adapter is nil")
	}

	content := a.pluginCmd.RawContent

	// Perform simple argument substitution if the command declares argument
	// names in its frontmatter (stored in the AllowedTools field as a
	// comma-separated list in our current extraction).
	if content != "" && len(args.Raw) > 0 {
		content = substituteSimpleArgs(content, args.Raw)
	}

	// If no content is available, return a descriptive fallback.
	if strings.TrimSpace(content) == "" {
		content = fmt.Sprintf("Plugin command /%s from %s\n\n%s",
			a.pluginCmd.Name,
			a.pluginCmd.PluginName,
			a.pluginCmd.Description,
		)
	}

	return command.Result{
		Output: content,
	}, nil
}

// substituteSimpleArgs replaces positional ${1}, ${2}, ... placeholders in
// content with the corresponding argument from args.  This is a minimal
// substitution that aligns with the simplest parameter patterns used by
// plugin commands; full named argument substitution is not yet migrated.
func substituteSimpleArgs(content string, args []string) string {
	for i, arg := range args {
		placeholder := fmt.Sprintf("${%d}", i+1)
		content = strings.ReplaceAll(content, placeholder, arg)
	}
	return content
}
