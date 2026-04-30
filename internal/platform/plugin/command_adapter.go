package plugin

import (
	"context"
	"fmt"
	"path/filepath"
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

	usage := fmt.Sprintf("/%s [args]", pc.Name)
	if pc.ArgumentHint != "" {
		usage = fmt.Sprintf("/%s %s", pc.Name, pc.ArgumentHint)
	}

	return command.Metadata{
		Name:        pc.Name,
		Description: desc,
		Usage:       usage,
		Hidden:      !pc.UserInvocable,
	}
}

// Execute returns the command's raw markdown content as text output.  This is
// a minimal passthrough that makes plugin commands callable without requiring
// the full engine query infrastructure.  Plugin variables and arguments are
// substituted into the content before returning.
func (a *CommandAdapter) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	if a == nil || a.pluginCmd == nil {
		return command.Result{}, fmt.Errorf("command adapter is nil")
	}

	content := a.pluginCmd.RawContent

	if content != "" {
		// 1. Plugin variable substitution (${CLAUDE_PLUGIN_ROOT}, ${CLAUDE_PLUGIN_DATA}).
		content = SubstitutePluginVariables(content, a.pluginCmd.PluginPath, a.pluginCmd.PluginSource)

		// 2. Skill directory substitution (${CLAUDE_SKILL_DIR}) for skill commands.
		if a.pluginCmd.IsSkill {
			skillDir := filepath.Dir(a.pluginCmd.SourcePath)
			content = SubstituteSkillDir(content, skillDir)
		}

		// 3. Argument substitution.
		if len(args.Raw) > 0 {
			// Backward-compatible ${1}, ${2}, ... positional substitution.
			content = substituteSimpleArgs(content, args.Raw)
			// Full argument substitution ($ARGUMENTS, $n, $name).
			// appendIfNoPlaceholder is false because substituteSimpleArgs already
			// handles positional ${n} placeholders.
			content = SubstituteArguments(content, args.Raw, a.pluginCmd.ArgumentNames, false)
		}
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

// ParsedAllowedTools parses the AllowedTools frontmatter field into a slice of
// individual tool names. Returns nil if AllowedTools is empty.
func (a *CommandAdapter) ParsedAllowedTools() []string {
	if a == nil || a.pluginCmd == nil {
		return nil
	}
	return a.pluginCmd.ParsedAllowedTools()
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
