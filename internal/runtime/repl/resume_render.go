package repl

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	coresession "github.com/sheepzhao/claude-code-go/internal/core/session"
)

// formatRecentSessionLine renders one recent-session candidate using a stable text-only layout.
func formatRecentSessionLine(summary coresession.Summary) string {
	displayText := strings.TrimSpace(summary.CustomTitle)
	if displayText == "" {
		displayText = strings.TrimSpace(summary.Preview)
	}
	if displayText == "" {
		displayText = "Previous conversation"
	}

	updatedAt := "unknown time"
	if !summary.UpdatedAt.IsZero() {
		updatedAt = summary.UpdatedAt.UTC().Format("2006-01-02 15:04 UTC")
	}

	projectHint := ""
	if summary.ProjectPath != "" {
		projectHint = fmt.Sprintf(" [%s]", filepath.Base(summary.ProjectPath))
	}

	return fmt.Sprintf("- %s | %s%s | %s", updatedAt, displayText, projectHint, summary.ID)
}

// formatIndexedRecentSessionLine renders one numbered picker row for interactive `/resume`.
func formatIndexedRecentSessionLine(index int, summary coresession.Summary) string {
	return fmt.Sprintf("%d. %s", index, strings.TrimPrefix(formatRecentSessionLine(summary), "- "))
}

// renderRecentResumeSessionsOutput formats either the legacy static list or the numbered picker view.
func renderRecentResumeSessionsOutput(projectPath string, summaries []coresession.Summary, interactive bool) string {
	lines := []string{"Recent conversations:"}
	currentProject, otherProjects := partitionSummariesByProject(projectPath, summaries)
	nextIndex := 1
	appendSummary := func(summary coresession.Summary) {
		if interactive {
			lines = append(lines, formatIndexedRecentSessionLine(nextIndex, summary))
			nextIndex++
			return
		}
		lines = append(lines, formatRecentSessionLine(summary))
	}
	for _, summary := range currentProject {
		appendSummary(summary)
	}
	if len(otherProjects) > 0 {
		if len(currentProject) > 0 {
			lines = append(lines, "Other projects:")
		}
		for _, summary := range otherProjects {
			appendSummary(summary)
		}
	}
	if interactive {
		lines = append(lines, resumeSelectionPrompt)
		return strings.Join(lines, "\n")
	}
	if len(currentProject) == 0 && len(otherProjects) > 0 {
		lines = append(lines, resumeCrossProjectUsage)
	} else {
		lines = append(lines, "Use /resume <session-id> <prompt> to continue one.")
		if len(otherProjects) > 0 {
			lines = append(lines, resumeCrossProjectUsage)
		}
	}
	return strings.Join(lines, "\n")
}

// partitionSummariesByProject keeps current-project sessions ahead of cross-project sessions in `/resume` output.
func partitionSummariesByProject(projectPath string, summaries []coresession.Summary) ([]coresession.Summary, []coresession.Summary) {
	var currentProject []coresession.Summary
	var otherProjects []coresession.Summary
	for _, summary := range summaries {
		if isCrossProjectSummary(projectPath, summary) {
			otherProjects = append(otherProjects, summary)
			continue
		}
		currentProject = append(currentProject, summary)
	}
	return currentProject, otherProjects
}

// readResumeSelection reads one picker response and maps it back to one summary.
func (r *Runner) readResumeSelection(summaries []coresession.Summary) (coresession.Summary, bool, error) {
	if r == nil || r.Input == nil {
		return coresession.Summary{}, false, io.EOF
	}

	line, err := bufio.NewReader(r.Input).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return coresession.Summary{}, false, err
	}
	if errors.Is(err, io.EOF) && line == "" {
		return coresession.Summary{}, false, io.EOF
	}

	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return coresession.Summary{}, false, nil
	}

	index, convErr := strconv.Atoi(trimmed)
	if convErr != nil || index < 1 || index > len(summaries) {
		return coresession.Summary{}, false, r.Renderer.RenderLine(resumeSelectionInvalid)
	}
	return summaries[index-1], true, nil
}

// isCrossProjectSummary reports whether one summary belongs to a different project than the active REPL workspace.
func isCrossProjectSummary(projectPath string, summary coresession.Summary) bool {
	if strings.TrimSpace(summary.ProjectPath) == "" {
		return false
	}
	if strings.TrimSpace(projectPath) == "" {
		return true
	}
	return filepath.Clean(summary.ProjectPath) != filepath.Clean(projectPath)
}

// quoteShellPath keeps the cross-project resume hint shell-safe without pulling shell formatting into lower layers.
func quoteShellPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return "."
	}
	return "'" + strings.ReplaceAll(filepath.Clean(path), "'", `'\''`) + "'"
}
