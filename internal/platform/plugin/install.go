package plugin

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// gitCommand is the git executable path, exposed as a variable so tests can
// override it with a mock implementation.
var gitCommand = "git"

// InstallOptions configures a plugin installation operation.
type InstallOptions struct {
	// Scope is the installation scope: "user" or "project".
	// "user" installs to ~/.claude/plugins/.
	// "project" installs to .claude/plugins/ under ProjectDir.
	Scope string
	// ProjectDir is required when Scope is "project".
	ProjectDir string
}

// InstallResult holds the outcome of a successful plugin installation.
type InstallResult struct {
	// Plugin is the loaded plugin metadata after installation.
	Plugin LoadedPlugin
	// InstallPath is the absolute filesystem path where the plugin was installed.
	InstallPath string
}

// InstallPlugin installs a plugin from the given source configuration.
//
// It dispatches based on the source type:
//   - SourceTypePath: copies a local directory to the plugins directory
//   - SourceTypeGit: clones a git repository to the plugins directory
//
// Other source types (npm, github, builtin) are not yet supported and return
// an error.
//
// The installation flow for each source type is:
//  1. Pre-read the manifest to determine the plugin name
//  2. Compute the target install path
//  3. Acquire the plugin files (copy or clone)
//  4. Validate the installed plugin
//  5. Build and return the LoadedPlugin
func InstallPlugin(source PluginSource, opts InstallOptions) (*InstallResult, error) {
	if strings.TrimSpace(opts.Scope) == "" {
		opts.Scope = "user"
	}

	var result *InstallResult

	switch source.Type {
	case SourceTypePath:
		res, err := installFromPath(source, opts)
		if err != nil {
			return nil, err
		}
		result = res
	case SourceTypeGit:
		res, err := installFromGit(source, opts)
		if err != nil {
			return nil, err
		}
		result = res
	default:
		return nil, fmt.Errorf("plugin source type %q is not yet supported for installation", source.Type)
	}

	logger.InfoCF("plugin.install", "plugin installed", map[string]any{
		"name": result.Plugin.Name,
		"path": result.InstallPath,
	})
	return result, nil
}

// computeInstallPath returns the directory where a plugin should be installed.
//
// For "user" scope: ~/.claude/plugins/{plugin-name}/
// For "project" scope: {projectDir}/.claude/plugins/{plugin-name}/
//
// If the target directory already exists and is non-empty, an error is returned
// to prevent accidental overwrites.
func computeInstallPath(pluginName string, opts InstallOptions) (string, error) {
	if strings.TrimSpace(pluginName) == "" {
		return "", fmt.Errorf("plugin name is required to compute install path")
	}

	var baseDir string
	switch opts.Scope {
	case "user":
		baseDir = GetPluginsDir()
	case "project":
		if strings.TrimSpace(opts.ProjectDir) == "" {
			return "", fmt.Errorf("ProjectDir is required for project-scoped plugin installation")
		}
		baseDir = filepath.Join(opts.ProjectDir, ".claude", "plugins")
	default:
		return "", fmt.Errorf("unknown installation scope %q", opts.Scope)
	}

	targetDir := filepath.Join(baseDir, pluginName)

	// Check for existing directory to avoid accidental overwrites.
	info, err := os.Stat(targetDir)
	if err == nil {
		if info.IsDir() {
			// Check if the directory is non-empty.
			entries, readErr := os.ReadDir(targetDir)
			if readErr == nil && len(entries) > 0 {
				return "", fmt.Errorf("plugin directory %s already exists and is not empty", targetDir)
			}
		} else {
			return "", fmt.Errorf("plugin target path %s already exists and is not a directory", targetDir)
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to check plugin target directory: %w", err)
	}

	return targetDir, nil
}

// installFromPath handles installation when the source is a local directory.
func installFromPath(source PluginSource, opts InstallOptions) (*InstallResult, error) {
	sourcePath := source.Value
	if strings.TrimSpace(sourcePath) == "" {
		return nil, fmt.Errorf("path source requires a non-empty value")
	}

	// Resolve to an absolute path for reliability.
	absSource, err := filepath.Abs(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve source path: %w", err)
	}

	// Verify the source directory exists.
	srcInfo, err := os.Stat(absSource)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("source plugin directory %s does not exist", absSource)
		}
		return nil, fmt.Errorf("failed to access source plugin directory: %w", err)
	}
	if !srcInfo.IsDir() {
		return nil, fmt.Errorf("source path %s is not a directory", absSource)
	}

	// Pre-read the manifest from the source directory to determine the plugin name.
	manifest, err := LoadManifest(absSource)
	if err != nil {
		return nil, fmt.Errorf("failed to load plugin manifest from source: %w", err)
	}

	// Compute the target install path using the manifest name.
	targetDir, err := computeInstallPath(manifest.Name, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to compute install path: %w", err)
	}

	// Copy the source directory to the target.
	if err := acquireFromPath(absSource, targetDir); err != nil {
		return nil, fmt.Errorf("failed to copy plugin from source: %w", err)
	}

	return buildInstallResult(targetDir, source, manifest)
}

