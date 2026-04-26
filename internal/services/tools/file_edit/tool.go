package file_edit

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	platformconfig "github.com/sheepzhao/claude-code-go/internal/platform/config"
	platformfs "github.com/sheepzhao/claude-code-go/internal/platform/fs"
	toolshared "github.com/sheepzhao/claude-code-go/internal/services/tools/shared"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// Name is the stable registry identifier used by the migrated FileEditTool.
	Name = "Edit"
	// defaultFilePerm is the fallback file mode for files created through the empty-old-string branch.
	defaultFilePerm = 0o644
	// defaultDirPerm is the fallback directory mode for parent directories created by the tool.
	defaultDirPerm = 0o755
	// unreadBeforeEditError mirrors the source tool's rejection when an existing file was not fully read first.
	unreadBeforeEditError = "File has not been read yet. Read it first before writing to it."
	// modifiedSinceReadError mirrors the source tool's rejection when the file changed after the recorded read.
	modifiedSinceReadError = "File has been modified since read, either by the user or by a linter. Read it again before attempting to write it."
	// fileAlreadyExistsForCreateError mirrors the source tool's rejection when an empty-old-string create is attempted against a non-empty file.
	fileAlreadyExistsForCreateError = "Cannot create new file - file already exists."
	// notebookEditToolName mirrors the source tool name referenced by the dedicated notebook rejection path.
	notebookEditToolName = "NotebookEdit"
	// maxEditFileSize mirrors the source tool's 1 GiB byte-level guard for edit operations.
	maxEditFileSize = int64(1024 * 1024 * 1024)
)

// Tool implements the first-batch FileEditTool.
type Tool struct {
	// fs provides the file read and write operations needed for in-place edits.
	fs platformfs.FileSystem
	// policy performs the minimal write-permission check before mutations happen.
	policy *corepermission.FilesystemPolicy
}

// Input is the typed request payload accepted by the migrated FileEditTool.
type Input struct {
	// FilePath stores the caller-provided file path to edit.
	FilePath string `json:"file_path"`
	// OldString stores the exact text that must be matched before editing.
	OldString string `json:"old_string"`
	// NewString stores the replacement text written back to the file.
	NewString string `json:"new_string"`
	// ReplaceAll controls whether every matching occurrence should be replaced.
	ReplaceAll bool `json:"replace_all,omitempty"`
}

// Output is the structured edit result returned in tool metadata.
type Output struct {
	// FilePath stores the caller-facing path of the edited file.
	FilePath string `json:"filePath"`
	// OldString stores the caller-provided search text.
	OldString string `json:"oldString"`
	// NewString stores the caller-provided replacement text.
	NewString string `json:"newString"`
	// Replacements reports how many occurrences were replaced.
	Replacements int `json:"replacements"`
	// Content stores the final file content after the edit succeeds.
	Content string `json:"content"`
	// OriginalContent stores the pre-edit file content for callers and tests.
	OriginalContent string `json:"originalContent"`
	// StructuredPatch stores the minimal diff payload shared with other write-oriented tools.
	StructuredPatch []toolshared.Hunk `json:"structuredPatch"`
}

// NewTool constructs a FileEditTool with explicit host dependencies.
func NewTool(fs platformfs.FileSystem, policy *corepermission.FilesystemPolicy) *Tool {
	return &Tool{
		fs:     fs,
		policy: policy,
	}
}

// Name returns the stable registration name for this tool.
func (t *Tool) Name() string {
	return Name
}

// Description returns the summary exposed to callers and tests.
func (t *Tool) Description() string {
	return `Performs exact string replacements in files.

Usage:
- You must use your Read tool at least once in the conversation before editing. This tool will error if you attempt an edit without reading the file.
- When editing text from Read tool output, ensure you preserve the exact indentation (tabs/spaces) as it appears AFTER the line number prefix. The line number prefix format is: line number + tab. Everything after that is the actual file content to match. Never include any part of the line number prefix in the old_string or new_string.
- ALWAYS prefer editing existing files in the codebase. NEVER write new files unless explicitly required.
- Only use emojis if the user explicitly requests it. Avoid adding emojis to files unless asked.
- The edit will FAIL if old_string is not unique in the file. Either provide a larger string with more surrounding context to make it unique or use replace_all to change every instance of old_string.
- Use replace_all for replacing and renaming strings across the file. This parameter is useful if you want to rename a variable for instance.`
}

