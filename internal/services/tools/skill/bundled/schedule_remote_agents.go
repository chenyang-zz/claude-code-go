package bundled

import (
	"os"

	"github.com/sheepzhao/claude-code-go/internal/services/policylimits"
	"github.com/sheepzhao/claude-code-go/internal/services/tools/skill"
)

const scheduleRemoteAgentsPrompt = `# Schedule Remote Agents

You are helping the user schedule, update, list, or run **remote** Claude Code agents. These are NOT local cron jobs — each trigger spawns a fully isolated remote session (CCR) in Anthropic's cloud infrastructure on a cron schedule.

## What You Can Do

Use the RemoteTrigger tool:

- {action: "list"} — list all triggers
- {action: "get", trigger_id: "..."} — fetch one trigger
- {action: "create", body: {...}} — create a trigger
- {action: "update", trigger_id: "...", body: {...}} — partial update
- {action: "run", trigger_id: "..."} — run a trigger now

You CANNOT delete triggers. If the user asks to delete, direct them to: https://claude.ai/code/scheduled

## Create body shape

` + "```json" + `
{
  "name": "AGENT_NAME",
  "cron_expression": "CRON_EXPR",
  "enabled": true,
  "job_config": {
    "ccr": {
      "environment_id": "ENVIRONMENT_ID",
      "session_context": {
        "model": "claude-sonnet-4-6",
        "sources": [
          {"git_repository": {"url": "https://github.com/ORG/REPO"}}
        ],
        "allowed_tools": ["Bash", "Read", "Write", "Edit", "Glob", "Grep"]
      },
      "events": [
        {"data": {
          "uuid": "<lowercase v4 uuid>",
          "session_id": "",
          "type": "user",
          "parent_tool_use_id": null,
          "message": {"content": "PROMPT_HERE", "role": "user"}
        }}
      ]
    }
  }
}
` + "```" + `

Generate a fresh lowercase UUID for events[].data.uuid yourself.

## Cron Expression Examples

The user's local timezone is auto-detected. Cron expressions are always in UTC. When the user says a local time, convert it to UTC for the cron expression but confirm: "9am LOCAL = Xam UTC, so the cron would be 0 X * * 1-5."

- 0 9 * * 1-5 — Every weekday at 9am UTC
- 0 */2 * * * — Every 2 hours
- 0 0 * * * — Daily at midnight UTC
- 30 14 * * 1 — Every Monday at 2:30pm UTC
- 0 8 1 * * — First of every month at 8am UTC

Minimum interval is 1 hour. */30 * * * * will be rejected.

## IMPORTANT: Auth Required

This skill requires authentication with a claude.ai account. If you get an auth error, tell the user to run /login first.

## Workflow

### CREATE a new trigger:
1. Understand the goal — Ask what they want the remote agent to do
2. Craft the prompt — Help write an effective agent prompt
3. Set the schedule — Ask when and how often
4. Choose the model — Default to claude-sonnet-4-6
5. Validate connections — Infer what services the agent will need
6. Review and confirm — Show the full configuration before creating
7. Create it — Call RemoteTrigger with action: "create" and show https://claude.ai/code/scheduled/{TRIGGER_ID}

### UPDATE a trigger:
1. List triggers first so they can pick one
2. Ask what they want to change
3. Show current vs proposed value
4. Confirm and update

### LIST triggers:
1. Fetch and display in a readable format

### RUN NOW:
1. List triggers if they haven't specified which one
2. Confirm which trigger
3. Execute and confirm

## Important Notes
- These are REMOTE agents — they run in Anthropic's cloud, not on the user's machine
- Always convert cron to human-readable when displaying
- Default to enabled: true unless user says otherwise
- The prompt is the most important part — the remote agent starts with zero context`

func registerScheduleRemoteAgentsSkill() {
	isEnabled := func() bool {
		if os.Getenv("CLAUDE_CODE_REMOTE_AGENTS") == "" {
			return false
		}
		allowed, _ := policylimits.IsAllowed(policylimits.ActionAllowRemoteSessions)
		return allowed
	}

	skill.RegisterBundledSkill(skill.BundledSkillDefinition{
		Name:         "schedule",
		Description:  "Create, update, list, or run scheduled remote agents (triggers) that execute on a cron schedule.",
		WhenToUse:    "When the user wants to schedule a recurring remote agent, set up automated tasks, create a cron job for Claude Code, or manage their scheduled agents/triggers.",
		UserInvocable: true,
		IsEnabled:     isEnabled,
		AllowedTools:  []string{"RemoteTrigger", "AskUserQuestion"},
		GetPromptForCommand: func(args string) (string, error) {
			prompt := scheduleRemoteAgentsPrompt
			if args != "" {
				prompt += "\n\n## User Request\n\nThe user said: \"" + args + "\"\n\nStart by understanding their intent and working through the appropriate workflow above."
			}
			return prompt, nil
		},
	})
}
