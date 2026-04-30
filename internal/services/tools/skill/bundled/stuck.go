package bundled

import "github.com/sheepzhao/claude-code-go/internal/services/tools/skill"

const stuckPrompt = `# /stuck — diagnose frozen/slow Claude Code sessions

The user thinks another Claude Code session on this machine is frozen, stuck, or very slow. Investigate and post a report to #claude-code-feedback.

## What to look for

Scan for other Claude Code processes (excluding the current one). Process names are typically ` + "`claude`" + ` (installed) or ` + "`cli`" + ` (native dev build).

Signs of a stuck session:
- **High CPU (>=90%) sustained** — likely an infinite loop. Sample twice, 1-2s apart, to confirm it's not a transient spike.
- **Process state D (uninterruptible sleep)** — often an I/O hang.
- **Process state T (stopped)** — user probably hit Ctrl+Z by accident.
- **Process state Z (zombie)** — parent isn't reaping.
- **Very high RSS (>=4GB)** — possible memory leak making the session sluggish.
- **Stuck child process** — a hung git, node, or shell subprocess can freeze the parent.

## Investigation steps

1. List all Claude Code processes (macOS/Linux):
   ` + "```" + `
   ps -axo pid=,pcpu=,rss=,etime=,state=,comm=,command= | grep -E '(claude|cli)' | grep -v grep
   ` + "```" + `

2. For anything suspicious, gather more context:
   - Child processes: ` + "`pgrep -lP <pid>`" + `
   - If high CPU: sample again after 1-2s to confirm it's sustained
   - If a child looks hung, note its full command line with ` + "`ps -p <child_pid> -o command=`" + `
   - Check the session's debug log: ` + "`~/.claude/debug/<session-id>.txt`" + `

3. Consider a stack dump for a truly frozen process:
   - macOS: ` + "`sample <pid> 3`" + ` gives a 3-second native stack sample

## Report

Only post to Slack if you actually found something stuck. If every session looks healthy, tell the user that directly.

If you found a stuck/slow session, post to #claude-code-feedback using the Slack MCP tool. Use a two-message structure:
1. Top-level message: one short line with hostname, version, and symptom
2. Thread reply: full diagnostic dump

If Slack MCP isn't available, format the report as a message the user can copy-paste.

## Notes
- Don't kill or signal any processes — this is diagnostic only.
- If the user gave an argument (e.g., a specific PID or symptom), focus there first.`

func registerStuckSkill() {
	skill.RegisterBundledSkill(skill.BundledSkillDefinition{
		Name:         "stuck",
		Description:  "Investigate frozen/stuck/slow Claude Code sessions on this machine and post a diagnostic report.",
		UserInvocable: true,
		GetPromptForCommand: func(args string) (string, error) {
			prompt := stuckPrompt
			if args != "" {
				prompt += "\n## User-provided context\n\n" + args + "\n"
			}
			return prompt, nil
		},
	})
}
