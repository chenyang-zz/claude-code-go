package skill

import (
	"os"
	"path/filepath"
	"strings"

	agentloader "github.com/sheepzhao/claude-code-go/internal/services/tools/agent/loader"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// LoadSkillsFromCommandsDir loads skills from legacy .claude/commands/
// directories. It supports two formats:
//
//  1. Directory format: commands/<name>/SKILL.md — the command name is the
//     directory name. The directory becomes the skill's baseDir.
//  2. Single file format: commands/<name>.md — the command name is the
//     filename without extension. No baseDir is set.
//
// Nested subdirectories produce namespaced commands using ":" separators:
// commands/subdir/foo.md → command name "subdir:foo".
//
// When a directory contains SKILL.md, any other .md files in that directory
// are dropped (SKILL.md takes over the namespace).
//
// All loaded skills are marked with loadedFrom = "commands_DEPRECATED" and
// source = "projectSettings".
func LoadSkillsFromCommandsDir(projectDir string) ([]*Skill, []LoadError, error) {
	commandsDir := filepath.Join(projectDir, ".claude", "commands")

	// Phase 1: collect all .md file paths grouped by parent directory.
	filesByDir, loadErrors := discoverCommandFiles(commandsDir)

	// Phase 2: apply SKILL.md takeover rule.
	finalFiles := applySkillFileRule(filesByDir)

	// Phase 3: build Skill objects from the final file list.
	var skills []*Skill
	for _, f := range finalFiles {
		skill, err := loadCommandFile(f.path, f.cmdName, f.baseDir, f.namespace)
		if err != nil {
			loadErrors = append(loadErrors, LoadError{
				Name:  f.cmdName,
				Error: err.Error(),
			})
			continue
		}
		if skill != nil {
			skills = append(skills, skill)
		}
	}

	return skills, loadErrors, nil
}

// commandFileInfo holds metadata for a discovered command file.
type commandFileInfo struct {
	path      string // full path to the .md file
	cmdName   string // resolved command name
	baseDir   string // directory for SKILL.md format, "" for plain .md
	namespace string // colon-separated namespace
	isSkill   bool   // true if this is a SKILL.md file
	parentDir string // parent directory (for grouping)
}

// discoverCommandFiles recursively walks the commands directory and collects
// all .md files grouped by parent directory.
func discoverCommandFiles(dir string) (map[string][]commandFileInfo, []LoadError) {
	filesByDir := make(map[string][]commandFileInfo)
	var loadErrors []LoadError

	// Use filepath.Walk for recursive traversal.
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if !os.IsNotExist(err) {
				loadErrors = append(loadErrors, LoadError{
					Name:  path,
					Error: "walk error: " + err.Error(),
				})
			}
			return nil
		}

		if d.IsDir() {
			return nil
		}

		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return nil
		}

		parentDir := filepath.Dir(path)
		baseName := strings.TrimSuffix(d.Name(), ".md")
		isSkill := strings.EqualFold(baseName, "SKILL")

		// Build command name with namespace.
		relDir := filepath.Dir(relPath)
		var namespace string
		var cmdName string
		var baseDir string

		if isSkill {
			// Directory format: command name = parent directory name.
			// The namespace is the parent of relDir (the directories above
			// the SKILL.md's parent directory).
			parentOfRelDir := filepath.Dir(relDir)
			namespace = buildCommandsNamespace(parentOfRelDir)
			cmdName = buildCommandName(namespace, filepath.Base(parentDir))
			baseDir = parentDir
		} else {
			// Single file format: command name = filename without .md.
			namespace = buildCommandsNamespace(relDir)
			cmdName = buildCommandName(namespace, baseName)
			baseDir = ""
		}

		filesByDir[parentDir] = append(filesByDir[parentDir], commandFileInfo{
			path:      path,
			cmdName:   cmdName,
			baseDir:   baseDir,
			namespace: namespace,
			isSkill:   isSkill,
			parentDir: parentDir,
		})

		return nil
	})

	return filesByDir, loadErrors
}

// applySkillFileRule applies the SKILL.md takeover rule: when a directory
// contains SKILL.md, only that file is kept; all other .md files in the
// same directory are dropped.
func applySkillFileRule(filesByDir map[string][]commandFileInfo) []commandFileInfo {
	var result []commandFileInfo

	for _, files := range filesByDir {
		var skillFile *commandFileInfo
		hasSkill := false
		for i := range files {
			if files[i].isSkill {
				if !hasSkill {
					skillFile = &files[i]
					hasSkill = true
				}
				// If multiple SKILL.md files exist (unlikely), use the first.
			}
		}

		if hasSkill && skillFile != nil {
			result = append(result, *skillFile)
		} else {
			result = append(result, files...)
		}
	}

	return result
}

// buildCommandsNamespace builds a colon-separated namespace from a relative
// directory path. "." or "" produce an empty namespace.
func buildCommandsNamespace(relDir string) string {
	relDir = filepath.ToSlash(relDir)
	if relDir == "" || relDir == "." {
		return ""
	}
	return strings.ReplaceAll(relDir, "/", ":")
}

// buildCommandName constructs a namespaced command name.
func buildCommandName(namespace, baseName string) string {
	if namespace == "" {
		return baseName
	}
	return namespace + ":" + baseName
}

// loadCommandFile reads a command .md file, parses frontmatter, and builds a Skill.
func loadCommandFile(path, cmdName, baseDir, namespace string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	frontmatter, body, err := agentloader.ParseFrontmatter(string(data))
	if err != nil {
		logger.DebugCF("skill", "failed to parse command frontmatter", map[string]any{
			"path":  path,
			"error": err.Error(),
		})
		return nil, err
	}

	resolvedName := toString(frontmatter, "name")
	if resolvedName == "" {
		resolvedName = cmdName
	}

	rawDescription := toString(frontmatter, "description")
	hasUserSpecified := rawDescription != ""
	description := rawDescription
	if description == "" {
		description = extractDescription(body, cmdName)
	}

	userInvocable := true // commands default to user-invocable
	model := ""
	if rawModel := toString(frontmatter, "model"); rawModel != "" && rawModel != "inherit" {
		model = rawModel
	}

	skill := NewSkill(SkillOptions{
		Name:                       resolvedName,
		DisplayName:                toString(frontmatter, "name"),
		Description:                description,
		HasUserSpecifiedDescription: hasUserSpecified,
		WhenToUse:                  toString(frontmatter, "when_to_use"),
		Version:                    toString(frontmatter, "version"),
		Content:                    body,
		BaseDir:                    baseDir,
		AllowedTools:               parseSlashCommandTools(frontmatter, "allowed-tools"),
		ArgumentHint:               toString(frontmatter, "argument-hint"),
		ArgumentNames:              parseArgumentNames(frontmatter, "arguments"),
		Model:                      model,
		DisableModelInvocation:     toBool(frontmatter, "disable-model-invocation"),
		UserInvocable:              &userInvocable,
		Source:                     "projectSettings",
		LoadedFrom:                 "commands_DEPRECATED",
	})

	logger.DebugCF("skill", "loaded command skill", map[string]any{
		"name":      skill.name,
		"base_dir":  baseDir,
		"namespace": namespace,
	})

	return skill, nil
}
