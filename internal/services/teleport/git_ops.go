package teleport

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/platform/shell"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	gitOpsComponent   = "teleport_git_ops"
	defaultGitTimeout = 30 * time.Second
)

// GitOps handles git operations for teleport session management,
// mirroring the git-related functions from src/utils/teleport.tsx.
type GitOps struct {
	executor *shell.Executor
	workDir  string
}

// NewGitOps creates a new GitOps instance with the given shell executor
// and working directory for git commands.
func NewGitOps(executor *shell.Executor, workDir string) *GitOps {
	return &GitOps{
		executor: executor,
		workDir:  workDir,
	}
}

// ValidateGitState checks that the git working directory is clean
// (ignoring untracked files). Returns a TeleportOperationError if there
// are uncommitted changes. Corresponds to validateGitState() in teleport.tsx.
func (g *GitOps) ValidateGitState(ctx context.Context) error {
	result, err := g.executor.Execute(ctx, shell.Request{
		Command:    "git --no-optional-locks status --porcelain -uno",
		WorkingDir: g.workDir,
		Timeout:    defaultGitTimeout,
	})
	if err != nil {
		return NewTeleportOperationError(
			fmt.Sprintf("failed to check git status: %v", err),
			"Error: Failed to check git status.\n",
		)
	}
	if strings.TrimSpace(result.Stdout) != "" {
		return NewTeleportOperationError(
			"Git working directory is not clean. Please commit or stash your changes before using --teleport.",
			"Error: Git working directory is not clean. Please commit or stash your changes before using --teleport.\n",
		)
	}
	logger.DebugCF(gitOpsComponent, "git working directory is clean", nil)
	return nil
}

// CheckOutTeleportedSessionBranch checks out the specified branch for a
// teleported session. If branch is empty, stays on the current branch.
// Returns the current branch name and any error encountered.
// Corresponds to checkOutTeleportedSessionBranch() in teleport.tsx.
func (g *GitOps) CheckOutTeleportedSessionBranch(ctx context.Context, branch string) (branchName string, branchError error) {
	// Log current branch before any operation
	if cb, err := g.getCurrentBranch(ctx); err == nil {
		logger.DebugCF(gitOpsComponent, "current branch before teleport", map[string]any{
			"branch": cb,
		})
	} else {
		logger.DebugCF(gitOpsComponent, "unable to determine current branch", map[string]any{
			"error": err.Error(),
		})
	}

	if branch != "" {
		logger.DebugCF(gitOpsComponent, "switching to branch", map[string]any{
			"branch": branch,
		})

		g.fetchFromOrigin(ctx, branch)
		if err := g.checkoutBranch(ctx, branch); err != nil {
			return g.resolveBranchName(ctx), err
		}

		if nb, err := g.getCurrentBranch(ctx); err == nil {
			logger.DebugCF(gitOpsComponent, "branch after checkout", map[string]any{
				"branch": nb,
			})
		}
	} else {
		logger.DebugCF(gitOpsComponent, "no branch specified, staying on current branch", nil)
	}

	return g.resolveBranchName(ctx), nil
}

// resolveBranchName attempts to get the current git branch name.
// Returns an empty string if the branch cannot be determined.
func (g *GitOps) resolveBranchName(ctx context.Context) string {
	if cb, err := g.getCurrentBranch(ctx); err == nil {
		return cb
	}
	return ""
}

// fetchFromOrigin fetches a specific branch from remote origin.
// If branch is empty, fetches all branches.
// On failure, logs the error but does not return it, matching the TS behaviour
// where fetchFromOrigin only logs errors via logError().
func (g *GitOps) fetchFromOrigin(ctx context.Context, branch string) {
	var cmd string
	if branch != "" {
		cmd = fmt.Sprintf("git fetch origin %s:%s", branch, branch)
	} else {
		cmd = "git fetch origin"
	}

	result, err := g.executor.Execute(ctx, shell.Request{
		Command:    cmd,
		WorkingDir: g.workDir,
		Timeout:    defaultGitTimeout,
	})
	if err != nil {
		logger.ErrorCF(gitOpsComponent, "failed to execute git fetch", map[string]any{
			"error": err.Error(),
		})
		return
	}

	if result.ExitCode != 0 {
		// If fetching a specific branch fails, it might not exist locally yet.
		// Try fetching just the ref without mapping to local branch.
		if branch != "" && strings.Contains(result.Stderr, "refspec") {
			logger.DebugCF(gitOpsComponent, "specific branch fetch failed, trying to fetch ref", map[string]any{
				"branch":      branch,
				"fetch_stderr": result.Stderr,
			})

			refResult, refErr := g.executor.Execute(ctx, shell.Request{
				Command:    fmt.Sprintf("git fetch origin %s", branch),
				WorkingDir: g.workDir,
				Timeout:    defaultGitTimeout,
			})
			if refErr != nil {
				logger.ErrorCF(gitOpsComponent, "failed to execute ref git fetch", map[string]any{
					"error": refErr.Error(),
				})
				return
			}
			if refResult.ExitCode != 0 {
				logger.ErrorCF(gitOpsComponent, "failed to fetch from remote origin", map[string]any{
					"stderr": refResult.Stderr,
				})
			}
		} else {
			logger.ErrorCF(gitOpsComponent, "failed to fetch from remote origin", map[string]any{
				"stderr": result.Stderr,
			})
		}
	}
}

