package plugin

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// userConfigRegex matches ${user_config.KEY} placeholders.
var userConfigRegex = regexp.MustCompile(`\$\{user_config\.([^}]+)\}`)

// SubstituteUserConfigVariables replaces ${user_config.KEY} placeholders in value
// with their resolved counterparts from userConfig.
//
// This is the strict variant used for MCP/LSP server config and hook commands.
// Missing keys produce an error — callers should only invoke this after user
// config has been validated, so a miss indicates a plugin authoring bug.
func SubstituteUserConfigVariables(value string, userConfig map[string]any) (string, error) {
	var missing []string
	result := userConfigRegex.ReplaceAllStringFunc(value, func(match string) string {
		submatches := userConfigRegex.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		key := submatches[1]
		v, ok := userConfig[key]
		if !ok {
			missing = append(missing, key)
			return match
		}
		return fmt.Sprintf("%v", v)
	})
	if len(missing) > 0 {
		return result, fmt.Errorf("missing required user configuration value(s): %s", strings.Join(missing, ", "))
	}
	return result, nil
}

// SubstituteUserConfigInContent replaces ${user_config.KEY} placeholders in content
// with their resolved values from userConfig.
//
// This is the content-safe variant used for skill/agent prose that goes to the
// model prompt. Differences from SubstituteUserConfigVariables:
//
//   - Sensitive-marked keys substitute to a descriptive placeholder instead of
//     the actual value — we don't put secrets in the model's context.
//   - Unknown keys stay literal (no throw) — matches how ${VAR} env refs behave
//     when the var is unset.
func SubstituteUserConfigInContent(content string, userConfig map[string]any, schema map[string]PluginConfigOption) string {
	return userConfigRegex.ReplaceAllStringFunc(content, func(match string) string {
		submatches := userConfigRegex.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		key := submatches[1]
		if opt, ok := schema[key]; ok && opt.Sensitive {
			return fmt.Sprintf("[sensitive option '%s' not available in skill content]", key)
		}
		v, ok := userConfig[key]
		if !ok {
			return match
		}
		return fmt.Sprintf("%v", v)
	})
}

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
