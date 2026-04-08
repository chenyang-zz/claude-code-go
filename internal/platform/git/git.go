package git

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// Client lists Git worktrees for one repository-root-adjacent workspace.
type Client struct {
	// Command stores the Git executable name or absolute path.
	Command string
}

// NewClient builds the default Git client used by production wiring.
func NewClient() Client {
	return Client{Command: "git"}
}

// ListWorktrees returns all visible worktree paths for cwd and safely falls back to an empty list when Git is unavailable.
func (c Client) ListWorktrees(ctx context.Context, cwd string) ([]string, error) {
	if strings.TrimSpace(cwd) == "" {
		return nil, nil
	}

	command := c.Command
	if strings.TrimSpace(command) == "" {
		command = "git"
	}

	cmd := exec.CommandContext(ctx, command, "worktree", "list", "--porcelain")
	cmd.Dir = cwd
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		logger.DebugCF("git", "failed to enumerate worktrees; falling back to empty list", map[string]any{
			"cwd":         cwd,
			"command":     command,
			"stderr":      strings.TrimSpace(stderr.String()),
			"error":       err.Error(),
			"worktree_ok": false,
		})
		return nil, nil
	}

	paths := parseWorktreePaths(stdout.String(), cwd)
	logger.DebugCF("git", "enumerated worktrees", map[string]any{
		"cwd":            cwd,
		"worktree_count": len(paths),
		"worktree_ok":    true,
	})
	return paths, nil
}

// parseWorktreePaths parses `git worktree list --porcelain` output into a stable list.
func parseWorktreePaths(output string, cwd string) []string {
	lines := strings.Split(output, "\n")
	seen := make(map[string]struct{}, len(lines))
	var paths []string
	for _, line := range lines {
		if !strings.HasPrefix(line, "worktree ") {
			continue
		}
		path := strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
		if path == "" {
			continue
		}
		cleaned := filepath.Clean(path)
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		paths = append(paths, cleaned)
	}

	currentWorktree := detectCurrentWorktree(filepath.Clean(cwd), paths)
	otherWorktrees := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == currentWorktree {
			continue
		}
		otherWorktrees = append(otherWorktrees, path)
	}
	sort.Strings(otherWorktrees)
	if currentWorktree == "" {
		return otherWorktrees
	}
	return append([]string{currentWorktree}, otherWorktrees...)
}

// detectCurrentWorktree returns the enclosing worktree path for cwd when one is present in the parsed list.
func detectCurrentWorktree(cwd string, worktrees []string) string {
	for _, path := range worktrees {
		if pathWithinWorktree(path, cwd) {
			return path
		}
	}
	return ""
}

// pathWithinWorktree reports whether target is equal to root or nested below it.
func pathWithinWorktree(root string, target string) bool {
	cleanRoot := filepath.Clean(root)
	cleanTarget := filepath.Clean(target)
	if cleanRoot == "." || cleanRoot == "" || cleanTarget == "." || cleanTarget == "" {
		return false
	}
	if cleanRoot == cleanTarget {
		return true
	}
	return strings.HasPrefix(cleanTarget, cleanRoot+string(filepath.Separator))
}
