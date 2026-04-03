package grep

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
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
}

// Output is the structured search result returned in tool metadata.
type Output struct {
	// DurationMs records the end-to-end search latency.
	DurationMs int64 `json:"durationMs"`
	// NumFiles reports how many filenames are included in the result payload.
	NumFiles int `json:"numFiles"`
	// Filenames contains the caller-facing relative or absolute paths for matched files.
	Filenames []string `json:"filenames"`
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
	return "Fast content search tool backed by ripgrep that returns matching file paths sorted by modification time."
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
		"working_dir": call.Context.WorkingDir,
	})

	results, err := t.runRipgrep(ctx, strings.TrimSpace(input.Pattern), strings.TrimSpace(input.Glob), searchPath)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	matches, err := t.collectMatches(results)
	if err != nil {
		return coretool.Result{}, err
	}

	sortMatches(matches)

	filenames := make([]string, 0, len(matches))
	for _, match := range matches {
		filenames = append(filenames, platformfs.ToRelativePath(match.absolutePath, call.Context.WorkingDir))
	}

	output := Output{
		DurationMs: time.Since(start).Milliseconds(),
		NumFiles:   len(filenames),
		Filenames:  filenames,
	}

	logger.DebugCF("grep_tool", "grep search finished", map[string]any{
		"pattern":     input.Pattern,
		"search_path": searchPath,
		"glob":        input.Glob,
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
		},
	}
}

// runRipgrep executes the host ripgrep binary and returns absolute matching file paths.
func (t *Tool) runRipgrep(ctx context.Context, pattern string, glob string, searchPath string) ([]string, error) {
	if pattern == "" {
		return nil, fmt.Errorf("grep tool: pattern is required")
	}

	commandName := t.commandName
	if strings.TrimSpace(commandName) == "" {
		commandName = defaultCommandName
	}

	args := []string{"--hidden", "--files-with-matches"}
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

// renderOutput formats the caller-facing result body for the first migration pass.
func renderOutput(output Output) string {
	if output.NumFiles == 0 {
		return "No files found"
	}
	return strings.Join(output.Filenames, "\n")
}

// inputPathOrWorkingDir keeps handled validation errors aligned with the user-provided path.
func inputPathOrWorkingDir(inputPath string, expandedPath string) string {
	if strings.TrimSpace(inputPath) == "" {
		return expandedPath
	}
	return inputPath
}
