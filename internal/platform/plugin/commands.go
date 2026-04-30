package plugin

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// ExtractCommands walks the plugin's commands/ directory and returns the
// extracted PluginCommand list. Each .md file is parsed for YAML frontmatter
// to populate command metadata. Commands are namespaced as "pluginName:cmdName".
func ExtractCommands(plugin *LoadedPlugin) ([]*PluginCommand, error) {
	if plugin.CommandsPath == "" {
		return nil, nil
	}
	return loadCommandsFromDir(plugin, plugin.CommandsPath, false)
}

// ExtractSkills walks the plugin's skills/ directory and returns the extracted
// PluginCommand list with IsSkill set to true. Skill directories containing
// SKILL.md are treated as leaf containers.
func ExtractSkills(plugin *LoadedPlugin) ([]*PluginCommand, error) {
	if plugin.SkillsPath == "" {
		return nil, nil
	}
	return loadCommandsFromDir(plugin, plugin.SkillsPath, true)
}

// loadCommandsFromDir walks a directory for .md files and builds PluginCommand
// objects from each one.
func loadCommandsFromDir(plugin *LoadedPlugin, dirPath string, isSkill bool) ([]*PluginCommand, error) {
	mdFiles, err := walkMarkdownFiles(dirPath, isSkill)
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory %s: %w", dirPath, err)
	}

	var commands []*PluginCommand
	seen := make(map[string]struct{})

	for _, filePath := range mdFiles {
		// Skip symlinks for security.
		if info, err := os.Lstat(filePath); err == nil && info.Mode()&os.ModeSymlink != 0 {
			logger.DebugCF("plugin.commands", "skipping symlink", map[string]any{
				"path": filePath,
			})
			continue
		}

		// Deduplicate.
		if _, exists := seen[filePath]; exists {
			continue
		}
		seen[filePath] = struct{}{}

		cmd, err := buildCommand(plugin, filePath, dirPath, isSkill)
		if err != nil {
			logger.DebugCF("plugin.commands", "failed to build command", map[string]any{
				"path":  filePath,
				"error": err.Error(),
			})
			continue
		}
		if cmd != nil {
			commands = append(commands, cmd)
		}
	}

	return commands, nil
}

// buildCommand reads a markdown file, parses its frontmatter, and constructs a
// PluginCommand with the appropriate namespaced name.
func buildCommand(plugin *LoadedPlugin, filePath, baseDir string, isSkill bool) (*PluginCommand, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	frontmatter, body, err := parseFrontmatter(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	cmdName := commandName(filePath, baseDir, plugin.Name, isSkill)

	displayName := frontmatter["name"]
	description := frontmatter["description"]
	if description == "" {
		description = firstParagraph(body)
	}

	cmd := &PluginCommand{
		Name:                   cmdName,
		DisplayName:            displayName,
		Description:            description,
		PluginName:             plugin.Name,
		PluginPath:             plugin.Path,
		SourcePath:             filePath,
		IsSkill:                isSkill,
		RawContent:             string(body),
		AllowedTools:           frontmatter["allowed-tools"],
		ArgumentHint:           frontmatter["argument-hint"],
		WhenToUse:              frontmatter["when_to_use"],
		Version:                frontmatter["version"],
		Model:                  frontmatter["model"],
		Effort:                 frontmatter["effort"],
		UserInvocable:          parseBool(frontmatter["user-invocable"], true),
		DisableModelInvocation: parseBool(frontmatter["disable-model-invocation"], false),
		Shell:                  defaultString(frontmatter["shell"], "bash"),
	}

	return cmd, nil
}

// walkMarkdownFiles recursively walks a directory and returns all .md file
// paths. When isSkill is true and a directory contains a SKILL.md file, that
// directory is treated as a leaf container — its .md files are included but
// subdirectories are not recursed into.
func walkMarkdownFiles(rootDir string, isSkill bool) ([]string, error) {
	var files []string

	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return nil, err
	}

	// Check if this is a skill directory (contains SKILL.md).
	if isSkill {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if strings.EqualFold(entry.Name(), "SKILL.md") {
				// Leaf container: only collect .md files in this directory,
				// don't recurse into subdirectories.
				for _, e := range entries {
					if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
						fullPath := filepath.Join(rootDir, e.Name())
						files = append(files, fullPath)
					}
				}
				return files, nil
			}
		}
	}

	for _, entry := range entries {
		fullPath := filepath.Join(rootDir, entry.Name())

		if entry.IsDir() {
			subFiles, err := walkMarkdownFiles(fullPath, isSkill)
			if err != nil {
				logger.DebugCF("plugin.commands", "failed to walk subdirectory", map[string]any{
					"path":  fullPath,
					"error": err.Error(),
				})
				continue
			}
			files = append(files, subFiles...)
		} else if strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			files = append(files, fullPath)
		}
	}

	return files, nil
}

