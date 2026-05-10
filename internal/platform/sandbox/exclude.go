package sandbox

import (
	"strings"
)

// ExcludeRuleType describes how an exclude pattern matches.
type ExcludeRuleType int

const (
	ExcludePrefix   ExcludeRuleType = iota // "npm run test:*" matches prefixes
	ExcludeExact                           // "docker" matches exact command
	ExcludeWildcard                        // "bazel*" matches glob
)

// ExcludeRule is a compiled exclude pattern ready for matching.
type ExcludeRule struct {
	Raw     string
	Type    ExcludeRuleType
	Prefix  string
	Command string
	Pattern string
}

// CompileExcludePattern converts a user-facing exclude pattern into a rule.
// Patterns ending in ":*" or matching tool rule syntax are treated as prefix matches.
func CompileExcludePattern(raw string) ExcludeRule {
	trimmed := strings.TrimSpace(raw)

	// Wildcard patterns: "npm run test:*" → prefix match
	if rest, ok := strings.CutSuffix(trimmed, ":*"); ok {
		return ExcludeRule{
			Raw:    raw,
			Type:   ExcludePrefix,
			Prefix: rest,
		}
	}

	// Wildcard patterns with "*"
	if strings.Contains(trimmed, "*") {
		return ExcludeRule{
			Raw:     raw,
			Type:    ExcludeWildcard,
			Pattern: trimmed,
		}
	}

	// Default: exact match
	return ExcludeRule{
		Raw:     raw,
		Type:    ExcludeExact,
		Command: trimmed,
	}
}

// MatchExcludeRule checks if a command matches the exclude rule.
func MatchExcludeRule(rule ExcludeRule, command string) bool {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return false
	}

	// Split compound commands (&&, ||, ;, |) into subcommands and check each.
	// This prevents "docker ps && curl evil.com" from escaping sandbox
	// just because "docker ps" matches an excluded pattern.
	subcommands := splitCompoundCommand(trimmed)

	for _, sub := range subcommands {
		sub = strings.TrimSpace(sub)
		if sub == "" {
			continue
		}
		// Generate candidates with env vars and safe wrappers stripped.
		candidates := generateMatchCandidates(sub)
		for _, cand := range candidates {
			if matchRule(rule, cand) {
				return true
			}
		}
	}
	return false
}

// matchRule applies a single rule against a candidate command string.
func matchRule(rule ExcludeRule, command string) bool {
	switch rule.Type {
	case ExcludePrefix:
		if command == rule.Prefix {
			return true
		}
		// Prefix match: rule.Prefix must be followed by a separator (space, :, /)
		if strings.HasPrefix(command, rule.Prefix) && len(command) > len(rule.Prefix) {
			sep := command[len(rule.Prefix)]
			if sep == ' ' || sep == ':' || sep == '/' || sep == '-' {
				return true
			}
		}
	case ExcludeExact:
		if command == rule.Command {
			return true
		}
	case ExcludeWildcard:
		return matchWildcard(rule.Pattern, command)
	}
	return false
}

// splitCompoundCommand splits a bash command on compound operators.
func splitCompoundCommand(command string) []string {
	var result []string
	var current strings.Builder
	depth := 0
	inSingle := false
	inDouble := false

	for i := 0; i < len(command); i++ {
		ch := command[i]
		switch {
		case ch == '\'' && !inDouble:
			inSingle = !inSingle
		case ch == '"' && !inSingle:
			inDouble = !inDouble
		case ch == '(' && !inSingle && !inDouble:
			depth++
		case ch == ')' && !inSingle && !inDouble:
			if depth > 0 {
				depth--
			}
		case (ch == '&' || ch == '|' || ch == ';') && depth == 0 && !inSingle && !inDouble:
			if current.Len() > 0 {
				result = append(result, current.String())
				current.Reset()
			}
			// Skip the operator character
			continue
		}
		current.WriteByte(ch)
	}
	if current.Len() > 0 {
		result = append(result, current.String())
	}

	// If splitting produced nothing useful, return the original
	if len(result) == 0 {
		return []string{command}
	}
	return result
}

