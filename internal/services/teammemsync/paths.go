package teammemsync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
	"github.com/sheepzhao/claude-code-go/internal/services/extractmemories"
)

// PathTraversalError is returned when a path validation detects a traversal
// or injection attempt.
type PathTraversalError struct {
	Message string
}

func (e *PathTraversalError) Error() string {
	return e.Message
}

// FlagTeamMemorySync is the feature flag key for team memory sync.
// Controlled by the CLAUDE_FEATURE_TEAM_MEMORY_SYNC env var.
const FlagTeamMemorySync = "TEAM_MEMORY_SYNC"

// IsTeamMemoryEnabled reports whether team memory features are enabled.
// Requires auto memory to be enabled AND the team memory feature flag.
// This keeps all team-memory consumers consistent when auto memory is disabled.
func IsTeamMemoryEnabled() bool {
	if !extractmemories.IsAutoMemoryEnabled() {
		return false
	}
	return featureflag.IsEnabled(FlagTeamMemorySync)
}

// GetTeamMemPath returns the team memory directory path for the given project root.
// Shape: <memoryBase>/projects/<sanitized-root>/memory/team/
// The trailing separator is guaranteed.
func GetTeamMemPath(projectRoot string) string {
	return filepath.Join(extractmemories.GetAutoMemPath(projectRoot), "team") + string(filepath.Separator)
}

// GetTeamMemEntrypoint returns the team memory index file path.
// Shape: <memoryBase>/projects/<sanitized-root>/memory/team/MEMORY.md
func GetTeamMemEntrypoint(projectRoot string) string {
	return filepath.Join(extractmemories.GetAutoMemPath(projectRoot), "team", "MEMORY.md")
}

// IsTeamMemPath checks whether a file path is within the team memory directory.
// Uses path resolution to eliminate .. segments. Does NOT resolve symlinks.
// For write validation use ValidateTeamMemKey which includes symlink resolution.
func IsTeamMemPath(filePath, projectRoot string) bool {
	resolvedPath := filepath.Clean(filePath)
	teamDir := strings.TrimSuffix(GetTeamMemPath(projectRoot), string(filepath.Separator))
	return strings.HasPrefix(resolvedPath, teamDir+string(filepath.Separator)) || resolvedPath == teamDir
}

// sanitizePathKey validates a relative path key against injection vectors.
// Returns nil if the key passes all checks, or a PathTraversalError otherwise.
//
// Defense layers implemented:
//  1. Null byte rejection (can truncate paths in C-based syscalls)
//  2. URL-encoded traversal detection
//  3. Unicode NFKC normalization attack defense (fullwidth chars)
//  4. Backslash rejection (Windows path separator traversal)
//  5. Absolute path rejection
func sanitizePathKey(key string) error {
	// Layer 1: reject null bytes.
	if strings.ContainsRune(key, '\x00') {
		return &PathTraversalError{fmt.Sprintf("null byte in path key: %q", key)}
	}

	// Layer 2: detect URL-encoded traversals.
	if decoded, err := urlDecodeIfPossible(key); err == nil && decoded != key {
		if strings.Contains(decoded, "..") || strings.Contains(decoded, "/") {
			return &PathTraversalError{fmt.Sprintf("URL-encoded traversal in path key: %q", key)}
		}
	}

	// Layer 3: Unicode NFKC normalization defense.
	if containsFullwidthTraversal(key) {
		return &PathTraversalError{fmt.Sprintf("unicode-normalized traversal in path key: %q", key)}
	}

	// Layer 4: reject backslashes.
	if strings.Contains(key, "\\") {
		return &PathTraversalError{fmt.Sprintf("backslash in path key: %q", key)}
	}

	// Layer 5: reject absolute paths.
	if strings.HasPrefix(key, "/") {
		return &PathTraversalError{fmt.Sprintf("absolute path key: %q", key)}
	}

	return nil
}

