package settingssync

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/platform/config"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// maxFileSizeBytes is the maximum file size for sync entries (500 KB).
	maxFileSizeBytes = 500 * 1024
)

// DeriveProjectID computes a project-level identifier from the git remote URL
// of the current working directory. Returns an empty string when the directory
// is not inside a git repository or the remote cannot be resolved.
//
// The implementation mirrors the TS getRepoRemoteHash() logic:
//  1. Runs `git remote get-url origin` to obtain the remote URL.
//  2. If the remote URL contains a GitHub SSH or HTTPS prefix, extracts the
//     org/repo part and returns it as the project identifier.
//  3. Otherwise falls back to the full remote URL.
func DeriveProjectID(cwd string) string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	remote := strings.TrimSpace(string(out))
	if remote == "" {
		return ""
	}
	return sanitizeRemoteURL(remote)
}

// sanitizeRemoteURL extracts an org/repo project identifier from common
// git remote URL formats (GitHub SSH/HTTPS).
func sanitizeRemoteURL(raw string) string {
	// GitHub SSH: git@github.com:org/repo.git
	if after, ok := strings.CutPrefix(raw, "git@github.com:"); ok {
		return strings.TrimSuffix(after, ".git")
	}
	// GitHub HTTPS: https://github.com/org/repo.git
	if after, ok := strings.CutPrefix(raw, "https://github.com/"); ok {
		return strings.TrimSuffix(after, ".git")
	}
	// Unknown format — return raw.
	return raw
}

// TryReadFile attempts to read a file for sync, enforcing a size limit and
// skipping empty/whitespace-only content. Returns nil when the file does not
// exist, exceeds the size limit, or is empty.
func TryReadFile(path string) []byte {
	if path == "" {
		return nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil
	}

	if info.Size() > maxFileSizeBytes {
		logger.DebugCF("settingssync", "file too large for sync", map[string]any{
			"path":        path,
			"size_bytes":  info.Size(),
			"max_bytes":   maxFileSizeBytes,
		})
		return nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	// Skip empty or whitespace-only content.
	if len(content) == 0 || isWhitespaceOnly(content) {
		return nil
	}

	return content
}

func isWhitespaceOnly(b []byte) bool {
	for _, c := range b {
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			return false
		}
	}
	return true
}

// BuildEntriesFromLocalFiles reads the four standard settings sync files from
// the local filesystem and returns a flat key-value map suitable for upload.
// projectID may be empty when the current directory is not a git repo; in that
// case project-scoped files are omitted.
func BuildEntriesFromLocalFiles(cwd, configHome, projectID string) map[string]string {
	entries := make(map[string]string)

	// 1. Global user settings: ~/.claude/settings.json
	userSettingsPath := resolveHomePath(config.GlobalConfigPath, configHome)
	content := TryReadFile(userSettingsPath)
	if content != nil {
		entries[KeyUserSettings] = string(content)
	}

	// 2. Global user memory: ~/.claude/CLAUDE.md
	userMemoryPath := filepath.Join(configHome, "CLAUDE.md")
	content = TryReadFile(userMemoryPath)
	if content != nil {
		entries[KeyUserMemory] = string(content)
	}

	// Project-scoped files require a project ID.
	if projectID != "" && cwd != "" {
		// 3. Project local settings: <cwd>/.claude/settings.local.json
		localSettingsPath := filepath.Join(cwd, config.LocalConfigPath)
		content = TryReadFile(localSettingsPath)
		if content != nil {
			entries[ProjectSettingsKey(projectID)] = string(content)
		}

		// 4. Project local memory: <cwd>/CLAUDE.local.md
		localMemoryPath := filepath.Join(cwd, "CLAUDE.local.md")
		content = TryReadFile(localMemoryPath)
		if content != nil {
			entries[ProjectMemoryKey(projectID)] = string(content)
		}
	}

	return entries
}

// resolveHomePath replaces a leading "~" with the config home directory path.
func resolveHomePath(p, configHome string) string {
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(configHome, p[2:])
	}
	return p
}
