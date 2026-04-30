package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// ExtractAgents walks the plugin's agents/ directory and returns the extracted
// AgentDefinition list. Each .md file is parsed for YAML frontmatter to
// populate agent metadata. Agents are namespaced as "pluginName:agentName".
func ExtractAgents(plugin *LoadedPlugin) ([]*AgentDefinition, error) {
	if plugin.AgentsPath == "" {
		return nil, nil
	}

	mdFiles, err := walkMarkdownFiles(plugin.AgentsPath, true)
	if err != nil {
		return nil, fmt.Errorf("failed to walk agents directory %s: %w", plugin.AgentsPath, err)
	}

	var agents []*AgentDefinition
	seen := make(map[string]struct{})

	for _, filePath := range mdFiles {
		if info, err := os.Lstat(filePath); err == nil && info.Mode()&os.ModeSymlink != 0 {
			logger.DebugCF("plugin.agents", "skipping symlink", map[string]any{
				"path": filePath,
			})
			continue
		}

		if _, exists := seen[filePath]; exists {
			continue
		}
		seen[filePath] = struct{}{}

		agent, err := buildAgentDefinition(plugin, filePath)
		if err != nil {
			logger.DebugCF("plugin.agents", "failed to build agent", map[string]any{
				"path":  filePath,
				"error": err.Error(),
			})
			continue
		}
		if agent != nil {
			agents = append(agents, agent)
		}
	}

	return agents, nil
}

// buildAgentDefinition reads a markdown file, parses its frontmatter, and
// constructs an AgentDefinition with the appropriate namespaced agent type.
func buildAgentDefinition(plugin *LoadedPlugin, filePath string) (*AgentDefinition, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	frontmatter, body, err := parseFrontmatter(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	agentType := buildAgentName(filePath, plugin.AgentsPath, plugin.Name, frontmatter["name"])
	displayName := frontmatter["name"]

	description := frontmatter["description"]
	if description == "" {
		description = firstParagraph(body)
	}

	maxTurns := 0
	if mt := frontmatter["max-turns"]; mt != "" {
		if n, err := strconv.Atoi(mt); err == nil {
			maxTurns = n
		}
	}

	return &AgentDefinition{
		AgentType:       agentType,
		DisplayName:     displayName,
		Description:     description,
		WhenToUse:       frontmatter["when-to-use"],
		PluginName:      plugin.Name,
		PluginPath:      plugin.Path,
		SourcePath:      filePath,
		Tools:           frontmatter["tools"],
		Skills:          frontmatter["skills"],
		Color:           frontmatter["color"],
		Model:           frontmatter["model"],
		Background:      parseBool(frontmatter["background"], false),
		Memory:          frontmatter["memory"],
		Isolation:       frontmatter["isolation"],
		Effort:          frontmatter["effort"],
		MaxTurns:        maxTurns,
		DisallowedTools: frontmatter["disallowed-tools"],
		RawContent:      string(body),
	}, nil
}

// buildAgentName constructs the fully namespaced agent type name. If an
// override name is provided (from frontmatter), it takes precedence over the
// filename. Subdirectory paths relative to baseDir become the namespace.
//
// Examples:
//   - agents/reviewer.md           → pluginName:reviewer
//   - agents/qa/test-runner.md     → pluginName:qa:test-runner
func buildAgentName(filePath, baseDir, pluginName, overrideName string) string {
	relPath, err := filepath.Rel(baseDir, filePath)
	if err != nil {
		base := filepath.Base(filePath)
		base = base[:len(base)-len(filepath.Ext(base))]
		if overrideName != "" {
			base = overrideName
		}
		return pluginName + ":" + base
	}

	dir := filepath.Dir(relPath)
	base := filepath.Base(filePath)
	base = base[:len(base)-len(filepath.Ext(base))]
	if overrideName != "" {
		base = overrideName
	}

	if dir == "." || dir == "" {
		return pluginName + ":" + base
	}

	namespace := filepath.ToSlash(dir)
	return pluginName + ":" + namespace + ":" + base
}
