package sqlite

// Migration describes one schema step that must be applied before the repository can read or write sessions.
type Migration struct {
	// Name identifies the migration in logs and errors.
	Name string
	// SQL stores the schema statement to execute.
	SQL string
}

var sessionMigrations = []Migration{
	{
		Name: "create_sessions_table",
		SQL: `
CREATE TABLE IF NOT EXISTS sessions (
	id TEXT PRIMARY KEY,
	project_path TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL,
	messages_json TEXT NOT NULL
);`,
	},
	{
		Name: "create_sessions_project_path_updated_at_index",
		SQL:  `CREATE INDEX IF NOT EXISTS idx_sessions_project_path_updated_at ON sessions(project_path, updated_at DESC);`,
	},
}
