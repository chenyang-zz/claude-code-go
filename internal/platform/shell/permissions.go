package shell

import (
	"fmt"
	"regexp"
	"strings"

	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

var bashRulePattern = regexp.MustCompile(`^\s*Bash\((.*)\)\s*$`)

// PermissionEvaluation stores the normalized permission outcome for one Bash command.
type PermissionEvaluation struct {
	// Decision reports whether the command is allowed, denied, or still needs approval.
	Decision corepermission.Decision
	// Rule stores the matched Bash(...) rule string when one participated in the decision.
	Rule string
	// Message stores the stable caller-facing explanation for deny or ask outcomes.
	Message string
}

// PermissionChecker evaluates the minimal migrated Bash(...) allow/deny/ask rule set.
type PermissionChecker struct {
	// allow stores normalized allow rules extracted from permissions.allow.
	allow []bashPermissionRule
	// deny stores normalized deny rules extracted from permissions.deny.
	deny []bashPermissionRule
	// ask stores normalized ask rules extracted from permissions.ask.
	ask []bashPermissionRule
}

// NewPermissionChecker builds one minimal Bash permission evaluator from the resolved config snapshot.
func NewPermissionChecker(cfg coreconfig.PermissionConfig) *PermissionChecker {
	return &PermissionChecker{
		allow: parseBashPermissionRules(cfg.Allow),
		deny:  parseBashPermissionRules(cfg.Deny),
		ask:   parseBashPermissionRules(cfg.Ask),
	}
}

// Check evaluates one command against deny, ask, and allow rules in that priority order.
func (c *PermissionChecker) Check(command string) PermissionEvaluation {
	normalized := normalizeCommandForPermission(command)
	if normalized == "" {
		return PermissionEvaluation{
			Decision: corepermission.DecisionAsk,
			Message:  "Claude requested permissions to execute an empty command, but you haven't granted it yet.",
		}
	}

	if rule, ok := matchPermissionRule(c.deny, normalized); ok {
		logger.DebugCF("bash_permissions", "bash command denied by rule", map[string]any{
			"command": normalized,
			"rule":    rule.raw,
		})
		return PermissionEvaluation{
			Decision: corepermission.DecisionDeny,
			Rule:     rule.raw,
			Message:  fmt.Sprintf("Permission to execute %q has been denied.", normalized),
		}
	}
	if rule, ok := matchPermissionRule(c.ask, normalized); ok {
		logger.DebugCF("bash_permissions", "bash command requires approval by rule", map[string]any{
			"command": normalized,
			"rule":    rule.raw,
		})
		return PermissionEvaluation{
			Decision: corepermission.DecisionAsk,
			Rule:     rule.raw,
			Message:  fmt.Sprintf("Claude requested permissions to execute %q, but you haven't granted it yet.", normalized),
		}
	}
	if rule, ok := matchPermissionRule(c.allow, normalized); ok {
		logger.DebugCF("bash_permissions", "bash command allowed by rule", map[string]any{
			"command": normalized,
			"rule":    rule.raw,
		})
		return PermissionEvaluation{
			Decision: corepermission.DecisionAllow,
			Rule:     rule.raw,
		}
	}

	logger.DebugCF("bash_permissions", "bash command requires approval by default", map[string]any{
		"command": normalized,
	})
	return PermissionEvaluation{
		Decision: corepermission.DecisionAsk,
		Message:  fmt.Sprintf("Claude requested permissions to execute %q, but you haven't granted it yet.", normalized),
	}
}

// bashPermissionMatchMode describes the minimal rule shapes supported by the migrated checker.
type bashPermissionMatchMode string

const (
	// bashPermissionMatchAll matches every Bash command.
	bashPermissionMatchAll bashPermissionMatchMode = "all"
	// bashPermissionMatchExact matches the fully normalized command string.
	bashPermissionMatchExact bashPermissionMatchMode = "exact"
	// bashPermissionMatchPrefix matches the normalized command prefix when the rule ends in :*.
	bashPermissionMatchPrefix bashPermissionMatchMode = "prefix"
)

// bashPermissionRule stores one normalized Bash(...) permission rule.
type bashPermissionRule struct {
	// raw stores the original settings string for diagnostics.
	raw string
	// pattern stores the normalized exact or prefix matcher payload.
	pattern string
	// mode stores which minimal matcher shape should be applied.
	mode bashPermissionMatchMode
}

// parseBashPermissionRules filters one string slice down to supported Bash(...) rules.
func parseBashPermissionRules(values []string) []bashPermissionRule {
	if len(values) == 0 {
		return nil
	}

	rules := make([]bashPermissionRule, 0, len(values))
	for _, raw := range values {
		rule, ok := parseBashPermissionRule(raw)
		if !ok {
			continue
		}
		rules = append(rules, rule)
	}
	if len(rules) == 0 {
		return nil
	}
	return rules
}

// parseBashPermissionRule normalizes one Bash(...) rule into the minimal exact/prefix/wildcard matcher.
func parseBashPermissionRule(raw string) (bashPermissionRule, bool) {
	matches := bashRulePattern.FindStringSubmatch(raw)
	if len(matches) != 2 {
		return bashPermissionRule{}, false
	}

	pattern := normalizeCommandForPermission(matches[1])
	if pattern == "" {
		return bashPermissionRule{}, false
	}
	if pattern == "*" {
		return bashPermissionRule{raw: strings.TrimSpace(raw), mode: bashPermissionMatchAll}, true
	}
	if strings.HasSuffix(pattern, ":*") {
		prefix := normalizeCommandForPermission(strings.TrimSpace(strings.TrimSuffix(pattern, ":*")))
		if prefix == "" {
			return bashPermissionRule{}, false
		}
		return bashPermissionRule{raw: strings.TrimSpace(raw), pattern: prefix, mode: bashPermissionMatchPrefix}, true
	}

	return bashPermissionRule{raw: strings.TrimSpace(raw), pattern: pattern, mode: bashPermissionMatchExact}, true
}

// matchPermissionRule returns the first rule that matches the normalized command string.
func matchPermissionRule(rules []bashPermissionRule, command string) (bashPermissionRule, bool) {
	for _, rule := range rules {
		if rule.matches(command) {
			return rule, true
		}
	}
	return bashPermissionRule{}, false
}

// matches reports whether the rule matches the normalized Bash command string.
func (r bashPermissionRule) matches(command string) bool {
	switch r.mode {
	case bashPermissionMatchAll:
		return true
	case bashPermissionMatchExact:
		return command == r.pattern
	case bashPermissionMatchPrefix:
		return matchesCommandPrefix(command, r.pattern)
	default:
		return false
	}
}

// matchesCommandPrefix keeps :* rules aligned with the minimal "same command or command + arguments" semantics.
func matchesCommandPrefix(command string, prefix string) bool {
	if command == prefix {
		return true
	}
	if !strings.HasPrefix(command, prefix) || len(command) <= len(prefix) {
		return false
	}

	remainder := command[len(prefix):]
	switch {
	case strings.HasPrefix(remainder, " "):
		return true
	case strings.HasPrefix(remainder, "|"):
		return true
	case strings.HasPrefix(remainder, ";"):
		return true
	case strings.HasPrefix(remainder, "&&"):
		return true
	case strings.HasPrefix(remainder, "||"):
		return true
	default:
		return false
	}
}

// normalizeCommandForPermission applies the minimal wrapper/env cleanup needed by exact and :* Bash rules.
func normalizeCommandForPermission(command string) string {
	tokens := strings.Fields(strings.TrimSpace(command))
	if len(tokens) == 0 {
		return ""
	}

	tokens = stripLeadingEnvAssignments(tokens)
	tokens = stripLeadingShellWrappers(tokens)
	return strings.Join(tokens, " ")
}

// stripLeadingEnvAssignments removes leading VAR=value prefixes so allow/deny rules can match the wrapped command.
func stripLeadingEnvAssignments(tokens []string) []string {
	for len(tokens) > 0 && looksLikeEnvAssignment(tokens[0]) {
		tokens = tokens[1:]
	}
	return tokens
}

// stripLeadingShellWrappers removes a small wrapper subset that still behaves like foreground execution in the current batch.
func stripLeadingShellWrappers(tokens []string) []string {
	for len(tokens) > 0 {
		switch tokens[0] {
		case "time", "nohup":
			tokens = tokens[1:]
			if len(tokens) > 0 && tokens[0] == "--" {
				tokens = tokens[1:]
			}
		case "nice":
			tokens = tokens[1:]
			if len(tokens) >= 2 && tokens[0] == "-n" {
				tokens = tokens[2:]
			} else if len(tokens) >= 1 && strings.HasPrefix(tokens[0], "-") && len(tokens[0]) > 1 {
				tokens = tokens[1:]
			}
			if len(tokens) > 0 && tokens[0] == "--" {
				tokens = tokens[1:]
			}
		case "timeout":
			next := stripTimeoutWrapper(tokens)
			if len(next) == len(tokens) {
				return tokens
			}
			tokens = next
		default:
			return tokens
		}
	}
	return tokens
}

// stripTimeoutWrapper removes one leading timeout wrapper when it follows the simple duration form used by the migrated batch.
func stripTimeoutWrapper(tokens []string) []string {
	if len(tokens) < 3 || tokens[0] != "timeout" {
		return tokens
	}

	index := 1
	for index < len(tokens) && strings.HasPrefix(tokens[index], "-") {
		index++
		if index < len(tokens) && !strings.HasPrefix(tokens[index], "-") {
			index++
		}
	}
	if index >= len(tokens) || !looksLikeTimeoutDuration(tokens[index]) {
		return tokens
	}

	index++
	if index < len(tokens) && tokens[index] == "--" {
		index++
	}
	return tokens[index:]
}

// looksLikeEnvAssignment reports whether the token is a simple shell VAR=value prefix.
func looksLikeEnvAssignment(token string) bool {
	if !strings.Contains(token, "=") {
		return false
	}
	key, _, ok := strings.Cut(token, "=")
	if !ok || key == "" {
		return false
	}
	for index, r := range key {
		switch {
		case r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z'):
			continue
		case index > 0 && r >= '0' && r <= '9':
			continue
		default:
			return false
		}
	}
	return true
}

// looksLikeTimeoutDuration reports whether one token matches the small timeout duration subset used by source defaults.
func looksLikeTimeoutDuration(token string) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}
	if len(token) > 1 {
		last := token[len(token)-1]
		if last == 's' || last == 'm' || last == 'h' || last == 'd' {
			token = token[:len(token)-1]
		}
	}
	if token == "" {
		return false
	}
	for _, r := range token {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
