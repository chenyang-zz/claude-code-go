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

	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	platformfs "github.com/sheepzhao/claude-code-go/internal/platform/fs"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// Name is the stable registry identifier used by the migrated GrepTool.
	Name = "Grep"
	// defaultCommandName is the ripgrep binary name used by the first migration pass.
	defaultCommandName = "rg"
	// outputModeFilesWithMatches returns the matched file list view.
	outputModeFilesWithMatches = "files_with_matches"
	// outputModeContent returns matching content lines directly from ripgrep.
	outputModeContent = "content"
	// outputModeCount returns per-file match counts from ripgrep.
	outputModeCount = "count"
)

var vcsDirectoriesToExclude = []string{
	".git",
	".svn",
	".hg",
	".bzr",
	".jj",
	".sl",
}

// Tool implements the first-batch GrepTool using the host ripgrep binary.
type Tool struct {
	// fs provides metadata lookups used to validate paths and sort matches by modification time.
	fs platformfs.FileSystem
	// policy performs the minimal read-permission check before invoking ripgrep.
	policy *corepermission.FilesystemPolicy
	// commandName stores the executable used for ripgrep searches.
	commandName string
}

// Input is the typed request payload accepted by the migrated GrepTool.
type Input struct {
	// Pattern stores the regular expression passed to ripgrep.
	Pattern string `json:"pattern"`
	// Path optionally narrows the search root to one file or directory.
	Path string `json:"path,omitempty"`
	// Glob optionally adds one ripgrep --glob filter.
	Glob string `json:"glob,omitempty"`
	// OutputMode selects whether grep returns filenames, matching content, or counts.
	OutputMode string `json:"output_mode,omitempty"`
}

// Output is the structured search result returned in tool metadata.
type Output struct {
	// Mode records which result projection is returned to the caller.
	Mode string `json:"mode,omitempty"`
	// DurationMs records the end-to-end search latency.
	DurationMs int64 `json:"durationMs"`
	// NumFiles reports how many filenames are included in the result payload.
	NumFiles int `json:"numFiles"`
	// Filenames contains the caller-facing relative or absolute paths for matched files.
	Filenames []string `json:"filenames"`
	// Content stores the caller-facing ripgrep content or count lines for non-file modes.
	Content string `json:"content,omitempty"`
	// NumLines reports how many content lines are returned in content mode.
	NumLines int `json:"numLines,omitempty"`
	// NumMatches reports the summed per-file match count in count mode.
	NumMatches int `json:"numMatches,omitempty"`
}

// matchCandidate stores one ripgrep hit together with its modification time for stable sorting.
type matchCandidate struct {
	// absolutePath keeps the canonical path returned by ripgrep and used for stat calls.
	absolutePath string
	// modTime keeps the file modification time used for descending sort order.
	modTime time.Time
}

// NewTool constructs a GrepTool with explicit host dependencies.
func NewTool(fs platformfs.FileSystem, policy *corepermission.FilesystemPolicy) *Tool {
	return &Tool{
		fs:          fs,
		policy:      policy,
		commandName: defaultCommandName,
	}
}

// Name returns the stable registration name for this tool.
func (t *Tool) Name() string {
	return Name
}

// Description returns the summary exposed to callers and tests.
func (t *Tool) Description() string {
	return "Fast content search tool backed by ripgrep that returns matching file paths, content lines, or match counts."
}

// IsReadOnly reports that grep search never mutates external state.
func (t *Tool) IsReadOnly() bool {
	return true
}

// IsConcurrencySafe reports that independent grep searches may run in parallel safely.
func (t *Tool) IsConcurrencySafe() bool {
	return true
}

