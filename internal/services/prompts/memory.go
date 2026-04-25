package prompts

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MemorySection loads workspace memory instructions from CLAUDE.md files.
type MemorySection struct{}

// Name returns the section identifier.
func (s MemorySection) Name() string { return "memory" }

// IsVolatile reports whether this section must be recomputed every turn.
func (s MemorySection) IsVolatile() bool { return false }

// Compute loads memory instructions from CLAUDE.md files on the current workspace path.
func (s MemorySection) Compute(ctx context.Context) (string, error) {
	data, _ := RuntimeContextFromContext(ctx)
	cwd := strings.TrimSpace(data.WorkingDir)
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return "", nil
		}
	}

	var blocks []string
	for _, candidate := range claudeMemoryCandidates(cwd) {
		content, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		trimmed := strings.TrimSpace(string(content))
		if trimmed == "" {
			continue
		}
		blocks = append(blocks, fmt.Sprintf("## %s\n\n%s", candidate, trimmed))
	}
	if len(blocks) == 0 {
		return "", nil
	}

	return "# Memory\n\nThe following CLAUDE.md files apply to this workspace:\n\n" + strings.Join(blocks, "\n\n"), nil
}

// claudeMemoryCandidates enumerates CLAUDE.md candidates from the workspace path upward to filesystem root.
func claudeMemoryCandidates(workspacePath string) []string {
	cleaned := filepath.Clean(workspacePath)
	candidates := []string{}
	seen := map[string]struct{}{}
	for {
		candidate := filepath.Join(cleaned, "CLAUDE.md")
		if _, ok := seen[candidate]; !ok {
			candidates = append(candidates, candidate)
			seen[candidate] = struct{}{}
		}
		parent := filepath.Dir(cleaned)
		if parent == cleaned {
			break
		}
		cleaned = parent
	}
	return candidates
}