// installFromGit handles installation when the source is a git repository URL.
func installFromGit(source PluginSource, opts InstallOptions) (*InstallResult, error) {
	gitURL := source.Value
	if strings.TrimSpace(gitURL) == "" {
		return nil, fmt.Errorf("git source requires a non-empty value")
	}

	// Clone to a temporary directory first so we can read the manifest
	// before moving to the final location.
	tmpDir, err := os.MkdirTemp("", "claude-plugin-git-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory for git clone: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := acquireFromGit(gitURL, source.Version, tmpDir); err != nil {
		return nil, fmt.Errorf("failed to clone plugin from git: %w", err)
	}

	// Read the manifest from the cloned repo.
	manifest, err := LoadManifest(tmpDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load plugin manifest from cloned repo: %w", err)
	}

	// Compute the final install path.
	targetDir, err := computeInstallPath(manifest.Name, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to compute install path: %w", err)
	}

	// Move from temp to final location.
	if err := os.Rename(tmpDir, targetDir); err != nil {
		// If rename fails (e.g., cross-device), fall back to copy.
		if err := acquireFromPath(tmpDir, targetDir); err != nil {
			return nil, fmt.Errorf("failed to move plugin to final location: %w", err)
		}
	}

	return buildInstallResult(targetDir, source, manifest)
}

// acquireFromPath recursively copies a directory from src to dst.
//
// It creates the destination directory if it does not exist, copies all files
// and subdirectories, and preserves file permissions. Symbolic links are
// skipped with a warning to prevent security issues.
func acquireFromPath(src, dst string) error {
	// Ensure the destination directory exists.
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := acquireFromPath(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			// Skip symbolic links for security.
			info, statErr := os.Lstat(srcPath)
			if statErr != nil {
				return fmt.Errorf("failed to stat %s: %w", srcPath, statErr)
			}
			if info.Mode()&os.ModeSymlink != 0 {
				logger.DebugCF("plugin.install", "skipping symbolic link", map[string]any{
					"path": srcPath,
				})
				continue
			}

			if err := copyFile(srcPath, dstPath); err != nil {
				return fmt.Errorf("failed to copy %s: %w", srcPath, err)
			}
		}
	}

	return nil
}

// copyFile copies a single file from src to dst, creating the destination file
// with the same permissions as the source.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, data, srcInfo.Mode())
}

// acquireFromGit clones a git repository URL into the destination directory
// using the git CLI. The destination directory must not already exist.
//
// If a version (tag, branch, or commit) is specified, it is passed to
// git clone --branch. A shallow clone (--depth 1) is used for efficiency.
func acquireFromGit(gitURL, version, dst string) error {
	args := []string{"clone", "--depth", "1"}
	if version != "" {
		args = append(args, "--branch", version)
	}
	args = append(args, gitURL, dst)

	cmd := exec.Command(gitCommand, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %w\n%s", err, string(output))
	}

	return nil
}

// buildInstallResult creates an InstallResult from the installed plugin path,
// source, and manifest. It validates the installation before returning.
func buildInstallResult(installPath string, source PluginSource, manifest *PluginManifest) (*InstallResult, error) {
	// Validate the installed plugin.
	if err := ValidateInstalledPlugin(installPath); err != nil {
		return nil, fmt.Errorf("installed plugin validation failed: %w", err)
	}

	// Resolve the absolute path for consistency.
	absPath, err := filepath.Abs(installPath)
	if err != nil {
		absPath = installPath
	}

	return &InstallResult{
		Plugin: LoadedPlugin{
			Name:     manifest.Name,
			Manifest: *manifest,
			Path:     absPath,
			Source:   source,
			Enabled:  true,
		},
		InstallPath: absPath,
	}, nil
}