// InputSchema returns the FileEditTool input contract exposed to model providers.
func (t *Tool) InputSchema() coretool.InputSchema {
	return inputSchema()
}

// IsReadOnly reports that file editing mutates external state.
func (t *Tool) IsReadOnly() bool {
	return false
}

// IsConcurrencySafe reports that concurrent edits to arbitrary files should not be assumed safe.
func (t *Tool) IsConcurrencySafe() bool {
	return false
}

// Invoke validates input, enforces write permissions, applies one exact replacement, and writes the updated file back.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{}, fmt.Errorf("file edit tool: nil receiver")
	}
	if t.fs == nil {
		return coretool.Result{}, fmt.Errorf("file edit tool: filesystem is not configured")
	}
	if t.policy == nil {
		return coretool.Result{}, fmt.Errorf("file edit tool: permission policy is not configured")
	}

	input, err := coretool.DecodeInput[Input](inputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}
	if strings.TrimSpace(input.FilePath) == "" {
		return coretool.Result{Error: "file_path is required"}, nil
	}

	filePath, err := platformfs.ExpandPath(input.FilePath, call.Context.WorkingDir)
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("file edit tool: expand path: %v", err)}, nil
	}

	if secretError := toolshared.CheckTeamMemorySecrets(filePath, input.NewString); secretError != "" {
		return coretool.Result{Error: secretError}, nil
	}

	if input.OldString == input.NewString {
		return coretool.Result{Error: "No changes to make: old_string and new_string are exactly the same."}, nil
	}

	evaluation := t.policy.CheckWritePermissionForTool(ctx, t.Name(), filePath, call.Context.WorkingDir)
	if err := evaluation.ToError(corepermission.FilesystemRequest{
		ToolName:   t.Name(),
		Path:       filePath,
		WorkingDir: call.Context.WorkingDir,
		Access:     corepermission.AccessWrite,
	}); err != nil {
		return coretool.Result{}, err
	}

	logger.DebugCF("file_edit_tool", "starting file edit", map[string]any{
		"file_path":   filePath,
		"working_dir": call.Context.WorkingDir,
		"replace_all": input.ReplaceAll,
	})

	if input.OldString == "" {
		return t.handleEmptyOldString(call.Context, input, filePath)
	}

	originalContent, filePerm, err := t.readEditableFile(call.Context, input.FilePath, filePath)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	actualOldString, ok := findActualString(originalContent, input.OldString)
	if !ok {
		return coretool.Result{
			Error: fmt.Sprintf("String to replace not found in file.\nString: %s", input.OldString),
		}, nil
	}

	actualNewString := preserveQuoteStyle(input.OldString, actualOldString, input.NewString)
	matchCount := strings.Count(originalContent, actualOldString)
	if matchCount == 0 {
		return coretool.Result{
			Error: fmt.Sprintf("String to replace not found in file.\nString: %s", input.OldString),
		}, nil
	}
	if matchCount > 1 && !input.ReplaceAll {
		return coretool.Result{
			Error: fmt.Sprintf("Found %d matches of the string to replace, but replace_all is false. To replace all occurrences, set replace_all to true. To replace only one occurrence, please provide more context to uniquely identify the instance.\nString: %s", matchCount, input.OldString),
		}, nil
	}

	updatedContent := originalContent
	replacementCount := 1
	if input.ReplaceAll {
		updatedContent = strings.ReplaceAll(originalContent, actualOldString, actualNewString)
		replacementCount = matchCount
	} else {
		updatedContent = strings.Replace(originalContent, actualOldString, actualNewString, 1)
	}
	if err := validateSettingsFileEdit(filePath, originalContent, updatedContent); err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	if err := t.fs.WriteFile(filePath, []byte(updatedContent), filePerm); err != nil {
		return coretool.Result{Error: fmt.Sprintf("file edit tool: write file: %v", err)}, nil
	}

	output := Output{
		FilePath:        platformfs.ToRelativePath(filePath, call.Context.WorkingDir),
		OldString:       input.OldString,
		NewString:       input.NewString,
		Replacements:    replacementCount,
		Content:         updatedContent,
		OriginalContent: originalContent,
		StructuredPatch: toolshared.BuildStructuredPatch(originalContent, updatedContent),
	}

	logger.DebugCF("file_edit_tool", "file edit finished", map[string]any{
		"file_path":    filePath,
		"replacements": replacementCount,
	})

	return coretool.Result{
		Output: renderOutput(output),
		Meta: map[string]any{
			"data": output,
		},
	}, nil
}

