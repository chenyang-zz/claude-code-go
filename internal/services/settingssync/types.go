// Package settingssync implements the Settings Sync service, which synchronizes
// user settings and memory files across Claude Code environments via the
// Anthropic Claude.ai backend API.
package settingssync

// SyncKey constants mirror the TS SYNC_KEYS object. They define the known
// key patterns used when uploading/downloading entries to/from the backend.
const (
	// KeyUserSettings maps to the global user settings file (~/.claude/settings.json).
	KeyUserSettings = "~/.claude/settings.json"
	// KeyUserMemory maps to the global user memory file (~/.claude/CLAUDE.md).
	KeyUserMemory = "~/.claude/CLAUDE.md"
)

// ProjectSettingsKey returns the remote key for a project-scoped settings file.
func ProjectSettingsKey(projectID string) string {
	return "projects/" + projectID + "/.claude/settings.local.json"
}

// ProjectMemoryKey returns the remote key for a project-scoped memory file.
func ProjectMemoryKey(projectID string) string {
	return "projects/" + projectID + "/CLAUDE.local.md"
}

// UserSyncContent holds the flat key-value entries that are synced.
type UserSyncContent struct {
	Entries map[string]string `json:"entries"`
}

// UserSyncData is the full response from GET /api/claude_code/user_settings.
type UserSyncData struct {
	UserID       string          `json:"userId"`
	Version      int             `json:"version"`
	LastModified string          `json:"lastModified"`
	Checksum     string          `json:"checksum"`
	Content      UserSyncContent `json:"content"`
}

// SettingsSyncFetchResult is the outcome of a fetch attempt.
type SettingsSyncFetchResult struct {
	Success   bool
	Data      *UserSyncData
	IsEmpty   bool // true when the server returns 404 (no data exists yet)
	Error     string
	SkipRetry bool // true for auth errors that should not be retried
}

// SettingsSyncUploadResult is the outcome of an upload attempt.
type SettingsSyncUploadResult struct {
	Success      bool
	Checksum     string
	LastModified string
	Error        string
}
