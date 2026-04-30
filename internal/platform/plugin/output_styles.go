package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// ExtractOutputStyles walks the plugin's output-styles/ directory and returns
// the extracted OutputStyleConfig list. Each .md file is parsed for YAML
// frontmatter to populate style metadata. Styles are namespaced as
// "pluginName:styleName".
func ExtractOutputStyles(plugin *LoadedPlugin) ([]*OutputStyleConfig, error) {
	if plugin.OutputStylesPath == "" {
		return nil, nil
	}

	// Output styles uses flat .md file walk (no SKILL.md leaf container logic).
	mdFiles, err := walkMarkdownFiles(plugin.OutputStylesPath, false)
	if err != nil {
		return nil, fmt.Errorf("failed to walk output-styles directory %s: %w", plugin.OutputStylesPath, err)
	}

	var styles []*OutputStyleConfig
	seen := make(map[string]struct{})

	for _, filePath := range mdFiles {
		// Skip symlinks for security.
		if info, err := os.Lstat(filePath); err == nil && info.Mode()&os.ModeSymlink != 0 {
			logger.DebugCF("plugin.output_styles", "skipping symlink", map[string]any{
				"path": filePath,
			})
			continue
		}

		// Deduplicate.
		if _, exists := seen[filePath]; exists {
			continue
		}
		seen[filePath] = struct{}{}

		style, err := buildOutputStyle(plugin, filePath)
		if err != nil {
			logger.DebugCF("plugin.output_styles", "failed to build output style", map[string]any{
				"path":  filePath,
				"error": err.Error(),
			})
			continue
		}
		if style != nil {
			styles = append(styles, style)
		}
	}

	return styles, nil
}

// buildOutputStyle reads a markdown file, parses its frontmatter, and
// constructs an OutputStyleConfig with the appropriate namespaced name.
func buildOutputStyle(plugin *LoadedPlugin, filePath string) (*OutputStyleConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	frontmatter, body, err := parseFrontmatter(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	// Base style name: frontmatter "name" field or filename without extension.
	baseName := frontmatter["name"]
	if baseName == "" {
		baseName = strings.TrimSuffix(filepath.Base(filePath), ".md")
	}

	description := frontmatter["description"]
	if description == "" {
		description = firstParagraph(body)
	}

	forceForPlugin := parseBool(frontmatter["force-for-plugin"], false)

	return &OutputStyleConfig{
		Name:           plugin.Name + ":" + baseName,
		Description:    description,
		Prompt:         strings.TrimSpace(string(body)),
		ForceForPlugin: forceForPlugin,
		PluginName:     plugin.Name,
	}, nil
}