// containsFullwidthTraversal checks for fullwidth Unicode characters that could
// normalize to path traversal sequences under NFKC.
func containsFullwidthTraversal(key string) bool {
	for _, r := range key {
		if r == '．' || r == '／' || r == '＼' || r == '\x00' {
			return true
		}
		if r == '․' || r == '‥' || r == '∕' {
			return true
		}
	}
	return false
}

// urlDecodeIfPossible attempts to URL-decode the given string.
// If the string contains invalid percent-encoding, returns the original string.
func urlDecodeIfPossible(s string) (string, error) {
	if !strings.Contains(s, "%") {
		return s, nil
	}
	var builder strings.Builder
	builder.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == '%' && i+2 < len(s) {
			hi, ok1 := unhex(s[i+1])
			lo, ok2 := unhex(s[i+2])
			if ok1 && ok2 {
				builder.WriteByte(byte(hi<<4 | lo))
				i += 3
				continue
			}
		}
		builder.WriteByte(s[i])
		i++
	}
	return builder.String(), nil
}

func unhex(c byte) (byte, bool) {
	switch {
	case '0' <= c && c <= '9':
		return c - '0', true
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10, true
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

// ValidateTeamMemKey validates a relative path key from the server against the
// team memory directory. Performs sanitization, string-level containment check,
// and symlink-resolved containment verification.
// Returns the resolved absolute path or a PathTraversalError.
func ValidateTeamMemKey(relativeKey, projectRoot string) (string, error) {
	if err := sanitizePathKey(relativeKey); err != nil {
		return "", err
	}

	teamDir := GetTeamMemPath(projectRoot)
	fullPath := filepath.Join(teamDir, relativeKey)

	// String-level containment check.
	resolvedPath := filepath.Clean(fullPath)
	teamDirClean := strings.TrimSuffix(teamDir, string(filepath.Separator))
	if !strings.HasPrefix(resolvedPath, teamDirClean+string(filepath.Separator)) &&
		resolvedPath != teamDirClean {
		return "", &PathTraversalError{fmt.Sprintf("key escapes team memory directory: %q", relativeKey)}
	}

	// Symlink-resolved containment verification.
	realPath, err := realpathDeepestExisting(resolvedPath)
	if err != nil {
		return "", err
	}
	if !isRealPathWithinTeamDir(realPath, projectRoot) {
		return "", &PathTraversalError{fmt.Sprintf("key escapes team memory directory via symlink: %q", relativeKey)}
	}

	return resolvedPath, nil
}

// realpathDeepestExisting resolves symlinks for the deepest existing ancestor
// of a path. The target file may not exist yet, so we walk up until a real
// existing ancestor is found, then rejoin the non-existing tail.
func realpathDeepestExisting(absolutePath string) (string, error) {
	var tail []string
	current := absolutePath

	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			if len(tail) == 0 {
				return resolved, nil
			}
			for i := len(tail) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, tail[i])
			}
			return resolved, nil
		}

		if !os.IsNotExist(err) {
			return "", &PathTraversalError{fmt.Sprintf("cannot verify path containment: %v", err)}
		}

		// Check for dangling symlink before walking up.
		if info, lstatErr := os.Lstat(current); lstatErr == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return "", &PathTraversalError{fmt.Sprintf("dangling symlink detected: %q", current)}
			}
		}

		parent := filepath.Dir(current)
		if parent == current {
			return absolutePath, nil
		}
		tail = append(tail, filepath.Base(current))
		current = parent
	}
}

// isRealPathWithinTeamDir checks whether a real (symlink-resolved) path is
// within the real team memory directory.
func isRealPathWithinTeamDir(realCandidate, projectRoot string) bool {
	teamDirRaw := GetTeamMemPath(projectRoot)
	teamDir := strings.TrimSuffix(teamDirRaw, string(filepath.Separator))

	realTeamDir, err := filepath.EvalSymlinks(teamDir)
	if err != nil {
		if os.IsNotExist(err) {
			return true
		}
		return false
	}

	if realCandidate == realTeamDir {
		return true
	}
	return strings.HasPrefix(realCandidate, realTeamDir+string(filepath.Separator))
}