// commandName builds the fully namespaced command name from the file path.
//
// For SKILL.md files, the parent directory name is used as the command base
// name. For regular .md files, the filename without extension is used.
// Subdirectory paths relative to baseDir become the namespace, with directory
// separators replaced by colons.
//
// Examples:
//   - commands/deploy.md        → pluginName:deploy
//   - commands/tools/build/SKILL.md → pluginName:tools:build
func commandName(filePath, baseDir, pluginName string, isSkill bool) string {
	relPath, err := filepath.Rel(baseDir, filePath)
	if err != nil {
		return pluginName + ":" + strings.TrimSuffix(filepath.Base(filePath), ".md")
	}

	dir := filepath.Dir(relPath)
	name := strings.TrimSuffix(filepath.Base(relPath), ".md")

	if isSkill && strings.EqualFold(filepath.Base(filePath), "SKILL.md") {
		// Use the parent directory name for SKILL.md.
		name = filepath.Base(filepath.Dir(filePath))
		// Also adjust dir to go one level up.
		dir = filepath.Dir(dir)
	}

	if dir == "." || dir == "" {
		return pluginName + ":" + name
	}

	// Replace directory separators with colons for namespace.
	namespace := strings.ReplaceAll(dir, string(filepath.Separator), ":")
	return pluginName + ":" + namespace + ":" + name
}

// parseFrontmatter extracts YAML frontmatter delimited by "---" from markdown
// content. It returns a map of key-value pairs (only simple string values are
// parsed), the remaining markdown body, and any error.
func parseFrontmatter(content []byte) (map[string]string, []byte, error) {
	text := string(content)
	if !strings.HasPrefix(text, "---") {
		return make(map[string]string), content, nil
	}

	// Find the closing "---".
	rest := text[3:]
	endIdx := strings.Index(rest, "\n---")
	if endIdx < 0 {
		// No closing delimiter; treat entire content as body.
		return make(map[string]string), content, nil
	}

	fmText := rest[:endIdx]
	body := []byte(strings.TrimSpace(rest[endIdx+4:]))

	frontmatter := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(fmText))
	for scanner.Scan() {
		line := scanner.Text()
		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:colonIdx])
		value := strings.TrimSpace(line[colonIdx+1:])
		// Strip surrounding quotes if present.
		value = strings.Trim(value, "\"'")
		if key != "" {
			frontmatter[key] = value
		}
	}

	return frontmatter, body, nil
}

// firstParagraph extracts the first non-empty paragraph from markdown content.
// It reads until it encounters an empty line or a heading.
func firstParagraph(body []byte) string {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			if len(lines) > 0 {
				break
			}
			continue
		}
		lines = append(lines, strings.TrimSpace(line))
	}
	return strings.Join(lines, " ")
}

// parseBool parses a boolean value from a frontmatter string. Accepts "true"
// and "false" (case-insensitive). Returns the default value if the string is
// empty or unrecognized.
func parseBool(value string, defaultVal bool) bool {
	switch strings.ToLower(value) {
	case "true":
		return true
	case "false":
		return false
	default:
		return defaultVal
	}
}

// defaultString returns value if non-empty, otherwise returns defaultVal.
func defaultString(value, defaultVal string) string {
	if value != "" {
		return value
	}
	return defaultVal
}
