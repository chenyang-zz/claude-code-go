package grep

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	platformfs "github.com/sheepzhao/claude-code-go/internal/platform/fs"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// runSearch dispatches to the output-mode-specific ripgrep path and result shaping.
func (t *Tool) runSearch(ctx context.Context, request searchRequest) (Output, error) {
	switch request.outputMode {
	case outputModeContent:
		lines, err := t.runRipgrep(ctx, request)
		if err != nil {
			return Output{}, err
		}
		pagedLines, appliedLimit, appliedOffset := applyHeadLimit(lines, request.headLimit, request.offset)
		relativeLines := relativizeRipgrepLines(pagedLines, request.workingDir)
		return Output{
			Mode:              outputModeContent,
			DurationMs:        time.Since(request.start).Milliseconds(),
			Content:           strings.Join(relativeLines, "\n"),
			NumLines:          len(relativeLines),
			AppliedLimit:      appliedLimit,
			AppliedOffset:     appliedOffset,
			PaginationSummary: buildPaginationSummary(len(lines), len(relativeLines), appliedLimit, appliedOffset, "results"),
		}, nil
	case outputModeCount:
		lines, err := t.runRipgrep(ctx, request)
		if err != nil {
			return Output{}, err
		}
		pagedLines, appliedLimit, appliedOffset := applyHeadLimit(lines, request.headLimit, request.offset)
		relativeLines := relativizeRipgrepLines(pagedLines, request.workingDir)
		numFiles, numMatches := summarizeCountLines(relativeLines)
		return Output{
			Mode:              outputModeCount,
			DurationMs:        time.Since(request.start).Milliseconds(),
			NumFiles:          numFiles,
			Content:           strings.Join(relativeLines, "\n"),
			NumMatches:        numMatches,
			AppliedLimit:      appliedLimit,
			AppliedOffset:     appliedOffset,
			PaginationSummary: buildPaginationSummary(len(lines), len(relativeLines), appliedLimit, appliedOffset, "count rows"),
		}, nil
	default:
		paths, err := t.runRipgrep(ctx, request)
		if err != nil {
			return Output{}, err
		}

		matches, err := t.collectMatches(paths)
		if err != nil {
			return Output{}, err
		}

		sortMatches(matches)
		pagedMatches, appliedLimit, appliedOffset := applyHeadLimit(matches, request.headLimit, request.offset)

		filenames := make([]string, 0, len(pagedMatches))
		for _, match := range pagedMatches {
			filenames = append(filenames, platformfs.ToRelativePath(match.absolutePath, request.workingDir))
		}

		return Output{
			Mode:              outputModeFilesWithMatches,
			DurationMs:        time.Since(request.start).Milliseconds(),
			NumFiles:          len(filenames),
			Filenames:         filenames,
			AppliedLimit:      appliedLimit,
			AppliedOffset:     appliedOffset,
			PaginationSummary: buildPaginationSummary(len(matches), len(filenames), appliedLimit, appliedOffset, "files"),
		}, nil
	}
}

// runRipgrep executes the host ripgrep binary and returns one line per ripgrep output row.
func (t *Tool) runRipgrep(ctx context.Context, request searchRequest) ([]string, error) {
	if request.pattern == "" {
		return nil, fmt.Errorf("grep tool: pattern is required")
	}

	commandName := t.commandName
	if strings.TrimSpace(commandName) == "" {
		commandName = defaultCommandName
	}

	args := []string{"--hidden", "--max-columns", "500"}
	if request.multiline {
		args = append(args, "-U", "--multiline-dotall")
	}
	if request.caseInsensitive {
		args = append(args, "-i")
	}

	switch request.outputMode {
	case outputModeContent:
		if request.showLineNumbers {
			args = append(args, "-n")
		}
		if request.context != nil {
			args = append(args, "-C", strconv.Itoa(*request.context))
		} else {
			if request.contextBefore != nil {
				args = append(args, "-B", strconv.Itoa(*request.contextBefore))
			}
			if request.contextAfter != nil {
				args = append(args, "-A", strconv.Itoa(*request.contextAfter))
			}
		}
	case outputModeCount:
		args = append(args, "--count")
	default:
		args = append(args, "--files-with-matches")
	}

	for _, dir := range vcsDirectoriesToExclude {
		args = append(args, "--glob", "!"+dir)
	}
	for _, pattern := range splitGlobPatterns(request.glob) {
		args = append(args, "--glob", pattern)
	}
	if request.fileType != "" {
		args = append(args, "--type", request.fileType)
	}
	if strings.HasPrefix(request.pattern, "-") {
		args = append(args, "-e", request.pattern)
	} else {
		args = append(args, request.pattern)
	}
	args = append(args, request.searchPath)

	cmd := exec.CommandContext(ctx, commandName, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	logger.DebugCF("grep_tool", "executing ripgrep", map[string]any{
		"command": commandName,
		"args":    args,
	})

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil, nil
		}

		stderrMessage := strings.TrimSpace(stderr.String())
		if stderrMessage == "" {
			stderrMessage = err.Error()
		}
		return nil, fmt.Errorf("grep tool: %s", stderrMessage)
	}

	return parseRipgrepOutput(stdout.String()), nil
}

