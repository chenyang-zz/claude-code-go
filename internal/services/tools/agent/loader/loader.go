package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
)

// LoadError records a single markdown file that failed to load or parse.
type LoadError struct {
	Path  string
	Error string
}

// LoadProjectAgents discovers and loads project-scoped agent definitions from
// the project's .claude/agents directory.
//
// It walks the directory recursively, reads every .md file, extracts YAML
// frontmatter, and builds an agent.Definition. Files that fail to parse are
// recorded in the returned error slice but do not abort the overall load.
//
// The returned source for every loaded agent is "projectSettings".
func LoadProjectAgents(projectDir string) ([]agent.Definition, []LoadError, error) {
	agentsDir := filepath.Join(projectDir, ".claude", "agents")
	return loadAgentsFromDir(agentsDir, "projectSettings")
}

// LoadUserAgents discovers and loads custom agent definitions from the
// user's ~/.claude/agents directory.
//
// It walks the directory recursively, reads every .md file, extracts YAML
// frontmatter, and builds an agent.Definition. Files that fail to parse are
// recorded in the returned error slice but do not abort the overall load.
//
// The returned source for every loaded agent is "userSettings".
func LoadUserAgents(homeDir string) ([]agent.Definition, []LoadError, error) {
	agentsDir := filepath.Join(homeDir, ".claude", "agents")
	return loadAgentsFromDir(agentsDir, "userSettings")
}

// LoadLocalAgents discovers and loads local-scoped agent definitions from the
// project's .claude/agents directory.
//
// Local agents share the same physical directory as project agents but use
// source "localSettings" for priority tracking. They are loaded after project
// agents so they can override project-scoped definitions.
//
// The returned source for every loaded agent is "localSettings".
func LoadLocalAgents(projectDir string) ([]agent.Definition, []LoadError, error) {
	agentsDir := filepath.Join(projectDir, ".claude", "agents")
	return loadAgentsFromDir(agentsDir, "localSettings")
}

// LoadManagedAgents discovers and loads managed agent definitions from the
// managed settings directory.
//
// It walks the directory recursively, reads every .md file, extracts YAML
// frontmatter, and builds an agent.Definition. Files that fail to parse are
// recorded in the returned error slice but do not abort the overall load.
//
// The returned source for every loaded agent is "policySettings".
func LoadManagedAgents(managedDir string) ([]agent.Definition, []LoadError, error) {
	agentsDir := filepath.Join(managedDir, ".claude", "agents")
	return loadAgentsFromDir(agentsDir, "policySettings")
}

// loadAgentsFromDir is the shared implementation for loading agent definitions
// from a directory tree. It discovers all .md files, parses YAML frontmatter,
// and returns the resulting definitions tagged with the given source label.
func loadAgentsFromDir(agentsDir string, source string) ([]agent.Definition, []LoadError, error) {
	// Check existence. Missing directory is not an error — just no agents.
	info, err := os.Stat(agentsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("failed to stat agents directory: %w", err)
	}
	if !info.IsDir() {
		return nil, nil, fmt.Errorf("agents path is not a directory: %s", agentsDir)
	}

	var files []string
	// Walk recursively to find all .md files.
	err = filepath.WalkDir(agentsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// Permission errors on individual files are logged but don't stop the walk.
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(path, ".md") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to walk agents directory: %w", err)
	}

	var defs []agent.Definition
	var loadErrors []LoadError

	for _, filePath := range files {
		def, err := loadAgentFile(filePath, agentsDir, source)
		if err != nil {
			loadErrors = append(loadErrors, LoadError{
				Path:  filePath,
				Error: err.Error(),
			})
			continue
		}
		// Skip files that don't look like agent definitions (no name in frontmatter).
		if def.AgentType == "" {
			continue
		}
		defs = append(defs, def)
	}

	return defs, loadErrors, nil
}

// loadAgentFile reads a single markdown file and converts it to an agent.Definition.
// Files without a 'name' field in frontmatter are treated as non-agent markdown
// and return an empty definition without error, allowing silent skipping.
func loadAgentFile(filePath string, baseDir string, source string) (agent.Definition, error) {
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return agent.Definition{}, fmt.Errorf("failed to read file: %w", err)
	}

	fm, content, err := ParseFrontmatter(string(raw))
	if err != nil {
		return agent.Definition{}, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	// Skip files without a 'name' field — they're likely co-located reference docs.
	if _, hasName := fm["name"]; !hasName {
		return agent.Definition{}, nil
	}

	def, err := BuildDefinitionFromFrontmatter(filePath, baseDir, fm, content, source)
	if err != nil {
		return agent.Definition{}, err
	}

	return def, nil
}
