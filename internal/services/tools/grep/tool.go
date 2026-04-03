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
	// defaultHeadLimit keeps grep output bounded when the caller does not request a page size.
	defaultHeadLimit = 250
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

// Tool implements the migrated GrepTool using the host ripgrep binary.
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
	// Glob optionally adds one or more ripgrep --glob filters.
	Glob string `json:"glob,omitempty"`
	// OutputMode selects whether grep returns filenames, matching content, or counts.
	OutputMode string `json:"output_mode,omitempty"`
	// ContextBefore maps to ripgrep -B in content mode.
	ContextBefore *int `json:"-B,omitempty"`
	// ContextAfter maps to ripgrep -A in content mode.
	ContextAfter *int `json:"-A,omitempty"`
	// ContextShort maps to ripgrep -C in content mode.
	ContextShort *int `json:"-C,omitempty"`
	// Context maps to ripgrep -C in content mode and takes precedence over ContextShort.
	Context *int `json:"context,omitempty"`
	// ShowLineNumbers controls ripgrep -n in content mode and defaults to true.
	ShowLineNumbers *bool `json:"-n,omitempty"`
	// CaseInsensitive enables ripgrep -i across output modes.
	CaseInsensitive bool `json:"-i,omitempty"`
	// Type restricts the search to one ripgrep file type.
	Type string `json:"type,omitempty"`
	// HeadLimit limits returned rows after search execution; zero disables pagination.
	HeadLimit *int `json:"head_limit,omitempty"`
	// Offset skips the first N rows before applying HeadLimit.
	Offset int `json:"offset,omitempty"`
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
	// AppliedLimit reports the effective page size when the result set was truncated.
	AppliedLimit *int `json:"appliedLimit,omitempty"`
	// AppliedOffset reports the number of rows skipped before rendering this page.
	AppliedOffset *int `json:"appliedOffset,omitempty"`
}

// matchCandidate stores one ripgrep hit together with its modification time for stable sorting.
type matchCandidate struct {
	// absolutePath keeps the canonical path returned by ripgrep and used for stat calls.
	absolutePath string
	// modTime keeps the file modification time used for descending sort order.
	modTime time.Time
}

// searchRequest collects the normalized search parameters used by one grep invocation.
type searchRequest struct {
	// pattern stores the ripgrep pattern after input normalization.
	pattern string
	// glob stores the optional caller-supplied ripgrep glob string.
	glob string
	// searchPath stores the absolute file or directory handed to ripgrep.
	searchPath string
	// workingDir stores the caller cwd used for relative path rendering.
	workingDir string
	// outputMode selects the ripgrep flags and result projection.
	outputMode string
	// contextBefore stores ripgrep -B for content mode.
	contextBefore *int
	// contextAfter stores ripgrep -A for content mode.
	contextAfter *int
	// context stores ripgrep -C for content mode and takes precedence over before/after.
	context *int
	// showLineNumbers controls ripgrep -n for content mode.
	showLineNumbers bool
	// caseInsensitive controls ripgrep -i.
	caseInsensitive bool
	// fileType stores the optional ripgrep --type filter.
	fileType string
	// headLimit limits projected rows after search execution; zero means unlimited.
	headLimit *int
	// offset skips projected rows before headLimit is applied.
	offset int
	// start stores the invocation start time for duration accounting.
	start time.Time
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

// Invoke validates input, enforces read permissions, runs ripgrep, and returns the requested projection.
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

	request := searchRequest{
		pattern:         strings.TrimSpace(input.Pattern),
		glob:            strings.TrimSpace(input.Glob),
		searchPath:      searchPath,
		workingDir:      call.Context.WorkingDir,
		outputMode:      normalizeOutputMode(input.OutputMode),
		contextBefore:   input.ContextBefore,
		contextAfter:    input.ContextAfter,
		context:         effectiveContext(input.Context, input.ContextShort),
		showLineNumbers: normalizeShowLineNumbers(input.ShowLineNumbers),
		caseInsensitive: input.CaseInsensitive,
		fileType:        strings.TrimSpace(input.Type),
		headLimit:       input.HeadLimit,
		offset:          clampNonNegative(input.Offset),
		start:           start,
	}

	logger.DebugCF("grep_tool", "starting grep search", map[string]any{
		"pattern":          input.Pattern,
		"search_path":      searchPath,
		"glob":             input.Glob,
		"output_mode":      request.outputMode,
		"context_before":   derefInt(input.ContextBefore),
		"context_after":    derefInt(input.ContextAfter),
		"context":          derefInt(request.context),
		"show_line_number": request.showLineNumbers,
		"case_insensitive": request.caseInsensitive,
		"file_type":        request.fileType,
		"head_limit":       derefInt(input.HeadLimit),
		"offset":           request.offset,
		"working_dir":      call.Context.WorkingDir,
	})

	output, err := t.runSearch(ctx, request)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	logger.DebugCF("grep_tool", "grep search finished", map[string]any{
		"pattern":        input.Pattern,
		"search_path":    searchPath,
		"glob":           input.Glob,
		"output_mode":    output.Mode,
		"num_files":      output.NumFiles,
		"num_lines":      output.NumLines,
		"num_matches":    output.NumMatches,
		"applied_limit":  derefInt(output.AppliedLimit),
		"applied_offset": derefInt(output.AppliedOffset),
		"is_dir":         info.IsDir(),
	})

	return coretool.Result{
		Output: renderOutput(output),
		Meta: map[string]any{
			"data": output,
		},
	}, nil
}

