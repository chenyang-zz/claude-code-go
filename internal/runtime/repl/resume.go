package repl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/sheepzhao/claude-code-go/internal/core/conversation"
	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// runResumeCommand restores one persisted session and immediately continues it with the provided prompt tail.
func (r *Runner) runResumeCommand(ctx context.Context, body string, forkSession bool) error {
	if strings.TrimSpace(body) == "" {
		return r.runRecentResumeSelection(ctx)
	}
	if r == nil || r.SessionManager == nil {
		return r.Renderer.RenderLine(resumeNotConfiguredMessage)
	}

	if sessionID, prompt, ok, err := r.tryParseExplicitResume(ctx, body); err != nil {
		return err
	} else if ok {
		return r.resumeSessionWithPrompt(ctx, sessionID, prompt, forkSession)
	}

	return r.searchAndResumeSession(ctx, body)
}

func (r *Runner) resumeSessionWithPrompt(ctx context.Context, sessionID string, prompt string, forkSession bool) error {
	snapshot, err := r.SessionManager.Resume(ctx, sessionID)
	if err != nil {
		if errors.Is(err, coresession.ErrSessionNotFound) {
			return r.Renderer.RenderLine(fmt.Sprintf("Session %s was not found.", sessionID))
		}
		return err
	}

	if forkSession {
		snapshot, err = r.forkSnapshot(ctx, snapshot)
		if err != nil {
			return err
		}
	}

	r.SessionID = snapshot.Session.ID
	if snapshot.Session.ProjectPath != "" {
		r.ProjectPath = snapshot.Session.ProjectPath
	}
	logger.DebugCF("repl", "resumed session from slash command", map[string]any{
		"session_id":    r.SessionID,
		"fork_session":  forkSession,
		"project_path":  r.ProjectPath,
		"message_count": len(snapshot.Session.Messages),
	})
	return r.runPrompt(ctx, conversation.History{Messages: snapshot.Session.Messages}, prompt)
}

func (r *Runner) searchAndResumeSession(ctx context.Context, body string) error {
	query := strings.TrimSpace(body)
	if query == "" {
		return r.Renderer.RenderLine(resumeUsageMessage)
	}

	titleMatches, err := r.SessionManager.FindByCustomTitle(ctx, query, 10)
	if err != nil {
		return err
	}
	if len(titleMatches) == 1 {
		return r.resumeMatchedSummary(ctx, query, titleMatches[0], true)
	}

	summaries, err := r.SessionManager.SearchAllProjects(ctx, query, 10)
	if err != nil {
		return err
	}
	if len(summaries) == 0 {
		return r.Renderer.RenderLine(fmt.Sprintf("Session %s was not found.", query))
	}
	if len(summaries) > 1 {
		lines := []string{
			fmt.Sprintf("Found %d conversations matching %s.", len(summaries), query),
			"Matching conversations:",
		}
		hasCrossProject := false
		for _, summary := range summaries {
			if isCrossProjectSummary(r.ProjectPath, summary) {
				hasCrossProject = true
			}
			lines = append(lines, formatRecentSessionLine(summary))
		}
		lines = append(lines, resumeMultipleMatchesUsage)
		if hasCrossProject {
			lines = append(lines, resumeCrossProjectUsage)
		}
		return r.Renderer.RenderLine(strings.Join(lines, "\n"))
	}

	return r.resumeMatchedSummary(ctx, query, summaries[0], false)
}

// runRecentResumeSelection renders the recent-session picker and optionally accepts one text selection.
func (r *Runner) runRecentResumeSelection(ctx context.Context) error {
	if r == nil || r.SessionManager == nil {
		return r.Renderer.RenderLine(resumeNotConfiguredMessage)
	}

	summaries, err := r.SessionManager.ListRecentAllProjects(ctx, 10)
	if err != nil {
		return err
	}
	if len(summaries) == 0 {
		return r.Renderer.RenderLine(resumeNoSessionsMessage)
	}
	if r.Input == nil {
		return r.Renderer.RenderLine(renderRecentResumeSessionsOutput(r.ProjectPath, summaries, false))
	}

	if err := r.Renderer.RenderLine(renderRecentResumeSessionsOutput(r.ProjectPath, summaries, true)); err != nil {
		return err
	}
	selection, ok, err := r.readResumeSelection(summaries)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	if !ok {
		return r.Renderer.RenderLine(resumeSelectionCancelled)
	}
	return r.resumeMatchedSummary(ctx, selection.ID, selection, false)
}

// runRenameCommand stores one user-assigned title for the current active session.
func (r *Runner) runRenameCommand(ctx context.Context, body string) error {
	if r == nil || r.SessionManager == nil {
		return r.Renderer.RenderLine(renameNotConfiguredMessage)
	}
	title := strings.TrimSpace(body)
	if title == "" {
		return r.Renderer.RenderLine(renameUsageMessage)
	}

	snapshot, err := r.SessionManager.RenameSession(ctx, r.sessionID(), r.ProjectPath, title)
	if err != nil {
		return err
	}
	r.SessionID = snapshot.Session.ID
	if snapshot.Session.ProjectPath != "" {
		r.ProjectPath = snapshot.Session.ProjectPath
	}
	logger.DebugCF("repl", "renamed current session", map[string]any{
		"session_id":   snapshot.Session.ID,
		"project_path": snapshot.Session.ProjectPath,
		"title":        snapshot.Session.CustomTitle,
	})
	return r.Renderer.RenderLine(fmt.Sprintf("Renamed conversation to %q.", snapshot.Session.CustomTitle))
}

