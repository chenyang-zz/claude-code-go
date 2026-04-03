package glob

import (
	"context"
	"fmt"
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
	// Name is the stable registry identifier used by the migrated GlobTool.
	Name = "Glob"
	// defaultMaxResults caps the number of matches returned to callers.
	defaultMaxResults = 100
)

// Tool implements GlobTool on top of the Go host architecture.
type Tool struct {
	// fs provides filesystem traversal and metadata access for glob searches.
	fs platformfs.FileSystem
	// policy performs the minimal read-permission gate before traversal starts.
	policy *corepermission.FilesystemPolicy
	// maxResults caps returned matches and drives truncation reporting.
	maxResults int
}

// Input is the typed request payload accepted by the migrated GlobTool.
type Input struct {
	// Pattern stores the glob expression used to filter candidate files.
	Pattern string `json:"pattern"`
	// Path optionally overrides the base directory to search from.
	Path string `json:"path,omitempty"`
}

// Output is the structured search result returned in tool metadata.
type Output struct {
	// DurationMs records the end-to-end search latency.
	DurationMs int64 `json:"durationMs"`
	// NumFiles reports how many filenames are included in the result payload.
	NumFiles int `json:"numFiles"`
	// Filenames contains the caller-facing relative or absolute paths for matched files.
	Filenames []string `json:"filenames"`
	// Truncated reports whether additional matches were dropped due to the result cap.
	Truncated bool `json:"truncated"`
}

// matchCandidate stores one file hit together with its modification time for stable sorting.
type matchCandidate struct {
	// absolutePath keeps the canonical path used during matching and permission checks.
	absolutePath string
	// modTime keeps the file modification time used for ascending sort order.
	modTime time.Time
}

// NewTool constructs a GlobTool with explicit host dependencies.
func NewTool(fs platformfs.FileSystem, policy *corepermission.FilesystemPolicy) *Tool {
	return &Tool{
		fs:         fs,
		policy:     policy,
		maxResults: defaultMaxResults,
	}
}

// Name returns the stable registration name for this tool.
func (t *Tool) Name() string {
	return Name
}

// Description returns the summary exposed to callers and tests.
func (t *Tool) Description() string {
	return "Fast file pattern matching tool that returns matching file paths sorted by modification time."
}

// IsReadOnly reports that glob search never mutates external state.
func (t *Tool) IsReadOnly() bool {
	return true
}

// IsConcurrencySafe reports that independent glob traversals may run in parallel safely.
func (t *Tool) IsConcurrencySafe() bool {
	return true
}

// Invoke validates input, enforces read permissions, performs the filesystem walk, and returns formatted hits.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{}, fmt.Errorf("glob tool: nil receiver")
	}
	if t.fs == nil {
		return coretool.Result{}, fmt.Errorf("glob tool: filesystem is not configured")
	}
	if t.policy == nil {
		return coretool.Result{}, fmt.Errorf("glob tool: permission policy is not configured")
	}

	start := time.Now()

	input, err := coretool.DecodeInput[Input](inputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	searchRoot, err := platformfs.ExpandPath(input.Path, call.Context.WorkingDir)
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("glob tool: expand path: %v", err)}, nil
	}

	info, err := t.fs.Stat(searchRoot)
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("Directory does not exist: %s", inputPathOrWorkingDir(input.Path, searchRoot))}, nil
	}
	if !info.IsDir() {
		return coretool.Result{Error: fmt.Sprintf("Path is not a directory: %s", input.Path)}, nil
	}

	permissionReq := corepermission.FilesystemRequest{
		ToolName:   t.Name(),
		Path:       searchRoot,
		WorkingDir: call.Context.WorkingDir,
		Access:     corepermission.AccessRead,
	}
	evaluation := t.policy.CheckReadPermissionForTool(ctx, t.Name(), searchRoot, call.Context.WorkingDir)
	if err := evaluation.ToError(permissionReq); err != nil {
		return coretool.Result{}, err
	}

	logger.DebugCF("glob_tool", "starting glob search", map[string]any{
		"pattern":     input.Pattern,
		"search_root": searchRoot,
		"working_dir": call.Context.WorkingDir,
		"max_results": t.effectiveMaxResults(),
	})

	matches, truncated, err := t.collectMatches(ctx, searchRoot, strings.TrimSpace(input.Pattern))
	if err != nil {
		return coretool.Result{}, err
	}

	sort.Slice(matches, func(i, j int) bool {
		if !matches[i].modTime.Equal(matches[j].modTime) {
			return matches[i].modTime.Before(matches[j].modTime)
		}
		return matches[i].absolutePath < matches[j].absolutePath
	})

	filenames := make([]string, 0, len(matches))
	for _, match := range matches {
		filenames = append(filenames, platformfs.ToRelativePath(match.absolutePath, call.Context.WorkingDir))
	}

	output := Output{
		DurationMs: time.Since(start).Milliseconds(),
		NumFiles:   len(filenames),
		Filenames:  filenames,
		Truncated:  truncated,
	}

	logger.DebugCF("glob_tool", "glob search finished", map[string]any{
		"pattern":     input.Pattern,
		"search_root": searchRoot,
		"num_files":   output.NumFiles,
		"truncated":   output.Truncated,
	})

	return coretool.Result{
		Output: renderOutput(output),
		Meta: map[string]any{
			"data": output,
		},
	}, nil
}