// inputSchema declares the GrepTool input contract for the second migration pass.
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
			"-B": {
				Type:        coretool.ValueKindInteger,
				Description: `Optional number of lines to show before each match in "content" mode.`,
			},
			"-A": {
				Type:        coretool.ValueKindInteger,
				Description: `Optional number of lines to show after each match in "content" mode.`,
			},
			"-C": {
				Type:        coretool.ValueKindInteger,
				Description: `Optional alias for symmetric context lines in "content" mode.`,
			},
			"context": {
				Type:        coretool.ValueKindInteger,
				Description: `Optional number of lines to show before and after each match in "content" mode.`,
			},
			"-n": {
				Type:        coretool.ValueKindBoolean,
				Description: `Optional toggle for line numbers in "content" mode; defaults to true.`,
			},
			"-i": {
				Type:        coretool.ValueKindBoolean,
				Description: "Optional case-insensitive search flag.",
			},
			"type": {
				Type:        coretool.ValueKindString,
				Description: "Optional ripgrep file type filter.",
			},
			"head_limit": {
				Type:        coretool.ValueKindInteger,
				Description: "Optional limit for returned rows; zero disables pagination.",
			},
			"offset": {
				Type:        coretool.ValueKindInteger,
				Description: "Optional number of rows to skip before head_limit is applied.",
			},
		},
	}
}

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
			Mode:          outputModeContent,
			DurationMs:    time.Since(request.start).Milliseconds(),
			Content:       strings.Join(relativeLines, "\n"),
			NumLines:      len(relativeLines),
			AppliedLimit:  appliedLimit,
			AppliedOffset: appliedOffset,
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
			Mode:          outputModeCount,
			DurationMs:    time.Since(request.start).Milliseconds(),
			NumFiles:      numFiles,
			Content:       strings.Join(relativeLines, "\n"),
			NumMatches:    numMatches,
			AppliedLimit:  appliedLimit,
			AppliedOffset: appliedOffset,
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
			Mode:          outputModeFilesWithMatches,
			DurationMs:    time.Since(request.start).Milliseconds(),
			NumFiles:      len(filenames),
			Filenames:     filenames,
			AppliedLimit:  appliedLimit,
			AppliedOffset: appliedOffset,
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

// effectiveContext picks the symmetric content-context flag, preferring the long-form input.
func effectiveContext(context *int, contextAlias *int) *int {
	if context != nil {
		return context
	}
	return contextAlias
}

// normalizeShowLineNumbers keeps content mode aligned with the source default of showing line numbers.
func normalizeShowLineNumbers(value *bool) bool {
	if value == nil {
		return true
	}
	return *value
}