// readEditableFile loads the current file content for the normal edit path and enforces existing-file safety checks.
func (t *Tool) readEditableFile(useContext coretool.UseContext, requestedPath string, filePath string) (string, os.FileMode, error) {
	info, err := t.fs.Stat(filePath)
	if err != nil {
		if platformfs.IsNotExist(err) {
			return "", defaultFilePerm, fmt.Errorf("File does not exist: %s", requestedPath)
		}
		return "", 0, fmt.Errorf("file edit tool: inspect target: %v", err)
	}
	if info.IsDir() {
		return "", 0, fmt.Errorf("Path is a directory, not a file: %s", requestedPath)
	}
	if info.Size() > maxEditFileSize {
		return "", 0, fmt.Errorf("File is too large to edit (%s). Maximum editable file size is %s.", formatFileSize(info.Size()), formatFileSize(maxEditFileSize))
	}
	if strings.HasSuffix(strings.ToLower(filePath), ".ipynb") {
		return "", 0, fmt.Errorf("File is a Jupyter Notebook. Use the %s to edit this file.", notebookEditToolName)
	}
	if err := validateExistingFileReadState(useContext, filePath, info.ModTime()); err != nil {
		return "", 0, err
	}

	originalBytes, err := t.fs.ReadFile(filePath)
	if err != nil {
		return "", 0, fmt.Errorf("file edit tool: read file: %v", err)
	}
	if !utf8.Valid(originalBytes) {
		return "", 0, fmt.Errorf("This tool cannot edit binary files. The file appears to contain non-text content.")
	}

	return string(originalBytes), info.Mode().Perm(), nil
}

// handleEmptyOldString applies the source-aligned create/empty-file replacement branch used when old_string is empty.
func (t *Tool) handleEmptyOldString(useContext coretool.UseContext, input Input, filePath string) (coretool.Result, error) {
	filePerm := os.FileMode(defaultFilePerm)
	originalContent := ""

	info, err := t.fs.Stat(filePath)
	switch {
	case err == nil:
		if info.IsDir() {
			return coretool.Result{Error: fmt.Sprintf("Path is a directory, not a file: %s", input.FilePath)}, nil
		}
		originalBytes, readErr := t.fs.ReadFile(filePath)
		if readErr != nil {
			return coretool.Result{Error: fmt.Sprintf("file edit tool: read file: %v", readErr)}, nil
		}
		if !utf8.Valid(originalBytes) {
			return coretool.Result{Error: "This tool cannot edit binary files. The file appears to contain non-text content."}, nil
		}

		originalContent = string(originalBytes)
		filePerm = info.Mode().Perm()
		if strings.TrimSpace(originalContent) != "" {
			return coretool.Result{Error: fileAlreadyExistsForCreateError}, nil
		}
	case platformfs.IsNotExist(err):
		originalContent = ""
	default:
		return coretool.Result{Error: fmt.Sprintf("file edit tool: inspect target: %v", err)}, nil
	}

	parentDir := filepath.Dir(filePath)
	if err := t.fs.MkdirAll(parentDir, defaultDirPerm); err != nil {
		return coretool.Result{Error: fmt.Sprintf("file edit tool: create parent directories: %v", err)}, nil
	}
	if err := t.fs.WriteFile(filePath, []byte(input.NewString), filePerm); err != nil {
		return coretool.Result{Error: fmt.Sprintf("file edit tool: write file: %v", err)}, nil
	}

	output := Output{
		FilePath:        platformfs.ToRelativePath(filePath, useContext.WorkingDir),
		OldString:       input.OldString,
		NewString:       input.NewString,
		Replacements:    1,
		Content:         input.NewString,
		OriginalContent: originalContent,
		StructuredPatch: toolshared.BuildStructuredPatch(originalContent, input.NewString),
	}

	logger.DebugCF("file_edit_tool", "file edit finished", map[string]any{
		"file_path":    filePath,
		"replacements": 1,
		"empty_old":    true,
	})

	return coretool.Result{
		Output: renderOutput(output),
		Meta: map[string]any{
			"data": output,
		},
	}, nil
}