// ensureUpstreamIsSet ensures the current branch has an upstream set.
// If not, checks whether origin/<branchName> exists and sets the upstream.
// On failure, logs but does not return an error, matching the TS behaviour.
func (g *GitOps) ensureUpstreamIsSet(ctx context.Context, branchName string) {
	// Check if upstream is already set
	result, err := g.executor.Execute(ctx, shell.Request{
		Command:    fmt.Sprintf("git rev-parse --abbrev-ref %s@{upstream}", branchName),
		WorkingDir: g.workDir,
		Timeout:    defaultGitTimeout,
	})
	if err == nil && result.ExitCode == 0 {
		logger.DebugCF(gitOpsComponent, "branch already has upstream set", map[string]any{
			"branch": branchName,
		})
		return
	}

	// Check if origin/<branchName> exists
	result, err = g.executor.Execute(ctx, shell.Request{
		Command:    fmt.Sprintf("git rev-parse --verify origin/%s", branchName),
		WorkingDir: g.workDir,
		Timeout:    defaultGitTimeout,
	})
	if err != nil || result.ExitCode != 0 {
		logger.DebugCF(gitOpsComponent, "remote branch does not exist, skipping upstream setup", map[string]any{
			"branch": branchName,
		})
		return
	}

	// Remote branch exists, set upstream
	logger.DebugCF(gitOpsComponent, "setting upstream for branch", map[string]any{
		"branch":   branchName,
		"upstream": fmt.Sprintf("origin/%s", branchName),
	})

	result, err = g.executor.Execute(ctx, shell.Request{
		Command:    fmt.Sprintf("git branch --set-upstream-to origin/%s %s", branchName, branchName),
		WorkingDir: g.workDir,
		Timeout:    defaultGitTimeout,
	})
	if err != nil || result.ExitCode != 0 {
		logger.DebugCF(gitOpsComponent, "failed to set upstream for branch", map[string]any{
			"branch": branchName,
			"stderr": result.Stderr,
		})
	} else {
		logger.DebugCF(gitOpsComponent, "successfully set upstream for branch", map[string]any{
			"branch": branchName,
		})
	}
}

// checkoutBranch attempts to check out a specific branch, with fallbacks
// for remote tracking branches. On success it also ensures upstream is set.
// Corresponds to checkoutBranch() in teleport.tsx.
func (g *GitOps) checkoutBranch(ctx context.Context, branchName string) error {
	// First try to checkout the branch as-is (might be local)
	result, err := g.executor.Execute(ctx, shell.Request{
		Command:    fmt.Sprintf("git checkout %s", branchName),
		WorkingDir: g.workDir,
		Timeout:    defaultGitTimeout,
	})
	if err != nil {
		return NewTeleportOperationError(
			fmt.Sprintf("failed to execute git checkout: %v", err),
			fmt.Sprintf("Error: Failed to checkout branch '%s'.\n", branchName),
		)
	}

	if result.ExitCode == 0 {
		g.ensureUpstreamIsSet(ctx, branchName)
		return nil
	}

	logger.DebugCF(gitOpsComponent, "local checkout failed, trying to checkout from origin", map[string]any{
		"branch": branchName,
		"stderr": result.Stderr,
	})

	// Try to checkout the remote branch and create a local tracking branch
	result, err = g.executor.Execute(ctx, shell.Request{
		Command:    fmt.Sprintf("git checkout -b %s --track origin/%s", branchName, branchName),
		WorkingDir: g.workDir,
		Timeout:    defaultGitTimeout,
	})
	if err == nil && result.ExitCode == 0 {
		g.ensureUpstreamIsSet(ctx, branchName)
		return nil
	}

	logger.DebugCF(gitOpsComponent, "remote checkout with -b failed, trying without -b", map[string]any{
		"branch": branchName,
		"stderr": result.Stderr,
	})

	// Try without -b in case the branch exists but isn't checked out
	result, err = g.executor.Execute(ctx, shell.Request{
		Command:    fmt.Sprintf("git checkout --track origin/%s", branchName),
		WorkingDir: g.workDir,
		Timeout:    defaultGitTimeout,
	})
	if err == nil && result.ExitCode == 0 {
		g.ensureUpstreamIsSet(ctx, branchName)
		return nil
	}

	return NewTeleportOperationError(
		fmt.Sprintf("Failed to checkout branch '%s': %s", branchName, result.Stderr),
		fmt.Sprintf("Error: Failed to checkout branch '%s'.\n", branchName),
	)
}

// getCurrentBranch returns the current git branch name.
// Corresponds to getCurrentBranch() in teleport.tsx.
func (g *GitOps) getCurrentBranch(ctx context.Context) (string, error) {
	result, err := g.executor.Execute(ctx, shell.Request{
		Command:    "git branch --show-current",
		WorkingDir: g.workDir,
		Timeout:    defaultGitTimeout,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	return strings.TrimSpace(result.Stdout), nil
}
