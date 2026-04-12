package commands

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
)

type recordingAdditionalDirectoryStore struct {
	saved []string
}

func (s *recordingAdditionalDirectoryStore) AddAdditionalDirectory(ctx context.Context, directory string) error {
	_ = ctx
	s.saved = append(s.saved, directory)
	return nil
}

// TestAddDirCommandMetadata verifies /add-dir exposes the migrated descriptor.
func TestAddDirCommandMetadata(t *testing.T) {
	got := AddDirCommand{}.Metadata()
	want := command.Metadata{
		Name:        "add-dir",
		Description: "Add a new working directory",
		Usage:       "/add-dir <path>",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Metadata() = %#v, want %#v", got, want)
	}
}

// TestAddDirCommandExecutePersistsDirectory verifies /add-dir validates and persists one explicit directory while widening read scope.
func TestAddDirCommandExecutePersistsDirectory(t *testing.T) {
	rootDir := t.TempDir()
	projectDir := filepath.Join(rootDir, "project")
	extraDir := filepath.Join(rootDir, "shared", "app")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll(extraDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	store := &recordingAdditionalDirectoryStore{}
	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}
	cfg := &coreconfig.Config{ProjectPath: projectDir}

	result, err := AddDirCommand{
		Config: cfg,
		Store:  store,
		Policy: policy,
	}.Execute(context.Background(), command.Args{RawLine: "../shared/app"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	wantPath := extraDir
	wantOutput := "Added " + wantPath + " as a working directory. Claude Code Go persists it to project settings now, but the interactive add-dir flow and session-only directory mode are not implemented yet."
	if result.Output != wantOutput {
		t.Fatalf("Execute() output = %q, want %q", result.Output, wantOutput)
	}
	if !reflect.DeepEqual(store.saved, []string{wantPath}) {
		t.Fatalf("saved directories = %#v, want %#v", store.saved, []string{wantPath})
	}
	if !reflect.DeepEqual(cfg.Permissions.AdditionalDirectories, []string{wantPath}) {
		t.Fatalf("config additional directories = %#v, want %#v", cfg.Permissions.AdditionalDirectories, []string{wantPath})
	}
	evaluation := policy.CheckReadPermissionForTool(context.Background(), "file_read", filepath.Join(wantPath, "README.md"), projectDir)
	if evaluation.Decision != corepermission.DecisionAllow {
		t.Fatalf("policy decision = %q, want allow", evaluation.Decision)
	}
}

// TestAddDirCommandExecuteRejectsMissingPath verifies the minimum text flow requires one explicit path argument.
func TestAddDirCommandExecuteRejectsMissingPath(t *testing.T) {
	_, err := AddDirCommand{}.Execute(context.Background(), command.Args{})
	if err == nil {
		t.Fatal("Execute() error = nil, want usage error")
	}
	if err.Error() != "usage: /add-dir <path>" {
		t.Fatalf("Execute() error = %q, want usage", err.Error())
	}
}

// TestAddDirCommandExecuteRejectsAlreadyAccessibleDirectory verifies /add-dir does not duplicate the workspace root or an existing extra directory.
func TestAddDirCommandExecuteRejectsAlreadyAccessibleDirectory(t *testing.T) {
	projectDir := t.TempDir()
	extraDir := filepath.Join(projectDir, "docs")
	if err := os.MkdirAll(extraDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	_, err := AddDirCommand{
		Config: &coreconfig.Config{
			ProjectPath: projectDir,
			Permissions: coreconfig.PermissionConfig{
				AdditionalDirectories: []string{"docs"},
			},
		},
		Store: &recordingAdditionalDirectoryStore{},
	}.Execute(context.Background(), command.Args{RawLine: "docs"})
	if err == nil {
		t.Fatal("Execute() error = nil, want already accessible error")
	}
	if !strings.Contains(err.Error(), "is already accessible within the existing working directory") {
		t.Fatalf("Execute() error = %q, want already accessible message", err.Error())
	}
}

// TestAddDirCommandExecuteRejectsFilePath verifies /add-dir preserves the stable parent-directory hint for non-directory paths.
func TestAddDirCommandExecuteRejectsFilePath(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "README.md")
	if err := os.WriteFile(filePath, []byte("hi"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := AddDirCommand{
		Config: &coreconfig.Config{ProjectPath: projectDir},
		Store:  &recordingAdditionalDirectoryStore{},
	}.Execute(context.Background(), command.Args{RawLine: "README.md"})
	if err == nil {
		t.Fatal("Execute() error = nil, want non-directory error")
	}
	want := "README.md is not a directory. Did you mean to add the parent directory " + projectDir + "?"
	if err.Error() != want {
		t.Fatalf("Execute() error = %q, want %q", err.Error(), want)
	}
}
