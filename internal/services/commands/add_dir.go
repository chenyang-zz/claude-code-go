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

// AddDirDestination records whether one added directory should live only in-memory or be remembered in local settings.
type AddDirDestination string

const (
	// AddDirDestinationSession keeps the directory only for the current process session.
	AddDirDestinationSession AddDirDestination = "session"
	// AddDirDestinationLocalSettings persists the directory into `.claude/settings.local.json`.
	AddDirDestinationLocalSettings AddDirDestination = "localSettings"
)

// AdditionalDirectoryStore persists remembered working directories for slash commands.
type AdditionalDirectoryStore interface {
	// AddAdditionalDirectory writes one extra working directory into the configured settings destination.
	AddAdditionalDirectory(ctx context.Context, directory string) error
}

// AddDirCommand exposes the minimum text-only `/add-dir` behavior and shared directory-application helpers.
type AddDirCommand struct {
	// Config carries the resolved runtime configuration snapshot used to derive current working-directory coverage.
	Config *coreconfig.Config
	// LocalStore persists remembered directories into local settings.
	LocalStore AdditionalDirectoryStore
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
	absolutePath, err := c.ResolveDirectory(requested)
	if err != nil {
		return command.Result{}, err
	}
	return c.ApplyDirectory(ctx, absolutePath, AddDirDestinationLocalSettings)
}

// ResolveDirectory validates one requested directory path and returns the stable absolute path used for later permission updates.
func (c AddDirCommand) ResolveDirectory(requested string) (string, error) {
	projectPath := ""
	if c.Config != nil {
		projectPath = c.Config.ProjectPath
	}

	absolutePath, err := platformfs.ExpandPath(requested, projectPath)
	if err != nil {
		return "", fmt.Errorf("expand add-dir path %q: %w", requested, err)
	}

	info, err := os.Stat(absolutePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("path %s was not found", absolutePath)
		}
		return "", fmt.Errorf("stat add-dir path %s: %w", absolutePath, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory. Did you mean to add the parent directory %s?", requested, filepath.Dir(absolutePath))
	}

	workingRoots, err := addDirWorkingRoots(projectPath, c.Config)
	if err != nil {
		return "", err
	}
	for _, root := range workingRoots {
		if addDirPathWithinRoot(root, absolutePath) {
			return "", fmt.Errorf("%s is already accessible within the existing working directory %s", requested, root)
		}
	}
	return absolutePath, nil
}

// ApplyDirectory expands the active permission scope and optionally persists the directory based on the chosen destination.
func (c AddDirCommand) ApplyDirectory(ctx context.Context, absolutePath string, destination AddDirDestination) (command.Result, error) {
	projectPath := ""
	directorySource := coreconfig.AdditionalDirectorySourceSession
	if c.Config != nil {
		projectPath = c.Config.ProjectPath
	}

	switch destination {
	case AddDirDestinationSession:
		// Session-only directories intentionally stay in-memory.
	case AddDirDestinationLocalSettings:
		directorySource = coreconfig.AdditionalDirectorySourceLocalSettings
		if c.LocalStore == nil {
			return command.Result{}, fmt.Errorf("local settings storage is not configured")
		}
		if err := c.LocalStore.AddAdditionalDirectory(ctx, absolutePath); err != nil {
			return command.Result{}, err
		}
	default:
		return command.Result{}, fmt.Errorf("unsupported add-dir destination %q", destination)
	}

	if c.Config != nil {
		addDirUpsertAdditionalDirectory(&c.Config.Permissions, absolutePath, directorySource)
	}
	if c.Policy != nil {
		c.Policy.AddReadRoot(absolutePath)
	}

	logger.DebugCF("commands", "added working directory via add-dir command", map[string]any{
		"directory":                   absolutePath,
		"project_path":                projectPath,
		"additional_directory_count":  len(c.currentAdditionalDirectories()),
		"destination":                 string(destination),
		"policy_read_root_configured": c.Policy != nil,
	})

	message := fmt.Sprintf("Added %s as a working directory for this session. Use /permissions to review the active workspace scope.", absolutePath)
	if destination == AddDirDestinationLocalSettings {
		message = fmt.Sprintf("Added %s as a working directory and saved it to local settings. Use /permissions to review the active workspace scope.", absolutePath)
	}
	return command.Result{
		Output: message,
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

	configuredDirectories := cfg.Permissions.AdditionalDirectories
	if len(cfg.Permissions.AdditionalDirectoryEntries) > 0 {
		configuredDirectories = coreconfig.AdditionalDirectoryPaths(cfg.Permissions.AdditionalDirectoryEntries)
	}
	for _, configured := range configuredDirectories {
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

// addDirUpsertAdditionalDirectory keeps the legacy path list and the sourced directory list in sync.
func addDirUpsertAdditionalDirectory(cfg *coreconfig.PermissionConfig, path string, source coreconfig.AdditionalDirectorySource) {
	if cfg == nil || addDirContainsString(cfg.AdditionalDirectories, path) {
		return
	}

	cfg.AdditionalDirectories = append(cfg.AdditionalDirectories, path)
	cfg.AdditionalDirectoryEntries = append(cfg.AdditionalDirectoryEntries, coreconfig.AdditionalDirectoryConfig{
		Path:   path,
		Source: source,
	})
}
