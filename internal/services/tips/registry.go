package tips

import (
	"os"
	"runtime"
)

// allTips contains the built-in tip definitions migrated from the TS source.
// Only tips with simple isRelevant predicates (no IDE/Marketplace/GrowthBook
// dependencies) are included.
var allTips = []Tip{
	{
		ID:               "new-user-warmup",
		Content:          "Start with small features or bug fixes, tell Claude to propose a plan, and verify its suggested edits",
		CooldownSessions: 3,
		IsRelevant: func() bool {
			return numStartups() < 10
		},
	},
	{
		ID:               "continue",
		Content:          "Run claude --continue or claude --resume to resume a conversation",
		CooldownSessions: 10,
		IsRelevant:       func() bool { return true },
	},
	{
		ID:               "todo-list",
		Content:          "Ask Claude to create a todo list when working on complex tasks to track progress and remain on track",
		CooldownSessions: 20,
		IsRelevant:       func() bool { return true },
	},
	{
		ID:               "enter-to-steer-in-relatime",
		Content:          "Send messages to Claude while it works to steer Claude in real-time",
		CooldownSessions: 20,
		IsRelevant:       func() bool { return true },
	},
	{
		ID:               "shift-tab",
		Content:          "Hit shift-tab to cycle between default mode, auto-accept edit mode, and plan mode",
		CooldownSessions: 10,
		IsRelevant:       func() bool { return true },
	},
	{
		ID:               "image-paste",
		Content:          "Use ctrl+v to paste images from your clipboard",
		CooldownSessions: 20,
		IsRelevant:       func() bool { return true },
	},
	{
		ID:               "theme-command",
		Content:          "Use /theme to change the color theme",
		CooldownSessions: 20,
		IsRelevant:       func() bool { return true },
	},
	{
		ID:               "colorterm-truecolor",
		Content:          "Try setting environment variable COLORTERM=truecolor for richer colors",
		CooldownSessions: 30,
		IsRelevant: func() bool {
			return getEnv("COLORTERM") == ""
		},
	},
	{
		ID:               "web-app",
		Content:          "Run tasks in the cloud while you keep coding locally · clau.de/web",
		CooldownSessions: 15,
		IsRelevant:       func() bool { return true },
	},
	{
		ID:               "mobile-app",
		Content:          "/mobile to use Claude Code from the Claude app on your phone",
		CooldownSessions: 15,
		IsRelevant:       func() bool { return true },
	},
	{
		ID:               "desktop-app",
		Content:          "Run Claude Code locally or remotely using the Claude desktop app: clau.de/desktop",
		CooldownSessions: 15,
		IsRelevant: func() bool {
			return runtime.GOOS != "linux"
		},
	},
	{
		ID:               "feedback-command",
		Content:          "Use /feedback to help us improve!",
		CooldownSessions: 15,
		IsRelevant: func() bool {
			return numStartups() > 5
		},
	},
	{
		ID:               "custom-commands",
		Content:          "Create skills by adding .md files to .claude/skills/ in your project or ~/.claude/skills/ for skills that work in any project",
		CooldownSessions: 15,
		IsRelevant: func() bool {
			return numStartups() > 10
		},
	},
	{
		ID:               "memory-command",
		Content:          "Use /memory to view and manage Claude memory",
		CooldownSessions: 15,
		IsRelevant:       func() bool { return true },
	},
	{
		ID:               "permissions",
		Content:          "Use /permissions to pre-approve and pre-deny bash, edit, and MCP tools",
		CooldownSessions: 10,
		IsRelevant: func() bool {
			return numStartups() > 10
		},
	},
}

// GetRelevantTips returns the subset of tips whose IsRelevant predicate is
// true and whose cooldown has been satisfied.
func GetRelevantTips() []Tip {
	var relevant []Tip
	for _, tip := range allTips {
		if !tip.IsRelevant() {
			continue
		}
		if sessions := GetSessionsSinceLastShown(tip.ID); sessions < tip.CooldownSessions {
			continue
		}
		relevant = append(relevant, tip)
	}
	return relevant
}

// getEnv is a testable shim for os.Getenv.
var getEnv = os.Getenv

// numStartups is a testable shim for the startup counter.
var numStartups = func() int {
	if defaultStore == nil {
		return 0
	}
	return defaultStore.GetNumStartups()
}