// inputSchema declares the FileEditTool input contract for the first migration pass.
func inputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"file_path": {
				Type:        coretool.ValueKindString,
				Description: "The file path to edit.",
				Required:    true,
			},
			"old_string": {
				Type:        coretool.ValueKindString,
				Description: "The exact string to replace.",
				Required:    true,
			},
			"new_string": {
				Type:        coretool.ValueKindString,
				Description: "The replacement string.",
				Required:    true,
			},
			"replace_all": {
				Type:        coretool.ValueKindBoolean,
				Description: "Whether to replace all occurrences of old_string.",
			},
		},
	}
}

// renderOutput converts the structured edit result into the minimal caller-facing text payload.
func renderOutput(output Output) string {
	return fmt.Sprintf("Updated file: %s (%d replacement)", output.FilePath, output.Replacements)
}

// validateExistingFileReadState enforces the minimal "read before edit" and drift-protection guards for existing files.
func validateExistingFileReadState(context coretool.UseContext, filePath string, currentModTime time.Time) error {
	state, ok := context.LookupReadState(filePath)
	if !ok || state.IsPartial {
		return fmt.Errorf(unreadBeforeEditError)
	}
	if !state.ObservedModTime.IsZero() && currentModTime.After(state.ObservedModTime) {
		return fmt.Errorf(modifiedSinceReadError)
	}

	return nil
}

const (
	leftSingleCurlyQuote  = '‘'
	rightSingleCurlyQuote = '’'
	leftDoubleCurlyQuote  = '“'
	rightDoubleCurlyQuote = '”'
)

// normalizeQuotes converts curly quotes into their straight-quote equivalents for matching.
func normalizeQuotes(value string) string {
	return strings.NewReplacer(
		string(leftSingleCurlyQuote), "'",
		string(rightSingleCurlyQuote), "'",
		string(leftDoubleCurlyQuote), "\"",
		string(rightDoubleCurlyQuote), "\"",
	).Replace(value)
}

// findActualString resolves the caller-provided search string to the exact file substring, allowing quote-only differences.
func findActualString(fileContent string, search string) (string, bool) {
	if strings.Contains(fileContent, search) {
		return search, true
	}

	normalizedSearch := []rune(normalizeQuotes(search))
	normalizedFile := []rune(normalizeQuotes(fileContent))
	searchIndex := indexRunes(normalizedFile, normalizedSearch)
	if searchIndex == -1 {
		return "", false
	}

	actualRunes := []rune(fileContent)
	searchWidth := len([]rune(search))
	return string(actualRunes[searchIndex : searchIndex+searchWidth]), true
}

// preserveQuoteStyle mirrors the file's curly-quote typography when the match was found via quote normalization.
func preserveQuoteStyle(expectedOld string, actualOld string, newValue string) string {
	if expectedOld == actualOld {
		return newValue
	}

	hasDoubleCurlyQuotes := strings.ContainsRune(actualOld, leftDoubleCurlyQuote) || strings.ContainsRune(actualOld, rightDoubleCurlyQuote)
	hasSingleCurlyQuotes := strings.ContainsRune(actualOld, leftSingleCurlyQuote) || strings.ContainsRune(actualOld, rightSingleCurlyQuote)
	if !hasDoubleCurlyQuotes && !hasSingleCurlyQuotes {
		return newValue
	}

	result := newValue
	if hasDoubleCurlyQuotes {
		result = applyCurlyDoubleQuotes(result)
	}
	if hasSingleCurlyQuotes {
		result = applyCurlySingleQuotes(result)
	}

	return result
}

// applyCurlyDoubleQuotes rewrites straight double quotes using a simple opening/closing heuristic.
func applyCurlyDoubleQuotes(value string) string {
	return string(mapQuoteRunes([]rune(value), '"', leftDoubleCurlyQuote, rightDoubleCurlyQuote, false))
}

