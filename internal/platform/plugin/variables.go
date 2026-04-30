package plugin

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// SubstitutePluginVariables replaces plugin-specific placeholders in value with
// their resolved counterparts.
//
// Supported placeholders:
//   - ${CLAUDE_PLUGIN_ROOT} → pluginPath (normalized with forward slashes)
//   - ${CLAUDE_PLUGIN_DATA} → data directory for pluginSource (only if non-empty)
func SubstitutePluginVariables(value, pluginPath, pluginSource string) string {
	// Replace ${CLAUDE_PLUGIN_ROOT} with the normalized plugin path.
	normalizedPath := filepath.ToSlash(pluginPath)
	value = strings.ReplaceAll(value, "${CLAUDE_PLUGIN_ROOT}", normalizedPath)

	// Replace ${CLAUDE_PLUGIN_DATA} with the plugin data directory.
	if pluginSource != "" {
		if dataDir, err := GetPluginDataDir(pluginSource); err == nil {
			normalizedDataDir := filepath.ToSlash(dataDir)
			value = strings.ReplaceAll(value, "${CLAUDE_PLUGIN_DATA}", normalizedDataDir)
		}
	}

	return value
}

// SubstituteSkillDir replaces ${CLAUDE_SKILL_DIR} placeholders in value with
// the given skill directory path (normalized with forward slashes). Only
// substitutes when skillDir is non-empty.
func SubstituteSkillDir(value, skillDir string) string {
	if skillDir != "" {
		normalizedDir := filepath.ToSlash(skillDir)
		value = strings.ReplaceAll(value, "${CLAUDE_SKILL_DIR}", normalizedDir)
	}
	return value
}

// SubstituteArguments replaces argument placeholders in content with values
// from args. It supports the following patterns:
//
//   - $ARGUMENTS        → all arguments joined by a single space
//   - $ARGUMENTS[n]    → the n-th argument (0-based index)
//   - $n               → shorthand for the n-th argument (1-based index)
//   - $name            → named argument mapped from argumentNames
//
// If appendIfNoPlaceholder is true and no placeholder was found in content,
// the arguments are appended as "\n\nARGUMENTS: {args}".
func SubstituteArguments(content string, args []string, argumentNames []string, appendIfNoPlaceholder bool) string {
	if len(args) == 0 && len(argumentNames) == 0 {
		return content
	}

	hasPlaceholder := false

	// $ARGUMENTS[n] → indexed argument (0-based).
	reIndexed := regexp.MustCompile(`\$ARGUMENTS\[(\d+)\]`)
	if reIndexed.MatchString(content) {
		hasPlaceholder = true
	}
	content = reIndexed.ReplaceAllStringFunc(content, func(match string) string {
		submatches := reIndexed.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		var idx int
		fmt.Sscanf(submatches[1], "%d", &idx)
		if idx >= 0 && idx < len(args) {
			return args[idx]
		}
		return match
	})

	// $ARGUMENTS → full argument string. Only match when NOT followed by '['.
	// Use a regex that captures the trailing character so we can preserve it.
	reBareArgs := regexp.MustCompile(`\$ARGUMENTS([^[]|$)`)
	if reBareArgs.MatchString(content) {
		hasPlaceholder = true
		fullArgs := strings.Join(args, " ")
		content = reBareArgs.ReplaceAllString(content, fullArgs+"${1}")
	}

	// $n → shorthand indexed argument (1-based).
	reShorthand := regexp.MustCompile(`\$(\d+)`)
	content = reShorthand.ReplaceAllStringFunc(content, func(match string) string {
		hasPlaceholder = true
		submatches := reShorthand.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		var idx int
		fmt.Sscanf(submatches[1], "%d", &idx)
		if idx >= 1 && idx <= len(args) {
			return args[idx-1]
		}
		return match
	})

	// $name → named argument (mapped from argumentNames by position).
	for i, name := range argumentNames {
		if name == "" {
			continue
		}
		placeholder := "$" + name
		if strings.Contains(content, placeholder) {
			hasPlaceholder = true
			var replacement string
			if i < len(args) {
				replacement = args[i]
			}
			content = strings.ReplaceAll(content, placeholder, replacement)
		}
	}

	// Fallback: append arguments if no placeholder was found.
	if !hasPlaceholder && appendIfNoPlaceholder {
		content += "\n\nARGUMENTS: " + strings.Join(args, " ")
	}

	return content
}
