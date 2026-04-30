package bundled

import (
	"os"

	"github.com/sheepzhao/claude-code-go/internal/services/tools/skill"
)

func registerDebugSkill() {
	skill.RegisterBundledSkill(skill.BundledSkillDefinition{
		Name:                "debug",
		Description:         "Enable debug logging for this session and help diagnose issues with Claude Code.",
		AllowedTools:        []string{"Read", "Grep", "Glob"},
		ArgumentHint:        "[issue description]",
		DisableModelInvocation: true,
		UserInvocable:        true,
		GetPromptForCommand: func(args string) (string, error) {
			debugLogPath := os.ExpandEnv("$HOME/.claude/debug")
			userSettingsPath := os.ExpandEnv("$HOME/.claude/settings.json")
			projectSettingsPath := ".claude/settings.json"
			localSettingsPath := ".claude/settings.local.json"

			issueDesc := args
			if issueDesc == "" {
				issueDesc = "The user did not describe a specific issue. Read the debug log and summarize any errors, warnings, or notable issues."
			}

			prompt := `# Debug Skill

Help the user debug an issue they're encountering in this current Claude Code session.

## Session Debug Log

The debug log directory is at: ` + "`" + debugLogPath + "`" + `

Use Read and Grep to inspect debug logs. Look for [ERROR] and [WARN] lines.

## Issue Description

` + issueDesc + `

## Settings

Remember that settings are in:
* user - ` + "`" + userSettingsPath + "`" + `
* project - ` + "`" + projectSettingsPath + "`" + `
* local - ` + "`" + localSettingsPath + "`" + `

## Instructions

1. Review the user's issue description
2. Look for [ERROR] and [WARN] entries, stack traces, and failure patterns in the debug log
3. Consider launching the claude-code-guide subagent to understand relevant Claude Code features
4. Explain what you found in plain language
5. Suggest concrete fixes or next steps
`
			return prompt, nil
		},
	})
}
