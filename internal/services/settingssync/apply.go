package settingssync

import (
	"os"
	"path/filepath"

	"github.com/sheepzhao/claude-code-go/internal/platform/config"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// ApplyRemoteEntriesToLocal writes the downloaded remote entries to their
// corresponding local files. It handles the four standard sync keys:
//   - KeyUserSettings → ~/.claude/settings.json
//   - KeyUserMemory → ~/.claude/CLAUDE.md
//   - ProjectSettingsKey → <cwd>/.claude/settings.local.json
//   - ProjectMemoryKey → <cwd>/CLAUDE.local.md
//
// Each entry is validated against the 500 KB size limit before writing.
func ApplyRemoteEntriesToLocal(entries map[string]string, cwd, configHome, projectID string) {
	settingsWritten := false
	memoryWritten := false

	// 1. Global user settings
	if content, ok := entries[KeyUserSettings]; ok {
		path := resolveHomePath(config.GlobalConfigPath, configHome)
		if !exceedsSizeLimit(content, path) {
			if WriteFileForSync(path, content) {
				settingsWritten = true
			}
		}
	}

	// 2. Global user memory
	if content, ok := entries[KeyUserMemory]; ok {
		path := filepath.Join(configHome, "CLAUDE.md")
		if !exceedsSizeLimit(content, path) {
			if WriteFileForSync(path, content) {
				memoryWritten = true
			}
		}
	}

	// 3-4. Project-scoped files (only when a project ID is available)
	if projectID != "" && cwd != "" {
		projectSettingsKey := ProjectSettingsKey(projectID)
		if content, ok := entries[projectSettingsKey]; ok {
			path := filepath.Join(cwd, config.LocalConfigPath)
			if !exceedsSizeLimit(content, path) {
				if WriteFileForSync(path, content) {
					settingsWritten = true
				}
			}
		}

		projectMemoryKey := ProjectMemoryKey(projectID)
		if content, ok := entries[projectMemoryKey]; ok {
			path := filepath.Join(cwd, "CLAUDE.local.md")
			if !exceedsSizeLimit(content, path) {
				if WriteFileForSync(path, content) {
					memoryWritten = true
				}
			}
		}
	}

	if settingsWritten || memoryWritten {
		logger.DebugCF("settingssync", "applied entries to local files", map[string]any{
			"settings_written": settingsWritten,
			"memory_written":   memoryWritten,
		})
	}
}

// WriteFileForSync atomically creates parent directories and writes content
// to the given path. Returns true on success, false on any error.
func WriteFileForSync(path, content string) bool {
	parentDir := filepath.Dir(path)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		logger.WarnCF("settingssync", "failed to create parent directory", map[string]any{
			"path":  parentDir,
			"error": err.Error(),
		})
		return false
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		logger.WarnCF("settingssync", "failed to write file", map[string]any{
			"path":  path,
			"error": err.Error(),
		})
		return false
	}

	return true
}

// exceedsSizeLimit checks whether the UTF-8 content exceeds the maximum file
// size allowed for sync entries (defense-in-depth match with backend limit).
func exceedsSizeLimit(content, path string) bool {
	sizeBytes := len(content)
	if sizeBytes > maxFileSizeBytes {
		logger.DebugCF("settingssync", "entry exceeds size limit", map[string]any{
			"path":       path,
			"size_bytes": sizeBytes,
			"max_bytes":  maxFileSizeBytes,
		})
		return true
	}
	return false
}