// generateMatchCandidates produces possible match candidates from a command
// by iteratively stripping env vars and safe wrappers (fixed-point approach).
func generateMatchCandidates(command string) []string {
	candidates := []string{command}
	seen := map[string]bool{command: true}

	// Binary hijack env vars that should be stripped when matching
	binaryHijackVars := []string{
		"LD_PRELOAD", "LD_LIBRARY_PATH", "DYLD_INSERT_LIBRARIES",
		"PATH", "HOME", "SHELL", "BASH_ENV",
	}

	// Safe wrappers that can be stripped
	safeWrappers := []string{
		"timeout", "nohup", "nice", "env", "eval", "sh", "bash", "zsh", "fish",
	}

	idx := 0
	for idx < len(candidates) {
		cmd := candidates[idx]
		idx++

		// Strip leading env var assignments
		stripped := stripLeadingEnvVars(cmd, binaryHijackVars)
		if stripped != cmd && !seen[stripped] {
			candidates = append(candidates, stripped)
			seen[stripped] = true
		}

		// Strip safe wrapper commands
		wrapperStripped := stripSafeWrappers(cmd, safeWrappers)
		if wrapperStripped != cmd && !seen[wrapperStripped] {
			candidates = append(candidates, wrapperStripped)
			seen[wrapperStripped] = true
		}
	}

	return candidates
}

// stripLeadingEnvVars removes leading KEY=VALUE assignments and known
// env variables from the command.
func stripLeadingEnvVars(command string, knownVars []string) string {
	trimmed := strings.TrimSpace(command)
	parts := strings.Fields(trimmed)

	// Find where the env vars end and the real command begins
	startIdx := 0
	for i, part := range parts {
		isEnv := false
		if strings.Contains(part, "=") && !strings.HasPrefix(part, "-") {
			isEnv = true
		}
		for _, kv := range knownVars {
			if strings.HasPrefix(part, kv+"=") {
				isEnv = true
				break
			}
		}
		if !isEnv {
			startIdx = i
			break
		}
		startIdx = i + 1
	}

	if startIdx >= len(parts) {
		return ""
	}
	return strings.Join(parts[startIdx:], " ")
}

// stripSafeWrappers removes a leading safe wrapper command if present.
func stripSafeWrappers(command string, wrappers []string) string {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return ""
	}
	first := strings.Fields(trimmed)[0]
	for _, w := range wrappers {
		if first == w {
			rest := strings.TrimSpace(strings.TrimPrefix(trimmed, first))
			if rest == "" {
				return ""
			}
			return rest
		}
	}
	return trimmed
}

// matchWildcard checks if a command matches a basic wildcard pattern.
// Supports "*" for any sequence of characters.
func matchWildcard(pattern, command string) bool {
	if !strings.Contains(pattern, "*") {
		return pattern == command
	}

	parts := strings.Split(pattern, "*")
	if len(parts) == 0 {
		return true
	}

	remaining := command
	for i, part := range parts {
		if part == "" {
			continue
		}
		if i == 0 {
			if !strings.HasPrefix(remaining, part) {
				return false
			}
			remaining = strings.TrimPrefix(remaining, part)
		} else {
			pos := strings.Index(remaining, part)
			if pos < 0 {
				return false
			}
			remaining = remaining[pos+len(part):]
		}
	}
	return true
}

// ExcludeEngine manages a set of compiled exclude rules.
type ExcludeEngine struct {
	rules []ExcludeRule
}

// NewExcludeEngine creates an exclude engine from a list of raw patterns.
func NewExcludeEngine(patterns []string) *ExcludeEngine {
	rules := make([]ExcludeRule, 0, len(patterns))
	for _, p := range patterns {
		rules = append(rules, CompileExcludePattern(p))
	}
	return &ExcludeEngine{rules: rules}
}

// IsExcluded checks if a command matches any exclude rule.
func (e *ExcludeEngine) IsExcluded(command string) bool {
	for _, rule := range e.rules {
		if MatchExcludeRule(rule, command) {
			return true
		}
	}
	return false
}

// AddPattern adds a new exclude pattern to the engine.
func (e *ExcludeEngine) AddPattern(pattern string) {
	e.rules = append(e.rules, CompileExcludePattern(pattern))
}

// Patterns returns all raw patterns.
func (e *ExcludeEngine) Patterns() []string {
	patterns := make([]string, 0, len(e.rules))
	for _, r := range e.rules {
		patterns = append(patterns, r.Raw)
	}
	return patterns
}