// inputSchema declares the GlobTool input contract.
func inputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"pattern": {
				Type:        coretool.ValueKindString,
				Description: "The glob pattern to match files against.",
				Required:    true,
			},
			"path": {
				Type:        coretool.ValueKindString,
				Description: "Optional directory to search in.",
			},
		},
	}
}

// collectMatches walks the requested root and returns matching regular files together with truncation state.
func (t *Tool) collectMatches(ctx context.Context, root string, pattern string) ([]matchCandidate, bool, error) {
	if pattern == "" {
		return nil, false, fmt.Errorf("glob tool: pattern is required")
	}

	maxResults := t.effectiveMaxResults()
	matches := make([]matchCandidate, 0, min(maxResults, 16))
	truncated := false

	err := t.walkMatches(ctx, root, root, pattern, maxResults, &matches, &truncated)
	if err != nil {
		return nil, false, err
	}

	return matches, truncated, nil
}

// walkMatches recursively traverses the requested root using the shared platform filesystem abstraction.
func (t *Tool) walkMatches(ctx context.Context, root string, currentDir string, pattern string, maxResults int, matches *[]matchCandidate, truncated *bool) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	entries, err := t.fs.ReadDir(currentDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		entryPath := filepath.Join(currentDir, entry.Name())
		if entry.IsDir() {
			if err := t.walkMatches(ctx, root, entryPath, pattern, maxResults, matches, truncated); err != nil {
				return err
			}
			continue
		}

		relativePath, err := filepath.Rel(root, entryPath)
		if err != nil {
			return err
		}

		matched, err := matchesGlobPattern(filepath.ToSlash(relativePath), entry.Name(), filepath.ToSlash(pattern))
		if err != nil {
			return fmt.Errorf("glob tool: invalid pattern %q: %w", pattern, err)
		}
		if !matched {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		if len(*matches) < maxResults {
			*matches = append(*matches, matchCandidate{
				absolutePath: entryPath,
				modTime:      info.ModTime(),
			})
			continue
		}

		*truncated = true
		return nil
	}

	return nil
}

// effectiveMaxResults returns the configured cap while preserving a sane default when tests inject zero.
func (t *Tool) effectiveMaxResults() int {
	if t.maxResults > 0 {
		return t.maxResults
	}
	return defaultMaxResults
}

// renderOutput converts the structured result into the minimal caller-facing text payload.
func renderOutput(output Output) string {
	if len(output.Filenames) == 0 {
		return "No files found"
	}

	lines := append([]string{}, output.Filenames...)
	if output.Truncated {
		lines = append(lines, "(Results are truncated. Consider using a more specific path or pattern.)")
	}
	return strings.Join(lines, "\n")
}

// inputPathOrWorkingDir returns the user-provided path when available, otherwise the resolved search root.
func inputPathOrWorkingDir(inputPath string, resolved string) string {
	if strings.TrimSpace(inputPath) != "" {
		return inputPath
	}
	return resolved
}

// matchesGlobPattern implements the glob subset needed for `*.go` and `src/**/*.ts` style searches.
func matchesGlobPattern(relativePath string, baseName string, pattern string) (bool, error) {
	normalizedPath := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(relativePath)), "./")
	normalizedPattern := strings.TrimSpace(filepath.ToSlash(pattern))
	if normalizedPattern == "" {
		return false, nil
	}

	if !strings.Contains(normalizedPattern, "/") {
		return filepath.Match(normalizedPattern, baseName)
	}

	return matchPathSegments(splitSegments(normalizedPath), splitSegments(normalizedPattern))
}

// splitSegments converts a slash-separated path into comparable segments.
func splitSegments(value string) []string {
	if value == "" || value == "." {
		return nil
	}
	return strings.Split(value, "/")
}

// matchPathSegments recursively evaluates path segments with support for `**` wildcards.
func matchPathSegments(pathSegments []string, patternSegments []string) (bool, error) {
	if len(patternSegments) == 0 {
		return len(pathSegments) == 0, nil
	}

	if patternSegments[0] == "**" {
		if len(patternSegments) == 1 {
			return true, nil
		}
		for index := 0; index <= len(pathSegments); index++ {
			matched, err := matchPathSegments(pathSegments[index:], patternSegments[1:])
			if err != nil {
				return false, err
			}
			if matched {
				return true, nil
			}
		}
		return false, nil
	}

	if len(pathSegments) == 0 {
		return false, nil
	}

	matched, err := filepath.Match(patternSegments[0], pathSegments[0])
	if err != nil {
		return false, err
	}
	if !matched {
		return false, nil
	}

	return matchPathSegments(pathSegments[1:], patternSegments[1:])
}

// min keeps small slice-capacity setup readable without dragging in extra helpers.
func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
