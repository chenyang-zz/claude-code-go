package coordinator

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

const coordinatorModeEnv = "CLAUDE_CODE_COORDINATOR_MODE"

// IsCoordinatorMode reports whether the current process is running in coordinator mode.
func IsCoordinatorMode() bool {
	value := strings.TrimSpace(os.Getenv(coordinatorModeEnv))
	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// GetCoordinatorSystemPrompt renders the coordinator system prompt with the provided worker tool summary.
func GetCoordinatorSystemPrompt(workerTools string) string {
	summary := strings.TrimSpace(workerTools)
	if summary == "" {
		summary = "the standard tools available in this session"
	}

	return fmt.Sprintf(`You are Claude Code, an AI assistant that coordinates software engineering tasks across multiple workers.

## Your Role

You are a coordinator. Your job is to:
- Help the user achieve their goal
- Direct workers to research, implement, and verify code changes
- Synthesize results and communicate with the user
- Answer directly when possible instead of delegating trivial work

## Worker Guidance

Workers spawned via the Agent tool have access to these tools: %s

Workers are internal collaborators. Never thank them or treat their messages as user-facing conversation. Summarize new information for the user as it arrives.

## Coordination Rules

- Use workers for research, implementation, and verification.
- Keep the user informed about what you launched and what you learned.
- Delegate higher-level work; do not offload trivial file reads or status checks.`, summary)
}

// RenderWorkerToolsSummary turns a runtime tool set into a stable human-readable summary.
func RenderWorkerToolsSummary(toolNames map[string]struct{}) string {
	if len(toolNames) == 0 {
		return ""
	}

	excluded := map[string]struct{}{
		"Agent":           {},
		"SendMessage":     {},
		"TaskStop":        {},
		"TaskCreate":      {},
		"TaskDelete":      {},
		"TeamCreate":      {},
		"TeamDelete":      {},
		"SyntheticOutput": {},
	}

	names := make([]string, 0, len(toolNames))
	for name := range toolNames {
		if _, ok := excluded[name]; ok {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return ""
	}

	sort.Strings(names)
	return strings.Join(names, ", ")
}
