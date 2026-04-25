package file_read

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/transcript"
)

const memoryFreshnessReminderTemplate = "<system-reminder>This memory is %d days old. Memories are point-in-time observations, not live state - claims about code behavior or file:line citations may be outdated. Verify against current code before asserting as fact.</system-reminder>\n"

// memoryFreshnessReminder returns the presentation-only reminder for memory-like files.
func memoryFreshnessReminder(filePath string, modTime time.Time) string {
	if !isMemoryAttachmentPath(filePath) {
		return ""
	}
	return memoryFreshnessNote(modTime)
}

// memoryFreshnessNote renders a stale-memory warning when the file is older than one day.
func memoryFreshnessNote(modTime time.Time) string {
	if modTime.IsZero() {
		return ""
	}
	days := int(time.Since(modTime).Hours() / 24)
	if days <= 1 {
		return ""
	}
	return fmt.Sprintf(memoryFreshnessReminderTemplate, days)
}

// isMemoryAttachmentPath reports whether a path points at a session or project memory file.
func isMemoryAttachmentPath(filePath string) bool {
	cleanedPath := filepath.ToSlash(filepath.Clean(filePath))
	configHome := filepath.ToSlash(filepath.Clean(transcript.GetClaudeConfigHomeDir()))
	if cleanedPath == configHome {
		return false
	}
	if !strings.HasPrefix(cleanedPath, configHome+"/") {
		return false
	}

	relativePath := strings.TrimPrefix(cleanedPath, configHome+"/")
	return strings.HasPrefix(relativePath, "session-memory/") ||
		(strings.HasPrefix(relativePath, "projects/") && strings.Contains(relativePath, "/memory/"))
}
