package extractmemories

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// autoMemDirname is the directory name for auto-memory under the project path.
	autoMemDirname = "memory"
	// autoMemEntrypointName is the MEMORY.md index filename.
	autoMemEntrypointName = "MEMORY.md"
)

// memoryBaseDir is the computed base dir for memory storage (memoized).
var (
	memoryBaseDirOnce sync.Once
	memoryBaseDir     string
)

// MemoryHeader holds the parsed metadata from a memory file.
type MemoryHeader struct {
	// Filename is the relative path from the memory directory.
	Filename string
	// FilePath is the absolute path to the memory file.
	FilePath string
	// ModTime is the file modification time.
	ModTime time.Time
	// Description is the "description" field from the frontmatter, or nil.
	Description *string
	// Type is the "type" field from the frontmatter (user/feedback/project/reference).
	Type string
}

// GetMemoryBaseDir returns the base directory for persistent memory storage.
// Resolution order:
//  1. CLAUDE_CODE_REMOTE_MEMORY_DIR env var (explicit override)
//  2. ~/.claude (default config home)
func GetMemoryBaseDir() string {
	memoryBaseDirOnce.Do(func() {
		if dir := os.Getenv("CLAUDE_CODE_REMOTE_MEMORY_DIR"); dir != "" {
			memoryBaseDir = filepath.Clean(dir)
			return
		}
		home, err := os.UserHomeDir()
		if err != nil {
			logger.WarnF("failed to get user home dir, using relative path", map[string]any{"error": err.Error()})
			memoryBaseDir = filepath.Join(".claude")
			return
		}
		memoryBaseDir = filepath.Join(home, ".claude")
	})
	return memoryBaseDir
}

// ResetMemoryBaseDir clears the memoized memory base dir (used in tests).
func ResetMemoryBaseDir() {
	memoryBaseDirOnce = sync.Once{}
}

// GetAutoMemPath returns the auto-memory directory path for the given
// project root. Shape: <memoryBase>/projects/<sanitized-root>/memory/
//
// The trailing separator is guaranteed.
func GetAutoMemPath(projectRoot string) string {
	base := GetMemoryBaseDir()
	projectsDir := filepath.Join(base, "projects")
	sanitized := sanitizePath(projectRoot)
	return filepath.Join(projectsDir, sanitized, autoMemDirname) + string(filepath.Separator)
}

// sanitizePath replaces characters that are unsafe for use in filesystem paths.
func sanitizePath(p string) string {
	// Replace path separators and other unsafe chars with underscores.
	// This is a simple substitution — TS uses a more complex sanitizePath
	// that also handles absolute paths and other edge cases.
	s := filepath.Clean(p)
	s = strings.TrimPrefix(s, string(filepath.Separator))
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
	)
	return replacer.Replace(s)
}

// GetAutoMemEntrypoint returns the MEMORY.md index file path for the given project root.
func GetAutoMemEntrypoint(projectRoot string) string {
	return filepath.Join(GetAutoMemPath(projectRoot), autoMemEntrypointName)
}

// IsAutoMemPath checks whether an absolute path is within the auto-memory
// directory for the given project root.
func IsAutoMemPath(absolutePath string, projectRoot string) bool {
	normalized := filepath.Clean(absolutePath)
	memDir := GetAutoMemPath(projectRoot)
	// Remove trailing separator for the prefix check.
	memDir = strings.TrimSuffix(memDir, string(filepath.Separator))
	return strings.HasPrefix(normalized, memDir+string(filepath.Separator)) || normalized == memDir
}

// IsAutoMemoryEnabled checks whether auto-memory features are enabled.
// Priority chain (first defined wins):
//  1. CLAUDE_CODE_DISABLE_AUTO_MEMORY env var (1/true -> OFF, 0/false -> ON)
//  2. CLAUDE_CODE_SIMPLE (--bare) -> OFF
//  3. Default: enabled
func IsAutoMemoryEnabled() bool {
	if v := os.Getenv("CLAUDE_CODE_DISABLE_AUTO_MEMORY"); v != "" {
		lower := strings.ToLower(v)
		if lower == "1" || lower == "true" {
			return false
		}
		if lower == "0" || lower == "false" {
			return true
		}
	}
	if v := os.Getenv("CLAUDE_CODE_SIMPLE"); v != "" {
		lower := strings.ToLower(v)
		if lower == "1" || lower == "true" {
			return false
		}
	}
	return true
}