// resumeMatchedSummary applies the existing project/worktree restore policy to one selected summary.
func (r *Runner) resumeMatchedSummary(ctx context.Context, query string, summary coresession.Summary, exactTitleMatch bool) error {
	if isCrossProjectSummary(r.ProjectPath, summary) {
		sameRepoWorktree, err := r.isSameRepoWorktree(ctx, summary)
		if err != nil {
			return err
		}
		if sameRepoWorktree {
			r.SessionID = summary.ID
			if summary.ProjectPath != "" {
				r.ProjectPath = summary.ProjectPath
			}
			logger.DebugCF("repl", "selected same-repo worktree session from resume search", map[string]any{
				"session_id":    r.SessionID,
				"project_path":  r.ProjectPath,
				"query":         query,
				"title_match":   exactTitleMatch,
				"same_repo_wt":  true,
				"cross_project": true,
			})
			return r.Renderer.RenderLine(fmt.Sprintf("Resumed conversation %s.", r.SessionID))
		}
		lines := []string{
			fmt.Sprintf("Found conversation %s in another project.", summary.ID),
			formatRecentSessionLine(summary),
			"Run it from that project directory:",
			fmt.Sprintf("  cd %s && cc /resume %s <prompt>", quoteShellPath(summary.ProjectPath), summary.ID),
		}
		return r.Renderer.RenderLine(strings.Join(lines, "\n"))
	}

	r.SessionID = summary.ID
	if summary.ProjectPath != "" {
		r.ProjectPath = summary.ProjectPath
	}
	logger.DebugCF("repl", "selected session from resume search", map[string]any{
		"session_id":   r.SessionID,
		"project_path": r.ProjectPath,
		"query":        query,
		"title_match":  exactTitleMatch,
	})
	return r.Renderer.RenderLine(fmt.Sprintf("Resumed conversation %s.", r.SessionID))
}

func (r *Runner) renderRecentResumeSessions(ctx context.Context) error {
	if r == nil || r.SessionManager == nil {
		return r.Renderer.RenderLine(resumeNotConfiguredMessage)
	}

	summaries, err := r.SessionManager.ListRecentAllProjects(ctx, 10)
	if err != nil {
		return err
	}
	if len(summaries) == 0 {
		return r.Renderer.RenderLine(resumeNoSessionsMessage)
	}
	return r.Renderer.RenderLine(renderRecentResumeSessionsOutput(r.ProjectPath, summaries, false))
}

// parseResumeBody splits the /resume tail into one session identifier and one follow-up prompt.
func parseResumeBody(body string) (string, string, error) {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return "", "", fmt.Errorf(resumeUsageMessage)
	}

	parts := strings.SplitN(trimmed, " ", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf(resumeUsageMessage)
	}

	sessionID := strings.TrimSpace(parts[0])
	prompt := strings.TrimSpace(parts[1])
	if sessionID == "" || prompt == "" {
		return "", "", fmt.Errorf(resumeUsageMessage)
	}
	return sessionID, prompt, nil
}

// tryParseExplicitResume preserves the existing `/resume <session-id> <prompt>` flow when the first token resolves to one saved session.
func (r *Runner) tryParseExplicitResume(ctx context.Context, body string) (string, string, bool, error) {
	sessionID, prompt, err := parseResumeBody(body)
	if err != nil {
		return "", "", false, nil
	}
	if !isLikelySessionID(sessionID) {
		return "", "", false, nil
	}

	if _, err := r.SessionManager.Resume(ctx, sessionID); err != nil {
		if errors.Is(err, coresession.ErrSessionNotFound) {
			return sessionID, prompt, true, nil
		}
		return "", "", false, err
	}
	return sessionID, prompt, true, nil
}

// isLikelySessionID keeps multi-word natural-language search terms out of the explicit `/resume <session-id> <prompt>` path.
func isLikelySessionID(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	if _, err := uuid.Parse(trimmed); err == nil {
		return true
	}
	return strings.Contains(trimmed, "-")
}

// isSameRepoWorktree reports whether a cross-project summary belongs to one of the current repository's worktrees.
func (r *Runner) isSameRepoWorktree(ctx context.Context, summary coresession.Summary) (bool, error) {
	if r == nil || r.WorktreeLister == nil {
		return false, nil
	}
	if strings.TrimSpace(r.ProjectPath) == "" || strings.TrimSpace(summary.ProjectPath) == "" {
		return false, nil
	}

	worktrees, err := r.WorktreeLister.ListWorktrees(ctx, r.ProjectPath)
	if err != nil {
		return false, err
	}
	if len(worktrees) == 0 {
		return false, nil
	}
	for _, worktree := range worktrees {
		if pathMatchesWorktree(worktree, summary.ProjectPath) {
			return true, nil
		}
	}
	return false, nil
}

// pathMatchesWorktree reports whether target equals one worktree path or is nested below it.
func pathMatchesWorktree(worktree string, target string) bool {
	cleanWorktree := strings.TrimSpace(worktree)
	cleanTarget := strings.TrimSpace(target)
	if cleanWorktree == "" || cleanTarget == "" {
		return false
	}
	cleanWorktree = filepath.Clean(cleanWorktree)
	cleanTarget = filepath.Clean(cleanTarget)
	if cleanWorktree == "." || cleanTarget == "." {
		return false
	}
	if cleanWorktree == cleanTarget {
		return true
	}
	return strings.HasPrefix(cleanTarget, cleanWorktree+string(filepath.Separator))
}
