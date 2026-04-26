package loader

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
)

// validPermissionModes lists the valid permission mode values for agent definitions.
var validPermissionModes = []string{"default", "plan", "acceptEdits", "dontAsk"}

// validEffortLevels lists the valid effort level string values.
var validEffortLevels = []string{"low", "medium", "high", "max", "auto"}

// validMemoryScopes lists the valid memory scope values.
var validMemoryScopes = []string{"user", "project", "local"}

// validIsolationModes lists the valid isolation mode values.
// "remote" is ant-only and excluded from batch-137.
var validIsolationModes = []string{"worktree"}

// BuildDefinitionFromFrontmatter constructs an agent.Definition from parsed
// frontmatter data, file metadata, and markdown body content.
//
// Required frontmatter fields: "name" (maps to AgentType) and
// "description" (maps to WhenToUse). If either is missing or empty,
// an error is returned.
//
// The content parameter is the markdown body (with frontmatter removed),
// which becomes the agent's static SystemPrompt after trimming.
func BuildDefinitionFromFrontmatter(
	filePath string,
	baseDir string,
	fm map[string]any,
	content string,
	source string,
) (agent.Definition, error) {
	name := getString(fm, "name")
	if name == "" {
		return agent.Definition{}, fmt.Errorf("missing required 'name' field in frontmatter")
	}

	description := getString(fm, "description")
	if description == "" {
		return agent.Definition{}, fmt.Errorf("missing required 'description' field in frontmatter")
	}
	// Unescape newlines that were escaped for YAML parsing.
	description = strings.ReplaceAll(description, "\\n", "\n")

	def := agent.Definition{
		AgentType:    name,
		WhenToUse:    description,
		Source:       source,
		BaseDir:      baseDir,
		Filename:     strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath)),
		SystemPrompt: strings.TrimSpace(content),
	}

	// Tools: nil means all tools, [] means no tools, ["*"] means all tools (nil).
	if v, ok := fm["tools"]; ok {
		def.Tools = parseAgentTools(v)
	}

	// DisallowedTools: same parsing semantics as tools.
	if v, ok := fm["disallowedTools"]; ok {
		def.DisallowedTools = parseAgentTools(v)
	}

	// Skills: comma-separated string or YAML list.
	if v, ok := fm["skills"]; ok {
		def.Skills = parseSkillList(v)
	}

	// Model: trim whitespace; preserve "inherit" verbatim.
	if v := getString(fm, "model"); v != "" {
		v = strings.TrimSpace(v)
		if strings.EqualFold(v, "inherit") {
			def.Model = "inherit"
		} else {
			def.Model = v
		}
	}

	// Effort: string level or integer. Invalid values are silently ignored.
	if v, ok := fm["effort"]; ok {
		if effort := parseEffort(v); effort != "" {
			def.Effort = effort
		}
	}

	// PermissionMode: validate against known modes.
	if v := getString(fm, "permissionMode"); v != "" {
		if slices.Contains(validPermissionModes, v) {
			def.PermissionMode = v
		}
	}

	// MaxTurns: positive integer. Invalid values are silently ignored.
	if v, ok := fm["maxTurns"]; ok {
		if n := parsePositiveInt(v); n > 0 {
			def.MaxTurns = n
		}
	}

	// Background: only true for literal true or "true" string.
	if v, ok := fm["background"]; ok {
		def.Background = parseBool(v)
	}

	// Memory: validate against known scopes.
	if v := getString(fm, "memory"); v != "" {
		if slices.Contains(validMemoryScopes, v) {
			def.Memory = v
		}
	}

	// Isolation: validate against known modes.
	if v := getString(fm, "isolation"); v != "" {
		if slices.Contains(validIsolationModes, v) {
			def.Isolation = v
		}
	}

	// InitialPrompt: non-empty string after trimming.
	if v := getString(fm, "initialPrompt"); v != "" {
		def.InitialPrompt = strings.TrimSpace(v)
	}

	return def, nil
}

// getString extracts a string value from frontmatter.
// Returns empty string for missing or non-string values.
func getString(fm map[string]any, key string) string {
	v, ok := fm[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// parseAgentTools parses the tools or disallowedTools frontmatter value.
//
// Semantics aligned with TypeScript parseAgentToolsFromFrontmatter:
//   - nil / missing → nil (all tools available)
//   - empty string or empty array → [] (no tools)
//   - contains "*" → nil (all tools available)
//   - otherwise → string slice of tool names
func parseAgentTools(v any) []string {
	if v == nil {
		return nil
	}

	var items []string
	switch val := v.(type) {
	case string:
		if val == "" {
			return []string{}
		}
		items = []string{val}
	case []any:
		if len(val) == 0 {
			return []string{}
		}
		for _, item := range val {
			if s, ok := item.(string); ok {
				items = append(items, s)
			}
		}
	default:
		return nil
	}

	// Wildcard means all tools.
	if slices.Contains(items, "*") {
		return nil
	}
	return items
}

// parseSkillList parses the skills frontmatter value.
// Supports comma-separated string or YAML string array.
func parseSkillList(v any) []string {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case string:
		if val == "" {
			return nil
		}
		parts := strings.Split(val, ",")
		var result []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				result = append(result, p)
			}
		}
		return result
	case []any:
		var result []string
		for _, item := range val {
			if s, ok := item.(string); ok && s != "" {
				result = append(result, s)
			}
		}
		return result
	default:
		return nil
	}
}

// parseEffort validates and normalizes an effort value from frontmatter.
// Accepts known level strings or positive integers.
// Returns empty string for invalid values.
func parseEffort(v any) string {
	if s, ok := v.(string); ok {
		s = strings.TrimSpace(s)
		if s == "" {
			return ""
		}
		for _, level := range validEffortLevels {
			if strings.EqualFold(s, level) {
				return level
			}
		}
		if _, err := strconv.Atoi(s); err == nil {
			return s
		}
		return ""
	}

	// Accept integer values directly from YAML.
	if n, ok := v.(int); ok && n > 0 {
		return strconv.Itoa(n)
	}
	if f, ok := v.(float64); ok {
		n := int(f)
		if n > 0 && float64(n) == f {
			return strconv.Itoa(n)
		}
	}

	return ""
}

// parsePositiveInt parses a positive integer from frontmatter.
// Returns 0 for missing, zero, negative, or non-numeric values.
func parsePositiveInt(v any) int {
	switch val := v.(type) {
	case int:
		if val > 0 {
			return val
		}
	case float64:
		n := int(val)
		if n > 0 && float64(n) == val {
			return n
		}
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(val))
		if err == nil && n > 0 {
			return n
		}
	}
	return 0
}

// parseBool parses a boolean frontmatter value.
// Only returns true for literal true or "true" string (case-insensitive).
// All other values (including "false", false, nil, strings) return false.
func parseBool(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(strings.ToLower(s)) == "true"
	}
	return false
}

