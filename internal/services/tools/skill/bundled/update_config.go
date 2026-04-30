package bundled

import "github.com/sheepzhao/claude-code-go/internal/services/tools/skill"

const updateConfigPrompt = `# Update Config Skill

Modify Claude Code configuration by updating settings.json files.

## When Hooks Are Required (Not Memory)

If the user wants something to happen automatically in response to an EVENT, they need a **hook** configured in settings.json. Memory/preferences cannot trigger automated actions.

**These require hooks:**
- "Before compacting, ask me what to preserve" → PreCompact hook
- "After writing files, run prettier" → PostToolUse hook with Write|Edit matcher
- "When I run bash commands, log them" → PreToolUse hook with Bash matcher
- "Always run tests after code changes" → PostToolUse hook

**Hook events:** PreToolUse, PostToolUse, PreCompact, PostCompact, Stop, Notification, SessionStart

## CRITICAL: Read Before Write

**Always read the existing settings file before making changes.** Merge new settings with existing ones - never replace the entire file.

## CRITICAL: Use AskUserQuestion for Ambiguity

When the user's request is ambiguous, use AskUserQuestion to clarify:
- Which settings file to modify (user/project/local)
- Whether to add to existing arrays or replace them
- Specific values when multiple options exist

## Decision: Config Tool vs Direct Edit

**Use the Config tool** for these simple settings:
- theme, editorMode, verbose, model
- language, alwaysThinkingEnabled
- permissions.defaultMode

**Edit settings.json directly** for:
- Hooks (PreToolUse, PostToolUse, etc.)
- Complex permission rules (allow/deny arrays)
- Environment variables
- MCP server configuration
- Plugin configuration

## Workflow

1. **Clarify intent** - Ask if the request is ambiguous
2. **Read existing file** - Use Read tool on the target settings file
3. **Merge carefully** - Preserve existing settings, especially arrays
4. **Edit file** - Use Edit tool (if file doesn't exist, ask user to create it first)
5. **Confirm** - Tell user what was changed

## Settings File Locations

| File | Scope | Git | Use For |
|------|-------|-----|---------|
| ~/.claude/settings.json | Global | N/A | Personal preferences for all projects |
| .claude/settings.json | Project | Commit | Team-wide hooks, permissions, plugins |
| .claude/settings.local.json | Project | Gitignore | Personal overrides for this project |

Settings load in order: user → project → local (later overrides earlier).

### Permissions
` + "```json" + `
{
  "permissions": {
    "allow": ["Bash(npm:*)", "Edit(.claude)", "Read"],
    "deny": ["Bash(rm -rf:*)"],
    "ask": ["Write(/etc/*)"],
    "defaultMode": "default",
    "additionalDirectories": ["/extra/dir"]
  }
}
` + "```" + `

**Permission Rule Syntax:**
- Exact match: "Bash(npm run test)"
- Prefix wildcard: "Bash(git:*)" - matches git status, git commit, etc.
- Tool only: "Read" - allows all Read operations

### Environment Variables
` + "```json" + `
{ "env": { "DEBUG": "true", "MY_API_KEY": "value" } }
` + "```" + `

### Model & Agent
` + "```json" + `
{ "model": "sonnet", "agent": "agent-name", "alwaysThinkingEnabled": true }
` + "```" + `

### Attribution (Commits & PRs)
` + "```json" + `
{ "attribution": { "commit": "Custom commit trailer text", "pr": "Custom PR description text" } }
` + "```" + `

### MCP Server Management
` + "```json" + `
{ "enableAllProjectMcpServers": true, "enabledMcpjsonServers": ["server1"], "disabledMcpjsonServers": ["blocked-server"] }
` + "```" + `

### Plugins
` + "```json" + `
{ "enabledPlugins": { "formatter@anthropic-tools": true } }
` + "```" + `

### Other Settings
- language: Preferred response language (e.g., "japanese")
- cleanupPeriodDays: Days to keep transcripts (default: 30; 0 disables persistence)
- respectGitignore: Whether to respect .gitignore (default: true)

## Hooks Configuration

### Hook Structure
` + "```json" + `
{
  "hooks": {
    "EVENT_NAME": [{
      "matcher": "ToolName|OtherTool",
      "hooks": [{
        "type": "command",
        "command": "your-command-here",
        "timeout": 60,
        "statusMessage": "Running..."
      }]
    }]
  }
}
` + "```" + `

### Hook Events

| Event | Matcher | Purpose |
|-------|---------|---------|
| PermissionRequest | Tool name | Run before permission prompt |
| PreToolUse | Tool name | Run before tool, can block |
| PostToolUse | Tool name | Run after successful tool |
| PostToolUseFailure | Tool name | Run after tool fails |
| Notification | Notification type | Run on notifications |
| Stop | - | Run when Claude stops |
| PreCompact | "manual"/"auto" | Before compaction |
| PostCompact | "manual"/"auto" | After compaction |
| UserPromptSubmit | - | When user submits |
| SessionStart | - | When session starts |

### Hook Types
1. **Command Hook** - Runs a shell command
2. **Prompt Hook** - Evaluates a condition with LLM
3. **Agent Hook** - Runs an agent with tools

### Hook Input (stdin JSON)
` + "```json" + `
{
  "session_id": "abc123",
  "tool_name": "Write",
  "tool_input": { "file_path": "/path/to/file.txt", "content": "..." },
  "tool_response": { "success": true }
}
` + "```" + `

### Hook JSON Output
` + "```json" + `
{
  "systemMessage": "Warning shown to user in UI",
  "continue": false,
  "stopReason": "Message shown when blocking",
  "decision": "block",
  "hookSpecificOutput": {
    "hookEventName": "PostToolUse",
    "additionalContext": "Context injected back to model"
  }
}
` + "```" + `

### Common Patterns

**Auto-format after writes:**
` + "```json" + `
{"hooks": {"PostToolUse": [{"matcher": "Write|Edit", "hooks": [{"type": "command", "command": "jq -r '.tool_response.filePath // .tool_input.file_path' | { read -r f; prettier --write \"$f\"; } 2>/dev/null || true"}]}]}}
` + "```" + `

**Log all bash commands:**
` + "```json" + `
{"hooks": {"PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "jq -r '.tool_input.command' >> ~/.claude/bash-log.txt"}]}]}}
` + "```" + `

## Merging Arrays (Important!)

When adding to permission arrays or hook arrays, **merge with existing**, don't replace. Always preserve existing settings.

## Common Mistakes to Avoid

1. Replacing instead of merging - Always preserve existing settings
2. Wrong file - Ask user if scope is unclear
3. Invalid JSON - Validate syntax after changes
4. Forgetting to read first - Always read before write

## Troubleshooting Hooks

If a hook isn't running:
1. Check the settings file - Read ~/.claude/settings.json or .claude/settings.json
2. Verify JSON syntax - Invalid JSON silently fails
3. Check the matcher - Does it match the tool name?
4. Check hook type - Is it "command", "prompt", or "agent"?
5. Test the command - Run the hook command manually
6. Use --debug - Run claude --debug to see hook execution logs`

func registerUpdateConfigSkill() {
	skill.RegisterBundledSkill(skill.BundledSkillDefinition{
		Name:         "update-config",
		Description:  "Use this skill to configure the Claude Code harness via settings.json. Automated behaviors (\"from now on when X\", \"each time X\", \"whenever X\", \"before/after X\") require hooks configured in settings.json. Also use for: permissions, env vars, hook troubleshooting, or any changes to settings.json files.",
		AllowedTools: []string{"Read"},
		UserInvocable: true,
		GetPromptForCommand: func(args string) (string, error) {
			prompt := updateConfigPrompt
			if args != "" {
				prompt += "\n\n## User Request\n\n" + args
			}
			return prompt, nil
		},
	})
}