// collectMatches resolves file metadata for each ripgrep hit and skips entries that disappeared mid-search.
func (t *Tool) collectMatches(paths []string) ([]matchCandidate, error) {
	matches := make([]matchCandidate, 0, len(paths))
	for _, absolutePath := range paths {
		info, err := t.fs.Stat(absolutePath)
		if err != nil {
			if platformfs.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		matches = append(matches, matchCandidate{
			absolutePath: absolutePath,
			modTime:      info.ModTime(),
		})
	}
	return matches, nil
}

// sortMatches orders matches by most recently modified file first and falls back to path order for ties.
func sortMatches(matches []matchCandidate) {
	sort.Slice(matches, func(i, j int) bool {
		if !matches[i].modTime.Equal(matches[j].modTime) {
			return matches[i].modTime.After(matches[j].modTime)
		}
		return matches[i].absolutePath < matches[j].absolutePath
	})
}

// parseRipgrepOutput splits ripgrep stdout into normalized absolute paths or raw match lines.
func parseRipgrepOutput(stdout string) []string {
	lines := strings.Split(stdout, "\n")
	parsed := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		parsed = append(parsed, trimmed)
	}
	return parsed
}

// relativizeRipgrepLines rewrites ripgrep output rows so leading absolute paths become cwd-relative.
func relativizeRipgrepLines(lines []string, workingDir string) []string {
	relativeLines := make([]string, 0, len(lines))
	for _, line := range lines {
		absolutePath, suffix, ok := splitRipgrepLocationPrefix(line)
		if !ok {
			relativeLines = append(relativeLines, line)
			continue
		}
		relativePath := platformfs.ToRelativePath(absolutePath, workingDir)
		relativeLines = append(relativeLines, relativePath+suffix)
	}
	return relativeLines
}

// splitRipgrepLocationPrefix extracts the leading absolute path from one ripgrep content/count row.
func splitRipgrepLocationPrefix(line string) (string, string, bool) {
	for i := 0; i < len(line)-2; i++ {
		if line[i] != ':' && line[i] != '-' {
			continue
		}
		if line[i+1] < '0' || line[i+1] > '9' {
			continue
		}
		if line[i+2] != ':' && line[i+2] != '-' {
			continue
		}
		return filepath.Clean(line[:i]), line[i:], true
	}

	colonIndex := strings.Index(line, ":")
	if colonIndex <= 0 {
		return "", "", false
	}
	return filepath.Clean(line[:colonIndex]), line[colonIndex:], true
}

// summarizeCountLines aggregates per-file count rows into file and match totals.
func summarizeCountLines(lines []string) (int, int) {
	numFiles := 0
	numMatches := 0
	for _, line := range lines {
		colonIndex := strings.LastIndex(line, ":")
		if colonIndex <= 0 {
			continue
		}

		count, err := strconv.Atoi(strings.TrimSpace(line[colonIndex+1:]))
		if err != nil {
			continue
		}

		numFiles++
		numMatches += count
	}
	return numFiles, numMatches
}
