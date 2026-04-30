package shared

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// maxSlugLength is the maximum length for a worktree slug.
	maxSlugLength = 64
	// worktreeDir is the subdirectory under the repo where worktrees are created.
	worktreeDir = ".claude"
	// worktreeSubDir is the sub-subdirectory for worktree roots.
	worktreeSubDir = "worktrees"
)

// validSlugSegment matches the TS VALID_WORKTREE_SLUG_SEGMENT regex.
var validSlugSegment = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// Manager handles git worktree lifecycle operations shared by EnterWorktreeTool
// and ExitWorktreeTool.
type Manager struct {
	gitCommand string
}

// NewManager creates a Manager with the default git command.
func NewManager() *Manager {
	return &Manager{gitCommand: "git"}
}

// ValidateSlug checks that the worktree slug conforms to the naming rules.
func (m *Manager) ValidateSlug(slug string) error {
	if len(slug) > maxSlugLength {
		return fmt.Errorf(
			"invalid worktree name: must be %d characters or fewer (got %d)",
			maxSlugLength, len(slug),
		)
	}
	segments := strings.Split(slug, "/")
	for _, seg := range segments {
		if seg == "." || seg == ".." {
			return fmt.Errorf(
				"invalid worktree name %q: must not contain \".\" or \"..\" path segments",
				slug,
			)
		}
		if !validSlugSegment.MatchString(seg) {
			return fmt.Errorf(
				"invalid worktree name %q: each \"/\"-separated segment must be non-empty and contain only letters, digits, dots, underscores, and dashes",
				slug,
			)
		}
	}
	return nil
}

// FindGitRoot locates the git repository root for the given directory.
func (m *Manager) FindGitRoot(cwd string) (string, error) {
	cmd := exec.Command(m.gitCommand, "rev-parse", "--show-toplevel")
	cmd.Dir = cwd
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("not in a git repository: %v", err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// WorktreeResult holds the outcome of a worktree creation operation.
type WorktreeResult struct {
	Path   string
	Branch string
}

// CreateWorktree creates a new git worktree with the given slug under the repo root.
func (m *Manager) CreateWorktree(cwd string, slug string) (*WorktreeResult, error) {
	if err := m.ValidateSlug(slug); err != nil {
		return nil, err
	}

	gitRoot, err := m.FindGitRoot(cwd)
	if err != nil {
		return nil, err
	}

	// Build the worktree path: .claude/worktrees/<slug>
	worktreePath := filepath.Join(gitRoot, worktreeDir, worktreeSubDir, slug)

	// Check if worktree already exists.
	if _, err := os.Stat(worktreePath); err == nil {
		// Worktree exists — verify it's a registered git worktree.
		branch := m.getWorktreeBranch(worktreePath)
		logger.DebugCF("worktree_manager", "reusing existing worktree", map[string]any{
			"worktree_path": worktreePath,
		})
		return &WorktreeResult{Path: worktreePath, Branch: branch}, nil
	}

	// Create the parent directory.
	parentDir := filepath.Dir(worktreePath)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return nil, fmt.Errorf("create worktree parent directory: %v", err)
	}

	// Generate a branch name from the slug and a random suffix.
	branchName := fmt.Sprintf("claude-code/%s-%s", slug, randomSuffix(6))

	// Run: git worktree add <path> -b <branch>
	cmd := exec.Command(m.gitCommand, "worktree", "add", worktreePath, "-b", branchName)
	cmd.Dir = gitRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf(
			"create worktree failed: %s: %v",
			strings.TrimSpace(stderr.String()), err,
		)
	}

	logger.DebugCF("worktree_manager", "created worktree", map[string]any{
		"worktree_path":   worktreePath,
		"worktree_branch": branchName,
		"git_root":        gitRoot,
	})

	return &WorktreeResult{Path: worktreePath, Branch: branchName}, nil
}

// RemoveWorktree removes a git worktree at the given path.
// When force is true, any uncommitted changes are discarded.
func (m *Manager) RemoveWorktree(worktreePath string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, worktreePath)

	cmd := exec.Command(m.gitCommand, args...)
	// Run from the parent directory of the worktree to ensure git resolves the repo.
	cmd.Dir = filepath.Dir(worktreePath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf(
			"remove worktree failed: %s: %v",
			strings.TrimSpace(stderr.String()), err,
		)
	}

	logger.DebugCF("worktree_manager", "removed worktree", map[string]any{
		"worktree_path": worktreePath,
		"force":         force,
	})
	return nil
}

// CountWorktreeChanges checks for uncommitted files and commits in a worktree.
// Returns nil when state cannot be determined.
func (m *Manager) CountWorktreeChanges(worktreePath string) *ChangeSummary {
	// git status --porcelain
	status := m.runGit(worktreePath, "status", "--porcelain")
	if status == nil {
		return nil
	}
	changedFiles := 0
	for _, line := range strings.Split(status.Stdout, "\n") {
		if strings.TrimSpace(line) != "" {
			changedFiles++
		}
	}

	return &ChangeSummary{
		ChangedFiles: changedFiles,
	}
}

// ChangeSummary holds the result of counting worktree changes.
type ChangeSummary struct {
	ChangedFiles int
}

// getWorktreeBranch retrieves the current branch of a worktree by reading HEAD.
func (m *Manager) getWorktreeBranch(worktreePath string) string {
	result := m.runGit(worktreePath, "rev-parse", "--abbrev-ref", "HEAD")
	if result == nil {
		return ""
	}
	return strings.TrimSpace(result.Stdout)
}

// gitResult holds the stdout and stderr of a git command run.
type gitResult struct {
	Stdout string
	Stderr string
}

// runGit executes a git command in the specified directory and returns the output.
func (m *Manager) runGit(workDir string, args ...string) *gitResult {
	cmd := exec.Command(m.gitCommand, args...)
	cmd.Dir = workDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil
	}
	return &gitResult{Stdout: stdout.String(), Stderr: stderr.String()}
}

// randomSuffix generates a random alphanumeric string of the given length.
func randomSuffix(n int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, n)
	for i := range b {
		b[i] = charset[rng.Intn(len(charset))]
	}
	return string(b)
}
