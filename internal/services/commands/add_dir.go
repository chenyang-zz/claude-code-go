package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	platformfs "github.com/sheepzhao/claude-code-go/internal/platform/fs"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// AdditionalDirectoryStore persists project-scoped additional working directories for slash commands.
type AdditionalDirectoryStore interface {
	// AddAdditionalDirectory writes one extra working directory into project settings.
	AddAdditionalDirectory(ctx context.Context, directory string) error
}

// AddDirCommand exposes the minimum text-only /add-dir flow before the interactive picker exists in the Go host.
type AddDirCommand struct {
	// Config carries the resolved runtime configuration snapshot used to derive current working-directory coverage.
	Config *coreconfig.Config
	// Store persists extra working directories into project settings.
	Store AdditionalDirectoryStore
	// Policy widens the active read permission roots so the new directory becomes usable immediately.
	Policy *corepermission.FilesystemPolicy
}

// Metadata returns the canonical slash descriptor for /add-dir.
func (c AddDirCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "add-dir",
		Description: "Add a new working directory",
		Usage:       "/add-dir <path>",
	}
}

// Execute validates one explicit directory path, persists it into project settings, and widens the active read scope.
func (c AddDirCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	requested := strings.TrimSpace(args.RawLine)
	if requested == "" {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}
	if c.Store == nil {
		return command.Result{}, fmt.Errorf("project settings storage is not configured")
	}

	projectPath := ""
	if c.Config != nil {
		projectPath = c.Config.ProjectPath
	}

	absolutePath, err := platformfs.ExpandPath(requested, projectPath)
	if err != nil {
		return command.Result{}, fmt.Errorf("expand add-dir path %q: %w", requested, err)
	}

	info, err := os.Stat(absolutePath)
	if err != nil {
		if os.IsNotExist(err) {
			return command.Result{}, fmt.Errorf("path %s was not found", absolutePath)
		}
		return command.Result{}, fmt.Errorf("stat add-dir path %s: %w", absolutePath, err)
	}
	if !info.IsDir() {
		return command.Result{}, fmt.Errorf("%s is not a directory. Did you mean to add the parent directory %s?", requested, filepath.Dir(absolutePath))
	}

	workingRoots, err := addDirWorkingRoots(projectPath, c.Config)
	if err != nil {
		return command.Result{}, err
	}
	for _, root := range workingRoots {
		if addDirPathWithinRoot(root, absolutePath) {
			return command.Result{}, fmt.Errorf("%s is already accessible within the existing working directory %s", requested, root)
		}
	}

	if err := c.Store.AddAdditionalDirectory(ctx, absolutePath); err != nil {
		return command.Result{}, err
	}

	if c.Config != nil && !addDirContainsString(c.Config.Permissions.AdditionalDirectories, absolutePath) {
		c.Config.Permissions.AdditionalDirectories = append(c.Config.Permissions.AdditionalDirectories, absolutePath)
	}
	if c.Policy != nil {
		c.Policy.AddReadRoot(absolutePath)
	}

	logger.DebugCF("commands", "added working directory via add-dir command", map[string]any{
		"directory":                   absolutePath,
		"project_path":                projectPath,
		"additional_directory_count":  len(c.currentAdditionalDirectories()),
		"policy_read_root_configured": c.Policy != nil,
	})

	return command.Result{
		Output: fmt.Sprintf("Added %s as a working directory. Claude Code Go persists it to project settings now, but the interactive add-dir flow and session-only directory mode are not implemented yet.", absolutePath),
	}, nil
}

func (c AddDirCommand) currentAdditionalDirectories() []string {
	if c.Config == nil {
		return nil
	}
	return c.Config.Permissions.AdditionalDirectories
}

func addDirWorkingRoots(projectPath string, cfg *coreconfig.Config) ([]string, error) {
	roots := make([]string, 0, 1)
	if strings.TrimSpace(projectPath) != "" {
		roots = append(roots, filepath.Clean(projectPath))
	}
	if cfg == nil {
		return roots, nil
	}

	for _, configured := range cfg.Permissions.AdditionalDirectories {
		expanded, err := platformfs.ExpandPath(configured, projectPath)
		if err != nil {
			return nil, fmt.Errorf("expand configured additional directory %q: %w", configured, err)
		}
		if addDirContainsString(roots, expanded) {
			continue
		}
		roots = append(roots, expanded)
	}
	return roots, nil
}

func addDirPathWithinRoot(root string, target string) bool {
	relativePath, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	if relativePath == "." {
		return true
	}
	if relativePath == ".." {
		return false
	}
	return !strings.HasPrefix(relativePath, ".."+string(filepath.Separator))
}

func addDirContainsString(values []string, target string) bool {
	return slices.Contains(values, target)
}
