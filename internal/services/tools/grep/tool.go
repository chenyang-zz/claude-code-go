package grep

import (
	"context"
	"fmt"
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
	// Multiline enables ripgrep multiline mode so patterns can span line boundaries.
	Multiline bool `json:"multiline,omitempty"`
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
	// PaginationSummary renders a user-facing description of the current page within the full result set.
	PaginationSummary string `json:"paginationSummary,omitempty"`
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
	// multiline enables ripgrep multiline mode.
	multiline bool
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

// InputSchema returns the GrepTool input contract exposed to model providers.
func (t *Tool) InputSchema() coretool.InputSchema {
	return inputSchema()
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
		multiline:       input.Multiline,
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
		"multiline":        request.multiline,
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