// applyCurlySingleQuotes rewrites straight single quotes while preserving apostrophes inside words.
func applyCurlySingleQuotes(value string) string {
	return string(mapQuoteRunes([]rune(value), '\'', leftSingleCurlyQuote, rightSingleCurlyQuote, true))
}

// mapQuoteRunes applies curly quotes of one kind based on a minimal opening/closing heuristic.
func mapQuoteRunes(chars []rune, target rune, opening rune, closing rune, allowApostrophe bool) []rune {
	result := make([]rune, 0, len(chars))
	for i, char := range chars {
		if char != target {
			result = append(result, char)
			continue
		}

		if allowApostrophe {
			prevIsLetter := i > 0 && unicode.IsLetter(chars[i-1])
			nextIsLetter := i < len(chars)-1 && unicode.IsLetter(chars[i+1])
			if prevIsLetter && nextIsLetter {
				result = append(result, closing)
				continue
			}
		}

		if isOpeningQuoteContext(chars, i) {
			result = append(result, opening)
			continue
		}

		result = append(result, closing)
	}

	return result
}

// isOpeningQuoteContext approximates whether a quote at the given index acts as an opening quote.
func isOpeningQuoteContext(chars []rune, index int) bool {
	if index == 0 {
		return true
	}

	switch chars[index-1] {
	case ' ', '\t', '\n', '\r', '(', '[', '{', '-', '—', '–':
		return true
	default:
		return false
	}
}

// indexRunes returns the first index at which needle appears in haystack, or -1 when absent.
func indexRunes(haystack []rune, needle []rune) int {
	if len(needle) == 0 {
		return 0
	}
	if len(needle) > len(haystack) {
		return -1
	}

outer:
	for start := 0; start <= len(haystack)-len(needle); start++ {
		for offset, char := range needle {
			if haystack[start+offset] != char {
				continue outer
			}
		}
		return start
	}

	return -1
}

// validateSettingsFileEdit keeps edits from breaking a settings file that was valid before the edit.
func validateSettingsFileEdit(filePath string, originalContent string, updatedContent string) error {
	if !isClaudeSettingsPath(filePath) {
		return nil
	}

	beforeValidation := platformconfig.ValidateSettingsContent(originalContent)
	if !beforeValidation.IsValid {
		return nil
	}
	afterValidation := platformconfig.ValidateSettingsContent(updatedContent)
	if !afterValidation.IsValid {
		return fmt.Errorf(
			"Claude Code settings.json validation failed after edit:\n%s\n\nFull schema:\n%s\nIMPORTANT: Do not update the env unless explicitly instructed to do so.",
			afterValidation.Error,
			afterValidation.FullSchema,
		)
	}

	return nil
}

// isClaudeSettingsPath returns whether the target path should receive settings-specific edit validation.
func isClaudeSettingsPath(filePath string) bool {
	normalized := strings.ToLower(filepath.Clean(filePath))
	claudeSettingsSuffix := strings.ToLower(filepath.Join(".claude", "settings.json"))
	claudeLocalSettingsSuffix := strings.ToLower(filepath.Join(".claude", "settings.local.json"))
	if strings.HasSuffix(normalized, claudeSettingsSuffix) || strings.HasSuffix(normalized, claudeLocalSettingsSuffix) {
		return true
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	globalSettingsPath := strings.ToLower(filepath.Clean(filepath.Join(homeDir, strings.TrimPrefix(platformconfig.GlobalConfigPath, "~/"))))
	return normalized == globalSettingsPath
}

// formatFileSize renders byte sizes with a small human-readable unit set for user-facing validation errors.
func formatFileSize(size int64) string {
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	value := float64(size)
	unitIndex := 0
	for value >= 1024 && unitIndex < len(units)-1 {
		value /= 1024
		unitIndex++
	}
	if unitIndex == 0 {
		return fmt.Sprintf("%d %s", size, units[unitIndex])
	}
	rounded := math.Round(value*10) / 10
	return fmt.Sprintf("%.1f %s", rounded, units[unitIndex])
}