// Invoke validates input, enforces read permissions, runs ripgrep, and returns sorted matching files.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{}, fmt.Errorf("grep tool: nil receiver")
	}
	if t.fs == nil {
		return coretool.Result{}, fmt.Errorf("grep tool: filesystem is not configured")
	}
	if t.policy == nil {
		return coretool.Result{}, fmt.Errorf("grep tool: permission policy is not configured")
	}

	start := time.Now()

	input, err := coretool.DecodeInput[Input](inputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	searchPath, err := platformfs.ExpandPath(input.Path, call.Context.WorkingDir)
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("grep tool: expand path: %v", err)}, nil
	}

	info, err := t.fs.Stat(searchPath)
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("Path does not exist: %s", inputPathOrWorkingDir(input.Path, searchPath))}, nil
	}

	evaluation := t.policy.CheckReadPermissionForTool(ctx, t.Name(), searchPath, call.Context.WorkingDir)
	if err := evaluation.ToError(corepermission.FilesystemRequest{
		ToolName:   t.Name(),
		Path:       searchPath,
		WorkingDir: call.Context.WorkingDir,
		Access:     corepermission.AccessRead,
	}); err != nil {
		return coretool.Result{}, err
	}

	logger.DebugCF("grep_tool", "starting grep search", map[string]any{
		"pattern":     input.Pattern,
		"search_path": searchPath,
		"glob":        input.Glob,
		"output_mode": normalizeOutputMode(input.OutputMode),
		"working_dir": call.Context.WorkingDir,
	})

	output, err := t.runSearch(ctx, searchRequest{
		pattern:    strings.TrimSpace(input.Pattern),
		glob:       strings.TrimSpace(input.Glob),
		searchPath: searchPath,
		workingDir: call.Context.WorkingDir,
		outputMode: normalizeOutputMode(input.OutputMode),
		start:      start,
	})
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	logger.DebugCF("grep_tool", "grep search finished", map[string]any{
		"pattern":     input.Pattern,
		"search_path": searchPath,
		"glob":        input.Glob,
		"output_mode": output.Mode,
		"num_files":   output.NumFiles,
		"is_dir":      info.IsDir(),
	})

	return coretool.Result{
		Output: renderOutput(output),
		Meta: map[string]any{
			"data": output,
		},
	}, nil
}

// inputSchema declares the GrepTool input contract for the first migration pass.
func inputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"pattern": {
				Type:        coretool.ValueKindString,
				Description: "The regular expression pattern to search for in file contents.",
				Required:    true,
			},
			"path": {
				Type:        coretool.ValueKindString,
				Description: "Optional file or directory to search in.",
			},
			"glob": {
				Type:        coretool.ValueKindString,
				Description: "Optional glob filter passed through to ripgrep.",
			},
			"output_mode": {
				Type:        coretool.ValueKindString,
				Description: `Optional result mode: "files_with_matches" (default), "content", or "count".`,
			},
		},
	}
}

// searchRequest collects the minimal parameters needed to project one grep output mode.
type searchRequest struct {
	// pattern stores the ripgrep pattern after input normalization.
	pattern string
	// glob stores the optional caller-supplied ripgrep glob.
	glob string
	// searchPath stores the absolute file or directory handed to ripgrep.
	searchPath string
	// workingDir stores the caller cwd used for relative path rendering.
	workingDir string
	// outputMode selects the ripgrep flags and result projection.
	outputMode string
	// start stores the invocation start time for duration accounting.
	start time.Time
}

// runSearch dispatches to the output-mode-specific ripgrep path and result shaping.
func (t *Tool) runSearch(ctx context.Context, request searchRequest) (Output, error) {
	switch request.outputMode {
	case outputModeContent:
		lines, err := t.runRipgrep(ctx, request.pattern, request.glob, request.searchPath, request.outputMode)
		if err != nil {
			return Output{}, err
		}
		relativeLines := relativizeRipgrepLines(lines, request.workingDir)
		return Output{
			Mode:       outputModeContent,
			DurationMs: time.Since(request.start).Milliseconds(),
			Content:    strings.Join(relativeLines, "\n"),
			NumLines:   len(relativeLines),
		}, nil
	case outputModeCount:
		lines, err := t.runRipgrep(ctx, request.pattern, request.glob, request.searchPath, request.outputMode)
		if err != nil {
			return Output{}, err
		}
		relativeLines := relativizeRipgrepLines(lines, request.workingDir)
		numFiles, numMatches := summarizeCountLines(relativeLines)
		return Output{
			Mode:       outputModeCount,
			DurationMs: time.Since(request.start).Milliseconds(),
			NumFiles:   numFiles,
			Content:    strings.Join(relativeLines, "\n"),
			NumMatches: numMatches,
		}, nil
	default:
		paths, err := t.runRipgrep(ctx, request.pattern, request.glob, request.searchPath, request.outputMode)
		if err != nil {
			return Output{}, err
		}

		matches, err := t.collectMatches(paths)
		if err != nil {
			return Output{}, err
		}

		sortMatches(matches)

		filenames := make([]string, 0, len(matches))
		for _, match := range matches {
			filenames = append(filenames, platformfs.ToRelativePath(match.absolutePath, request.workingDir))
		}

		return Output{
			Mode:       outputModeFilesWithMatches,
			DurationMs: time.Since(request.start).Milliseconds(),
			NumFiles:   len(filenames),
			Filenames:  filenames,
		}, nil
	}
}