// splitGlobPatterns expands a comma-or-space-separated glob string into ripgrep --glob arguments.
func splitGlobPatterns(glob string) []string {
	trimmed := strings.TrimSpace(glob)
	if trimmed == "" {
		return nil
	}

	rawPatterns := strings.Fields(trimmed)
	patterns := make([]string, 0, len(rawPatterns))
	for _, rawPattern := range rawPatterns {
		if strings.Contains(rawPattern, "{") && strings.Contains(rawPattern, "}") {
			patterns = append(patterns, rawPattern)
			continue
		}
		for _, part := range strings.Split(rawPattern, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				patterns = append(patterns, part)
			}
		}
	}
	return patterns
}

// applyHeadLimit paginates rows or matches and reports whether truncation metadata should be surfaced.
func applyHeadLimit[T any](items []T, limit *int, offset int) ([]T, *int, *int) {
	safeOffset := clampOffset(offset, len(items))
	if limit != nil && *limit == 0 {
		return items[safeOffset:], nil, offsetPointer(safeOffset)
	}

	effectiveLimit := defaultHeadLimit
	if limit != nil {
		effectiveLimit = clampNonNegative(*limit)
	}

	end := safeOffset + effectiveLimit
	if end > len(items) {
		end = len(items)
	}

	var appliedLimit *int
	if len(items)-safeOffset > effectiveLimit {
		appliedLimit = intPointer(effectiveLimit)
	}

	return items[safeOffset:end], appliedLimit, offsetPointer(safeOffset)
}

// clampNonNegative normalizes user-provided paging values to non-negative integers.
func clampNonNegative(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

// clampOffset keeps pagination offsets within the result bounds.
func clampOffset(offset int, itemCount int) int {
	normalized := clampNonNegative(offset)
	if normalized > itemCount {
		return itemCount
	}
	return normalized
}

// intPointer allocates one stable integer pointer for optional output fields.
func intPointer(value int) *int {
	return &value
}

// offsetPointer omits zero offsets from the structured pagination metadata.
func offsetPointer(offset int) *int {
	if offset <= 0 {
		return nil
	}
	return intPointer(offset)
}

// derefInt turns optional integers into log-friendly values.
func derefInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
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

// formatLimitInfo renders the human-facing pagination suffix shown in tool output.
func formatLimitInfo(appliedLimit *int, appliedOffset *int) string {
	parts := make([]string, 0, 2)
	if appliedLimit != nil {
		parts = append(parts, fmt.Sprintf("limit: %d", *appliedLimit))
	}
	if appliedOffset != nil && *appliedOffset > 0 {
		parts = append(parts, fmt.Sprintf("offset: %d", *appliedOffset))
	}
	return strings.Join(parts, ", ")
}

// renderOutput formats the caller-facing result body for the current migration pass.
func renderOutput(output Output) string {
	switch output.Mode {
	case outputModeContent:
		if strings.TrimSpace(output.Content) == "" {
			return "No matches found"
		}
		limitInfo := formatLimitInfo(output.AppliedLimit, output.AppliedOffset)
		if limitInfo == "" {
			return output.Content
		}
		return output.Content + "\n\n[Showing results with pagination = " + limitInfo + "]"
	case outputModeCount:
		if strings.TrimSpace(output.Content) == "" {
			return "No matches found"
		}
		limitInfo := formatLimitInfo(output.AppliedLimit, output.AppliedOffset)
		summary := fmt.Sprintf(
			"\n\nFound %d total %s across %d %s%s.",
			output.NumMatches,
			pluralize(output.NumMatches, "occurrence", "occurrences"),
			output.NumFiles,
			pluralize(output.NumFiles, "file", "files"),
			formatPaginationSuffix(limitInfo),
		)
		return output.Content + summary
	default:
		if output.NumFiles == 0 {
			return "No files found"
		}
		limitInfo := formatLimitInfo(output.AppliedLimit, output.AppliedOffset)
		if limitInfo == "" {
			return strings.Join(output.Filenames, "\n")
		}
		return fmt.Sprintf(
			"Found %d %s %s\n%s",
			output.NumFiles,
			pluralize(output.NumFiles, "file", "files"),
			limitInfo,
			strings.Join(output.Filenames, "\n"),
		)
	}
}

// formatPaginationSuffix appends pagination details to summary text only when needed.
func formatPaginationSuffix(limitInfo string) string {
	if limitInfo == "" {
		return ""
	}
	return " with pagination = " + limitInfo
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
