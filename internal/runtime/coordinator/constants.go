package coordinator

// AsyncAgentAllowedTools lists the tools available to async agent workers.
// Corresponds to TS ASYNC_AGENT_ALLOWED_TOOLS in src/constants/tools.ts.
var AsyncAgentAllowedTools = map[string]struct{}{
	"Read":            {},
	"WebSearch":       {},
	"TodoWrite":       {},
	"Grep":            {},
	"WebFetch":        {},
	"Glob":            {},
	"Bash":            {},
	"Edit":            {},
	"Write":           {},
	"NotebookEdit":    {},
	"Skill":           {},
	"SyntheticOutput": {},
	"ToolSearch":      {},
	"EnterWorktree":   {},
	"ExitWorktree":    {},
}

// InternalWorkerTools lists tools that are internal to workers and should be
// excluded from the worker tools summary shown in the coordinator prompt.
// Corresponds to TS INTERNAL_WORKER_TOOLS in src/coordinator/coordinatorMode.ts.
var InternalWorkerTools = map[string]struct{}{
	"TeamCreate":      {},
	"TeamDelete":      {},
	"SendMessage":     {},
	"SyntheticOutput": {},
}

// CoordinatorModeAllowedTools lists the tools available to the coordinator itself.
// Corresponds to TS COORDINATOR_MODE_ALLOWED_TOOLS in src/constants/tools.ts.
var CoordinatorModeAllowedTools = map[string]struct{}{
	"Agent":           {},
	"TaskStop":        {},
	"SendMessage":     {},
	"SyntheticOutput": {},
}

// SimpleModeTools is the reduced tool set used when CLAUDE_CODE_SIMPLE is enabled.
var SimpleModeTools = []string{"Bash", "Read", "Edit"}