// runRipgrep executes the host ripgrep binary and returns one line per ripgrep output row.
func (t *Tool) runRipgrep(ctx context.Context, pattern string, glob string, searchPath string, outputMode string) ([]string, error) {
	if pattern == "" {
		return nil, fmt.Errorf("grep tool: pattern is required")
	}

	commandName := t.commandName
	if strings.TrimSpace(commandName) == "" {
		commandName = defaultCommandName
	}

	args := []string{"--hidden"}
	switch outputMode {
	case outputModeContent:
		// No extra flags: ripgrep returns matching lines in path:line format.
	case outputModeCount:
		args = append(args, "--count")
	default:
		args = append(args, "--files-with-matches")
	}
	for _, dir := range vcsDirectoriesToExclude {
		args = append(args, "--glob", "!"+dir)
	}
	if glob != "" {
		args = append(args, "--glob", glob)
	}
	if strings.HasPrefix(pattern, "-") {
		args = append(args, "-e", pattern)
	} else {
		args = append(args, pattern)
	}
	args = append(args, searchPath)

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

// parseRipgrepOutput splits ripgrep stdout into normalized absolute paths.
func parseRipgrepOutput(stdout string) []string {
	lines := strings.Split(stdout, "\n")
	paths := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		paths = append(paths, filepath.Clean(trimmed))
	}
	return paths
}

// relativizeRipgrepLines rewrites ripgrep output rows so leading absolute paths become cwd-relative.
func relativizeRipgrepLines(lines []string, workingDir string) []string {
	relativeLines := make([]string, 0, len(lines))
	for _, line := range lines {
		colonIndex := strings.Index(line, ":")
		if colonIndex <= 0 {
			relativeLines = append(relativeLines, line)
			continue
		}

		absolutePath := filepath.Clean(line[:colonIndex])
		relativePath := platformfs.ToRelativePath(absolutePath, workingDir)
		relativeLines = append(relativeLines, relativePath+line[colonIndex:])
	}
	return relativeLines
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

// normalizeOutputMode keeps unknown modes aligned with the default files-with-matches path.
func normalizeOutputMode(outputMode string) string {
	switch strings.TrimSpace(outputMode) {
	case outputModeContent:
		return outputModeContent
	case outputModeCount:
		return outputModeCount
	default:
		return outputModeFilesWithMatches
	}
}

// renderOutput formats the caller-facing result body for the current migration pass.
func renderOutput(output Output) string {
	switch output.Mode {
	case outputModeContent:
		if strings.TrimSpace(output.Content) == "" {
			return "No matches found"
		}
		return output.Content
	case outputModeCount:
		if strings.TrimSpace(output.Content) == "" {
			return "No matches found"
		}
		summary := fmt.Sprintf(
			"\n\nFound %d total %s across %d %s.",
			output.NumMatches,
			pluralize(output.NumMatches, "occurrence", "occurrences"),
			output.NumFiles,
			pluralize(output.NumFiles, "file", "files"),
		)
		return output.Content + summary
	default:
		if output.NumFiles == 0 {
			return "No files found"
		}
		return strings.Join(output.Filenames, "\n")
	}
}

// inputPathOrWorkingDir keeps handled validation errors aligned with the user-provided path.
func inputPathOrWorkingDir(inputPath string, expandedPath string) string {
	if strings.TrimSpace(inputPath) == "" {
		return expandedPath
	}
	return inputPath
}

// pluralize chooses a singular or plural label for small caller-facing summaries.
func pluralize(count int, singular string, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}
