// Package transcript provides JSONL transcript file path resolution and buffered
// writing for conversation history persistence.
package transcript

import (
	"os"
	"path/filepath"
	"strings"
)

// MaxSanitizedLength is the maximum length for a single filesystem path component
// (directory or file name). Most filesystems limit individual components to 255 bytes.
// We use 200 to leave room for the hash suffix and separator.
const MaxSanitizedLength = 200

// SanitizePath makes a string safe for use as a directory or file name.
// It replaces all non-alphanumeric characters with hyphens.
// For deeply nested paths that would exceed filesystem limits, it truncates
// and appends a djb2 hash suffix for uniqueness.
func SanitizePath(name string) string {
	sanitized := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, name)

	if len(sanitized) <= MaxSanitizedLength {
		return sanitized
	}

	hash := djb2Hash(name)
	return sanitized[:MaxSanitizedLength] + "-" + hash
}

// djb2Hash computes the djb2 hash of a string and returns it as a base-36
// string, matching the TypeScript implementation in sessionStoragePortable.ts.
func djb2Hash(s string) string {
	var hash uint64 = 5381
	for i := 0; i < len(s); i++ {
		hash = ((hash << 5) + hash) + uint64(s[i]) // hash * 33 + c
	}
	return uint64ToBase36(hash)
}

// base36Chars is the character set used for base-36 encoding.
const base36Chars = "0123456789abcdefghijklmnopqrstuvwxyz"

// uint64ToBase36 converts a uint64 to a base-36 string.
func uint64ToBase36(n uint64) string {
	if n == 0 {
		return "0"
	}
	var buf [64]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = base36Chars[n%36]
		n /= 36
	}
	return string(buf[i:])
}

// GetClaudeConfigHomeDir returns the Claude configuration directory.
// It uses the CLAUDE_CONFIG_DIR environment variable if set,
// otherwise falls back to ~/.claude.
func GetClaudeConfigHomeDir() string {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".claude"
	}
	return filepath.Join(home, ".claude")
}

// GetProjectsDir returns the directory where session transcript projects
// are stored (~/.claude/projects).
func GetProjectsDir() string {
	return filepath.Join(GetClaudeConfigHomeDir(), "projects")
}

// GetProjectDir returns the sanitized project directory for a given path.
func GetProjectDir(projectDir string) string {
	return filepath.Join(GetProjectsDir(), SanitizePath(projectDir))
}

// GetTranscriptPath returns the full path to a session's JSONL transcript file.
// The path follows the pattern: ~/.claude/projects/<sanitized-cwd>/<sessionID>.jsonl
func GetTranscriptPath(sessionID, cwd string) string {
	return filepath.Join(GetProjectDir(cwd), sessionID+".jsonl")
}
