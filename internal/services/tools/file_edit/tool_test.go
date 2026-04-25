package file_edit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	platformfs "github.com/sheepzhao/claude-code-go/internal/platform/fs"
)

// TestToolInvokeReplacesSingleOccurrence verifies the first-batch FileEditTool replaces one unique match in place.
func TestToolInvokeReplacesSingleOccurrence(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "main.go")
	mustWriteFile(t, filePath, "package main\n\nfunc main() {}\n")
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	policy, err := newAllowWritePolicy(projectDir)
	if err != nil {
		t.Fatalf("newAllowWritePolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path":  "main.go",
			"old_string": "main()",
			"new_string": "run()",
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
			ReadState: coretool.ReadStateSnapshot{
				Files: map[string]coretool.ReadState{
					filePath: {
						ReadAt:          time.Unix(100, 0),
						ObservedModTime: info.ModTime(),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() result.Error = %q", result.Error)
	}
	if result.Output != "Updated file: main.go (1 replacement)" {
		t.Fatalf("Invoke() output = %q", result.Output)
	}

	updatedContent, readErr := os.ReadFile(filePath)
	if readErr != nil {
		t.Fatalf("ReadFile() error = %v", readErr)
	}
	if string(updatedContent) != "package main\n\nfunc run() {}\n" {
		t.Fatalf("updated content = %q", string(updatedContent))
	}

	data := result.Meta["data"].(Output)
	if data.Replacements != 1 || data.Content != string(updatedContent) {
		t.Fatalf("Invoke() metadata = %#v", data)
	}
	if len(data.StructuredPatch) != 1 {
		t.Fatalf("Invoke() structured patch = %#v, want one hunk", data.StructuredPatch)
	}
	if got := data.StructuredPatch[0].Lines; len(got) != 2 || got[0] != "-func main() {}" || got[1] != "+func run() {}" {
		t.Fatalf("Invoke() structured patch lines = %#v", got)
	}
}

// TestToolInvokeRequiresReplaceAllForAmbiguousMatch verifies repeated matches are rejected unless replace_all is enabled.
func TestToolInvokeRequiresReplaceAllForAmbiguousMatch(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "notes.txt")
	mustWriteFile(t, filePath, "foo\nfoo\n")
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	policy, err := newAllowWritePolicy(projectDir)
	if err != nil {
		t.Fatalf("newAllowWritePolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path":  "notes.txt",
			"old_string": "foo",
			"new_string": "bar",
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
			ReadState: coretool.ReadStateSnapshot{
				Files: map[string]coretool.ReadState{
					filePath: {
						ReadAt:          time.Unix(100, 0),
						ObservedModTime: info.ModTime(),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error == "" {
		t.Fatal("Invoke() result.Error = empty, want ambiguity error")
	}
}

// TestToolInvokeSupportsReplaceAll verifies callers can replace all exact matches when requested.
func TestToolInvokeSupportsReplaceAll(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "notes.txt")
	mustWriteFile(t, filePath, "foo\nfoo\n")
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	policy, err := newAllowWritePolicy(projectDir)
	if err != nil {
		t.Fatalf("newAllowWritePolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path":   "notes.txt",
			"old_string":  "foo",
			"new_string":  "bar",
			"replace_all": true,
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
			ReadState: coretool.ReadStateSnapshot{
				Files: map[string]coretool.ReadState{
					filePath: {
						ReadAt:          time.Unix(100, 0),
						ObservedModTime: info.ModTime(),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() result.Error = %q", result.Error)
	}

	data := result.Meta["data"].(Output)
	if data.Replacements != 2 {
		t.Fatalf("Invoke() replacements = %d, want 2", data.Replacements)
	}
	if len(data.StructuredPatch) != 1 {
		t.Fatalf("Invoke() structured patch = %#v, want one hunk", data.StructuredPatch)
	}
}

// TestToolInvokeMatchesCurlyQuotes verifies quote-only differences do not prevent a replacement.
func TestToolInvokeMatchesCurlyQuotes(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "notes.txt")
	mustWriteFile(t, filePath, "say “hello”\n")
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	policy, err := newAllowWritePolicy(projectDir)
	if err != nil {
		t.Fatalf("newAllowWritePolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path":  "notes.txt",
			"old_string": `say "hello"` + "\n",
			"new_string": `say "goodbye"` + "\n",
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
			ReadState: coretool.ReadStateSnapshot{
				Files: map[string]coretool.ReadState{
					filePath: {
						ReadAt:          time.Unix(100, 0),
						ObservedModTime: info.ModTime(),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() result.Error = %q", result.Error)
	}

	updatedContent, readErr := os.ReadFile(filePath)
	if readErr != nil {
		t.Fatalf("ReadFile() error = %v", readErr)
	}
	if string(updatedContent) != "say “goodbye”\n" {
		t.Fatalf("updated content = %q", string(updatedContent))
	}
}

// TestToolInvokePreservesCurlySingleQuotes verifies normalized matches keep single-quote typography in replacements.
func TestToolInvokePreservesCurlySingleQuotes(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "notes.txt")
	mustWriteFile(t, filePath, "it’s ‘fine’\n")
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	policy, err := newAllowWritePolicy(projectDir)
	if err != nil {
		t.Fatalf("newAllowWritePolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path":  "notes.txt",
			"old_string": "it's 'fine'\n",
			"new_string": "it's 'done'\n",
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
			ReadState: coretool.ReadStateSnapshot{
				Files: map[string]coretool.ReadState{
					filePath: {
						ReadAt:          time.Unix(100, 0),
						ObservedModTime: info.ModTime(),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() result.Error = %q", result.Error)
	}

	updatedContent, readErr := os.ReadFile(filePath)
	if readErr != nil {
		t.Fatalf("ReadFile() error = %v", readErr)
	}
	if string(updatedContent) != "it’s ‘done’\n" {
		t.Fatalf("updated content = %q", string(updatedContent))
	}
}

// TestToolInvokeCreatesFileWhenOldStringEmpty verifies empty old_string can create a missing file without prior read state.
func TestToolInvokeCreatesFileWhenOldStringEmpty(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "new.txt")

	policy, err := newAllowWritePolicy(projectDir)
	if err != nil {
		t.Fatalf("newAllowWritePolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path":  "new.txt",
			"old_string": "",
			"new_string": "created\n",
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() result.Error = %q", result.Error)
	}

	content, readErr := os.ReadFile(filePath)
	if readErr != nil {
		t.Fatalf("ReadFile() error = %v", readErr)
	}
	if string(content) != "created\n" {
		t.Fatalf("created content = %q", string(content))
	}

	data := result.Meta["data"].(Output)
	if data.OriginalContent != "" {
		t.Fatalf("Invoke() original content = %q, want empty", data.OriginalContent)
	}
	if data.Replacements != 1 {
		t.Fatalf("Invoke() replacements = %d, want 1", data.Replacements)
	}
}

// TestToolInvokeWritesIntoEmptyFileWhenOldStringEmpty verifies an existing empty file can be filled through the empty-old-string branch.
func TestToolInvokeWritesIntoEmptyFileWhenOldStringEmpty(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "empty.txt")
	mustWriteFile(t, filePath, "")

	policy, err := newAllowWritePolicy(projectDir)
	if err != nil {
		t.Fatalf("newAllowWritePolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path":  "empty.txt",
			"old_string": "",
			"new_string": "seed\n",
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() result.Error = %q", result.Error)
	}

	content, readErr := os.ReadFile(filePath)
	if readErr != nil {
		t.Fatalf("ReadFile() error = %v", readErr)
	}
	if string(content) != "seed\n" {
		t.Fatalf("updated content = %q", string(content))
	}
}

// TestToolInvokeRejectsCreateWhenTargetAlreadyHasContent verifies empty old_string does not overwrite a non-empty file.
func TestToolInvokeRejectsCreateWhenTargetAlreadyHasContent(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "notes.txt")
	mustWriteFile(t, filePath, "before\n")

	policy, err := newAllowWritePolicy(projectDir)
	if err != nil {
		t.Fatalf("newAllowWritePolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path":  "notes.txt",
			"old_string": "",
			"new_string": "after\n",
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != fileAlreadyExistsForCreateError {
		t.Fatalf("Invoke() result.Error = %q, want %q", result.Error, fileAlreadyExistsForCreateError)
	}

	content, readErr := os.ReadFile(filePath)
	if readErr != nil {
		t.Fatalf("ReadFile() error = %v", readErr)
	}
	if string(content) != "before\n" {
		t.Fatalf("file content = %q, want unchanged", string(content))
	}
}

// TestToolInvokeRejectsEditWithoutReadState verifies existing files must be read before in-place editing.
func TestToolInvokeRejectsEditWithoutReadState(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "notes.txt")
	mustWriteFile(t, filePath, "before\n")

	policy, err := newAllowWritePolicy(projectDir)
	if err != nil {
		t.Fatalf("newAllowWritePolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path":  "notes.txt",
			"old_string": "before",
			"new_string": "after",
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != unreadBeforeEditError {
		t.Fatalf("Invoke() result.Error = %q, want %q", result.Error, unreadBeforeEditError)
	}
}

// TestToolInvokeRejectsEditAfterPartialRead verifies partial reads do not satisfy the edit safety guard.
func TestToolInvokeRejectsEditAfterPartialRead(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "notes.txt")
	mustWriteFile(t, filePath, "before\n")

	policy, err := newAllowWritePolicy(projectDir)
	if err != nil {
		t.Fatalf("newAllowWritePolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path":  "notes.txt",
			"old_string": "before",
			"new_string": "after",
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
			ReadState: coretool.ReadStateSnapshot{
				Files: map[string]coretool.ReadState{
					filePath: {
						ReadAt:          time.Unix(100, 0),
						ObservedModTime: time.Unix(90, 0),
						IsPartial:       true,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != unreadBeforeEditError {
		t.Fatalf("Invoke() result.Error = %q, want %q", result.Error, unreadBeforeEditError)
	}
}

// TestToolInvokeRejectsEditAfterFileDrift verifies later file modifications invalidate an earlier full read.
func TestToolInvokeRejectsEditAfterFileDrift(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "notes.txt")
	mustWriteFile(t, filePath, "before\n")

	driftTime := time.Unix(200, 0)
	if err := os.Chtimes(filePath, driftTime, driftTime); err != nil {
		t.Fatalf("Chtimes() error = %v", err)
	}

	policy, err := newAllowWritePolicy(projectDir)
	if err != nil {
		t.Fatalf("newAllowWritePolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path":  "notes.txt",
			"old_string": "before",
			"new_string": "after",
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
			ReadState: coretool.ReadStateSnapshot{
				Files: map[string]coretool.ReadState{
					filePath: {
						ReadAt:          time.Unix(150, 0),
						ObservedModTime: time.Unix(100, 0),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != modifiedSinceReadError {
		t.Fatalf("Invoke() result.Error = %q, want %q", result.Error, modifiedSinceReadError)
	}
}

// TestToolInvokeRejectsNotebookFiles verifies .ipynb targets use the dedicated notebook rejection path.
func TestToolInvokeRejectsNotebookFiles(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "notes.ipynb")
	mustWriteFile(t, filePath, "{\"cells\":[]}\n")
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	policy, err := newAllowWritePolicy(projectDir)
	if err != nil {
		t.Fatalf("newAllowWritePolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path":  "notes.ipynb",
			"old_string": "[]",
			"new_string": "[1]",
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
			ReadState: coretool.ReadStateSnapshot{
				Files: map[string]coretool.ReadState{
					filePath: {
						ReadAt:          time.Unix(100, 0),
						ObservedModTime: info.ModTime(),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	want := "File is a Jupyter Notebook. Use the NotebookEdit to edit this file."
	if result.Error != want {
		t.Fatalf("Invoke() result.Error = %q, want %q", result.Error, want)
	}
}

// TestToolInvokeRejectsOversizedFiles verifies the byte-level size guard trips before reading large files.
func TestToolInvokeRejectsOversizedFiles(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "huge.txt")
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := file.Truncate(maxEditFileSize + 1); err != nil {
		_ = file.Close()
		t.Fatalf("Truncate() error = %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	policy, err := newAllowWritePolicy(projectDir)
	if err != nil {
		t.Fatalf("newAllowWritePolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path":  "huge.txt",
			"old_string": "a",
			"new_string": "b",
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
			ReadState: coretool.ReadStateSnapshot{
				Files: map[string]coretool.ReadState{
					filePath: {
						ReadAt:          time.Unix(100, 0),
						ObservedModTime: info.ModTime(),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !strings.Contains(result.Error, "File is too large to edit") {
		t.Fatalf("Invoke() result.Error = %q, want size-guard error", result.Error)
	}
}

// TestToolInvokeRejectsInvalidSettingsEdits verifies valid settings files cannot be edited into invalid JSON.
func TestToolInvokeRejectsInvalidSettingsEdits(t *testing.T) {
	projectDir := t.TempDir()
	settingsDir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	filePath := filepath.Join(settingsDir, "settings.json")
	mustWriteFile(t, filePath, "{\n  \"model\": \"sonnet\"\n}\n")
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	policy, err := newAllowWritePolicy(projectDir)
	if err != nil {
		t.Fatalf("newAllowWritePolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path":  ".claude/settings.json",
			"old_string": "\"sonnet\"",
			"new_string": "\"sonnet\",\n",
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
			ReadState: coretool.ReadStateSnapshot{
				Files: map[string]coretool.ReadState{
					filePath: {
						ReadAt:          time.Unix(100, 0),
						ObservedModTime: info.ModTime(),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !strings.Contains(result.Error, "Claude Code settings.json validation failed after edit") {
		t.Fatalf("Invoke() result.Error = %q, want settings validation error", result.Error)
	}
	if !strings.Contains(result.Error, "Full schema:") {
		t.Fatalf("Invoke() result.Error = %q, want fullSchema output", result.Error)
	}
}

// TestToolInvokeRejectsInvalidSettingsEditsWithEnv verifies env-enabled settings files still receive edit validation.
func TestToolInvokeRejectsInvalidSettingsEditsWithEnv(t *testing.T) {
	projectDir := t.TempDir()
	settingsDir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	filePath := filepath.Join(settingsDir, "settings.json")
	mustWriteFile(t, filePath, "{\n  \"model\": \"sonnet\",\n  \"env\": {\n    \"COUNT\": 1\n  }\n}\n")
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	policy, err := newAllowWritePolicy(projectDir)
	if err != nil {
		t.Fatalf("newAllowWritePolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path":  ".claude/settings.json",
			"old_string": "\"sonnet\"",
			"new_string": "\"sonnet\",\n",
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
			ReadState: coretool.ReadStateSnapshot{
				Files: map[string]coretool.ReadState{
					filePath: {
						ReadAt:          time.Unix(100, 0),
						ObservedModTime: info.ModTime(),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !strings.Contains(result.Error, "Claude Code settings.json validation failed after edit") {
		t.Fatalf("Invoke() result.Error = %q, want env-aware settings validation error", result.Error)
	}
}

// TestToolInvokeAllowsRepairingAlreadyInvalidSettings verifies edits can proceed when the original settings file is already invalid.
func TestToolInvokeAllowsRepairingAlreadyInvalidSettings(t *testing.T) {
	projectDir := t.TempDir()
	settingsDir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	filePath := filepath.Join(settingsDir, "settings.json")
	mustWriteFile(t, filePath, "{\n  \"model\":\n}\n")
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	policy, err := newAllowWritePolicy(projectDir)
	if err != nil {
		t.Fatalf("newAllowWritePolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path":  ".claude/settings.json",
			"old_string": "\"model\":\n",
			"new_string": "\"model\": \"sonnet\"\n",
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
			ReadState: coretool.ReadStateSnapshot{
				Files: map[string]coretool.ReadState{
					filePath: {
						ReadAt:          time.Unix(100, 0),
						ObservedModTime: info.ModTime(),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Invoke() result.Error = %q", result.Error)
	}
}

// TestToolInvokeRejectsSettingsTypeMismatch verifies FileEditTool now reuses the migrated settings schema validator.
func TestToolInvokeRejectsSettingsTypeMismatch(t *testing.T) {
	projectDir := t.TempDir()
	settingsDir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	filePath := filepath.Join(settingsDir, "settings.json")
	mustWriteFile(t, filePath, "{\n  \"model\": \"sonnet\"\n}\n")
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	policy, err := newAllowWritePolicy(projectDir)
	if err != nil {
		t.Fatalf("newAllowWritePolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path":  ".claude/settings.json",
			"old_string": "\"sonnet\"",
			"new_string": "123",
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
			ReadState: coretool.ReadStateSnapshot{
				Files: map[string]coretool.ReadState{
					filePath: {
						ReadAt:          time.Unix(100, 0),
						ObservedModTime: info.ModTime(),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !strings.Contains(result.Error, "- model: Expected string, but received number") {
		t.Fatalf("Invoke() result.Error = %q, want schema type mismatch", result.Error)
	}
}

// TestToolInvokeRejectsInvalidExpandedSettingsFields verifies FileEditTool reuses expanded batch-06 settings validation.
func TestToolInvokeRejectsInvalidExpandedSettingsFields(t *testing.T) {
	projectDir := t.TempDir()
	settingsDir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	filePath := filepath.Join(settingsDir, "settings.json")
	mustWriteFile(t, filePath, "{\n  \"defaultShell\": \"bash\"\n}\n")
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	policy, err := newAllowWritePolicy(projectDir)
	if err != nil {
		t.Fatalf("newAllowWritePolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path":  ".claude/settings.json",
			"old_string": "\"bash\"",
			"new_string": "\"zsh\"",
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
			ReadState: coretool.ReadStateSnapshot{
				Files: map[string]coretool.ReadState{
					filePath: {
						ReadAt:          time.Unix(100, 0),
						ObservedModTime: info.ModTime(),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !strings.Contains(result.Error, "- defaultShell: Invalid value. Expected one of: \"bash\", \"powershell\"") {
		t.Fatalf("Invoke() result.Error = %q, want defaultShell enum error", result.Error)
	}
	if !strings.Contains(result.Error, "Full schema:") {
		t.Fatalf("Invoke() result.Error = %q, want fullSchema output", result.Error)
	}
}

// TestToolInvokeRejectsWriteWithoutPermission verifies the migrated tool reuses the minimal write-permission gate.
func TestToolInvokeRejectsWriteWithoutPermission(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "notes.txt")
	mustWriteFile(t, filePath, "before\n")

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	_, err = tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path":  "notes.txt",
			"old_string": "before",
			"new_string": "after",
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
		},
	})
	if err == nil {
		t.Fatalf("Invoke() error = nil, want permission error")
	}

	var permissionErr *corepermission.PermissionError
	if !errors.As(err, &permissionErr) {
		t.Fatalf("Invoke() error = %T, want *PermissionError", err)
	}
	if permissionErr.Decision != corepermission.DecisionAsk {
		t.Fatalf("Invoke() decision = %q, want %q", permissionErr.Decision, corepermission.DecisionAsk)
	}
}

// TestToolInvokeRejectsMissingString verifies file drift or mismatched expectations are rejected without writing.
func TestToolInvokeRejectsMissingString(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "notes.txt")
	mustWriteFile(t, filePath, "before\n")
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	policy, err := newAllowWritePolicy(projectDir)
	if err != nil {
		t.Fatalf("newAllowWritePolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path":  "notes.txt",
			"old_string": "missing",
			"new_string": "after",
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
			ReadState: coretool.ReadStateSnapshot{
				Files: map[string]coretool.ReadState{
					filePath: {
						ReadAt:          time.Unix(100, 0),
						ObservedModTime: info.ModTime(),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Error == "" {
		t.Fatal("Invoke() result.Error = empty, want missing string error")
	}
}

// TestToolInvokeRejectsTeamMemorySecrets verifies team memory edits with secrets are blocked before writing.
func TestToolInvokeRejectsTeamMemorySecrets(t *testing.T) {
	projectDir := t.TempDir()
	teamMemoryPath := filepath.Join(projectDir, "projects", "demo", "memory", "team", "MEMORY.md")
	if err := os.MkdirAll(filepath.Dir(teamMemoryPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	mustWriteFile(t, teamMemoryPath, "before\n")

	policy, err := newAllowWritePolicy(projectDir)
	if err != nil {
		t.Fatalf("newAllowWritePolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)

	result, err := tool.Invoke(context.Background(), coretool.Call{
		Name: Name,
		Input: map[string]any{
			"file_path":  teamMemoryPath,
			"old_string": "before",
			"new_string": "ghp_" + strings.Repeat("a", 36),
		},
		Context: coretool.UseContext{
			WorkingDir: projectDir,
			ReadState: coretool.ReadStateSnapshot{
				Files: map[string]coretool.ReadState{
					teamMemoryPath: {
						ReadAt:          time.Unix(100, 0),
						ObservedModTime: time.Unix(100, 0),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !strings.Contains(result.Error, "potential secrets") {
		t.Fatalf("Invoke() result.Error = %q, want team memory secret rejection", result.Error)
	}
}

// newAllowWritePolicy constructs a minimal policy that allows writes inside one workspace.
func newAllowWritePolicy(workspace string) (*corepermission.FilesystemPolicy, error) {
	return corepermission.NewFilesystemPolicy(corepermission.RuleSet{
		Write: []corepermission.Rule{
			{
				Source:   corepermission.RuleSourceUserSettings,
				Decision: corepermission.DecisionAllow,
				BaseDir:  workspace,
				Pattern:  "**",
			},
		},
	})
}

// mustWriteFile creates a file and fails the test immediately on errors to keep setup terse.
func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