// isMarkdownFile checks if a filename has a markdown extension.
func isMarkdownFile(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".md")
}

// ScanMemoryFiles scans a memory directory for .md files (excluding MEMORY.md),
// reads their YAML frontmatter, and returns headers sorted newest-first.
// Capped at 200 entries.
func ScanMemoryFiles(memoryDir string) ([]MemoryHeader, error) {
	entries, err := os.ReadDir(memoryDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var headers []MemoryHeader
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !isMarkdownFile(name) || name == autoMemEntrypointName {
			continue
		}

		absPath := filepath.Join(memoryDir, name)
		info, statErr := entry.Info()
		if statErr != nil {
			continue
		}

		fm, err := parseFrontmatter(absPath)
		if err != nil {
			// Non-fatal: file may be malformed, skip it.
			logger.DebugCF("extractmemories", "failed to parse frontmatter", map[string]any{
				"file":  absPath,
				"error": err.Error(),
			})
			continue
		}

		header := MemoryHeader{
			Filename: name,
			FilePath: absPath,
			ModTime:  info.ModTime(),
			Type:     fm.Type,
		}
		if fm.Description != "" {
			desc := fm.Description
			header.Description = &desc
		}
		headers = append(headers, header)
	}

	// Sort newest-first by modification time.
	sort.Slice(headers, func(i, j int) bool {
		return headers[i].ModTime.After(headers[j].ModTime)
	})

	// Cap at 200 entries.
	const maxEntries = 200
	if len(headers) > maxEntries {
		headers = headers[:maxEntries]
	}

	return headers, nil
}

// memoryFrontmatter holds parsed frontmatter fields from a memory .md file.
type memoryFrontmatter struct {
	Name        string
	Description string
	Type        string
}

// parseFrontmatter reads a markdown file and extracts YAML frontmatter
// delimited by --- lines at the start of the file.
// Only reads the first 40 lines (~4KB) to keep scanning fast.
func parseFrontmatter(filePath string) (memoryFrontmatter, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return memoryFrontmatter{}, err
	}
	content := string(data)
	return parseFrontmatterString(content), nil
}

// parseFrontmatterString extracts frontmatter fields from the given content.
func parseFrontmatterString(content string) memoryFrontmatter {
	lines := strings.Split(content, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return memoryFrontmatter{}
	}

	// Find the closing ---.
	var fmLines []string
	for i := 1; i < len(lines) && i < 40; i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			fmLines = lines[1:i]
			break
		}
	}
	if len(fmLines) == 0 {
		return memoryFrontmatter{}
	}

	var fm memoryFrontmatter
	for _, line := range fmLines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch key {
		case "name":
			fm.Name = value
		case "description":
			fm.Description = value
		case "type":
			fm.Type = value
		}
	}
	return fm
}

// validMemoryTypes is the set of valid memory types.
var validMemoryTypes = map[string]bool{
	"user":      true,
	"feedback":  true,
	"project":   true,
	"reference": true,
}

// FormatMemoryManifest formats memory headers as a text manifest: one line per
// file with [type] filename (timestamp): description.
func FormatMemoryManifest(memories []MemoryHeader) string {
	var lines []string
	for _, m := range memories {
		tag := ""
		if validMemoryTypes[m.Type] {
			tag = "[" + m.Type + "] "
		}
		ts := m.ModTime.Format(time.RFC3339)
		if m.Description != nil {
			lines = append(lines, "- "+tag+m.Filename+" ("+ts+"): "+*m.Description)
		} else {
			lines = append(lines, "- "+tag+m.Filename+" ("+ts+")")
		}
	}
	return strings.Join(lines, "\n")
}

// IsRemoteMode checks whether the session is running in remote mode.
func IsRemoteMode() bool {
	return os.Getenv("CLAUDE_CODE_REMOTE") == "1" || os.Getenv("CLAUDE_CODE_REMOTE") == "true"
}
