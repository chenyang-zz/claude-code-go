package config

const (
	// GlobalConfigPath identifies the user-scoped Claude Code settings file.
	GlobalConfigPath = "~/.claude/settings.json"
	// ProjectConfigPath identifies the repository-scoped Claude Code settings file.
	ProjectConfigPath = ".claude/settings.json"
	// LocalConfigPath identifies the machine-local project settings file.
	LocalConfigPath = ".claude/settings.local.json"
	// DefaultSessionDBRelativePath identifies the default SQLite location used by the Go host session store.
	DefaultSessionDBRelativePath = ".claude/claude-code-go/sessions.db"
)
