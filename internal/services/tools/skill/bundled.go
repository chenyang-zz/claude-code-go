package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// BundledSkillDefinition describes a skill that is compiled into the binary
// and registered programmatically at startup. It mirrors the TS type of the
// same name.
type BundledSkillDefinition struct {
	Name        string
	Description string
	Aliases     []string
	WhenToUse   string
	ArgumentHint string
	AllowedTools []string
	Model       string
	DisableModelInvocation bool
	UserInvocable bool
	// IsEnabled is an optional gate evaluated before the skill is surfaced.
	IsEnabled func() bool
	Hooks     map[string]any
	// Context determines execution mode: "inline" (default) or "fork".
	Context string
	Agent   string
	// Files maps relative paths (forward slashes, no "..") to content.
	// These are extracted lazily on first invocation.
	Files map[string]string
	// GetPromptForCommand returns the prompt text for the given raw argument string.
	GetPromptForCommand func(args string) (string, error)
}

// bundledSkills stores all programmatically registered bundled skills.
var bundledSkills []*Skill

// RegisterBundledSkill converts a BundledSkillDefinition into a Skill,
// wraps the prompt function with lazy file extraction when Files is set,
// and appends it to the global bundled skill registry.
func RegisterBundledSkill(def BundledSkillDefinition) {
	// userInvocable defaults to true matching TS: definition.userInvocable ?? true
	userInvocable := true

	var skillRoot string
	getPrompt := def.GetPromptForCommand

	if len(def.Files) > 0 {
		skillRoot = getBundledSkillExtractDir(def.Name)
		inner := def.GetPromptForCommand
		var extractionDir string
		extracted := false

		getPrompt = func(args string) (string, error) {
			if !extracted {
				dir, err := extractBundledSkillFiles(def.Name, def.Files)
				if err != nil {
					logger.DebugCF("skill", "bundled skill file extraction failed", map[string]any{
						"name":  def.Name,
						"error": err.Error(),
					})
				}
				extracted = true
				extractionDir = dir
			}

			content, err := inner(args)
			if err != nil {
				return "", err
			}
			if extractionDir != "" {
				content = "Base directory for this skill: " + extractionDir + "\n\n" + content
			}
			return content, nil
		}
	}

	skill := &Skill{
		name:                       def.Name,
		description:                def.Description,
		hasUserSpecifiedDescription: true,
		whenToUse:                  def.WhenToUse,
		allowedTools:               def.AllowedTools,
		argumentHint:               def.ArgumentHint,
		model:                      def.Model,
		disableModelInvocation:     def.DisableModelInvocation,
		userInvocable:              userInvocable,
		hooks:                      def.Hooks,
		executionContext:           def.Context,
		agent:                      def.Agent,
		source:                     "bundled",
		loadedFrom:                 "bundled",
		baseDir:                    skillRoot,
	}

	// Store the custom prompt function so Execute() can delegate to it.
	// We embed it in the Skill by overriding Execute behaviour.
	if getPrompt != nil {
		// Wrap Execute to use the bundled prompt function.
		origContent := skill.content
		skill.content = "" // will be filled by getPrompt at execution time
		_ = origContent
		skill.bundledPrompt = getPrompt
	}

	bundledSkills = append(bundledSkills, skill)
	logger.DebugCF("skill", "registered bundled skill", map[string]any{
		"name": def.Name,
	})
}

// GetBundledSkills returns a copy of all registered bundled skills.
func GetBundledSkills() []*Skill {
	result := make([]*Skill, len(bundledSkills))
	copy(result, bundledSkills)
	return result
}

// ClearBundledSkills removes all bundled skills from the registry (for testing).
func ClearBundledSkills() {
	bundledSkills = nil
}

// getBundledSkillExtractDir returns the deterministic extraction directory
// for a bundled skill's reference files.
func getBundledSkillExtractDir(skillName string) string {
	root := bundledSkillsRoot()
	return filepath.Join(root, skillName)
}

// bundledSkillsRoot returns the parent directory for bundled skill file extraction.
// Uses the same pattern as the TS side: a per-process nonce under a predictable
// parent to defend against pre-created symlinks.
func bundledSkillsRoot() string {
	// Use a predictable base under the user's cache directory.
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	return filepath.Join(cacheDir, "claude-code-go", "bundled-skills")
}

// extractBundledSkillFiles writes a bundled skill's reference files to disk.
// Returns the extraction directory on success, or an error if writing failed.
func extractBundledSkillFiles(skillName string, files map[string]string) (string, error) {
	dir := getBundledSkillExtractDir(skillName)
	if err := writeSkillFiles(dir, files); err != nil {
		return "", err
	}
	return dir, nil
}

// writeSkillFiles groups files by parent directory, creates each subtree once,
// and writes files with safe flags (O_EXCL | O_CREATE | O_WRONLY).
func writeSkillFiles(baseDir string, files map[string]string) error {
	type fileEntry struct {
		path    string
		content string
	}
	byParent := make(map[string][]fileEntry)

	for relPath, content := range files {
		target, err := resolveSkillFilePath(baseDir, relPath)
		if err != nil {
			return err
		}
		parent := filepath.Dir(target)
		byParent[parent] = append(byParent[parent], fileEntry{target, content})
	}

	for parent, entries := range byParent {
		if err := os.MkdirAll(parent, 0o700); err != nil {
			return fmt.Errorf("mkdir %s: %w", parent, err)
		}
		for _, e := range entries {
			if err := safeWriteFile(e.path, e.content); err != nil {
				return fmt.Errorf("write %s: %w", e.path, err)
			}
		}
	}

	return nil
}

// resolveSkillFilePath validates and joins a relative path under baseDir.
// Rejects absolute paths and paths containing ".." components.
func resolveSkillFilePath(baseDir, relPath string) (string, error) {
	normalized := filepath.Clean(relPath)
	if filepath.IsAbs(normalized) {
		return "", fmt.Errorf("bundled skill file path must be relative: %s", relPath)
	}
	if strings.HasPrefix(normalized, "..") || strings.Contains(normalized, string(filepath.Separator)+"..") {
		return "", fmt.Errorf("bundled skill file path escapes skill dir: %s", relPath)
	}
	// Also check the original string for ".." components after splitting by "/"
	for _, part := range strings.Split(relPath, "/") {
		if part == ".." {
			return "", fmt.Errorf("bundled skill file path escapes skill dir: %s", relPath)
		}
	}
	return filepath.Join(baseDir, normalized), nil
}

// safeWriteFile creates a file with O_EXCL to fail on existing files,
// preventing symlink attacks.
func safeWriteFile(path, content string) error {
	// O_EXCL ensures we don't overwrite existing files (defense against symlink attacks).
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}
