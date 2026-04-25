package prompts

import (
	"context"
	"strings"
)

const (
	askUserQuestionToolName = "AskUserQuestion"
	skillToolName           = "Skill"
	discoverSkillsToolName  = "DiscoverSkills"
)

// SessionGuidanceSection provides runtime-specific guidance about tool usage
// and agent selection.
type SessionGuidanceSection struct{}

// Name returns the section identifier.
func (s SessionGuidanceSection) Name() string { return "session_guidance" }

// IsVolatile reports whether this section must be recomputed every turn.
func (s SessionGuidanceSection) IsVolatile() bool { return false }

// Compute generates the session-specific guidance section content.
func (s SessionGuidanceSection) Compute(ctx context.Context) (string, error) {
	data, _ := RuntimeContextFromContext(ctx)

	var items []string
	if data.HasTool(askUserQuestionToolName) {
		items = append(items, "If you do not understand why the user has denied a tool call, use AskUserQuestion to ask them.")
	}
	if data.HasTool("Agent") {
		items = append(items, "Use the Agent tool for complex multi-step tasks or broad codebase exploration, and avoid duplicating work that another agent is already doing.")
	}
	if data.HasTool("Read") && (data.HasTool("Glob") || data.HasTool("Grep")) {
		items = append(items, "For simple directed searches, use Read or the search tools directly instead of delegating to an agent.")
	}
	if data.HasTool("Read") && data.HasTool("Bash") {
		items = append(items, "If you need to inspect a specific file path, prefer Read. If you need shell execution, keep Bash for commands that truly require a shell.")
	}
	if data.HasTool(skillToolName) {
		items = append(items, "Use the Skill tool only for user-invocable skills that are explicitly surfaced. Do not guess built-in CLI commands.")
	}
	if data.HasTool(discoverSkillsToolName) && data.HasTool(skillToolName) {
		items = append(items, "If the surfaced skills do not cover your next step, or you are about to pivot into an unusual workflow, call DiscoverSkills with a specific description of what you are doing.")
	}
	if len(items) == 0 {
		return "", nil
	}

	return strings.Join(append([]string{"# Session-specific guidance"}, prependBullets(items)...), "\n"), nil
}

// prependBullets formats a list of guidance items as Markdown bullets.
func prependBullets(items []string) []string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, " - "+item)
	}
	return lines
}
