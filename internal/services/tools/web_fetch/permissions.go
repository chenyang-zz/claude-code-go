package web_fetch

import (
	"fmt"
	"net/url"
	"strings"

	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
)

// PermissionChecker evaluates WebFetch allow/deny/ask rules by hostname.
type PermissionChecker struct {
	allow []string
	deny  []string
	ask   []string
}

// NewPermissionChecker builds a WebFetch permission evaluator from the resolved config.
func NewPermissionChecker(allow, deny, ask []string) *PermissionChecker {
	return &PermissionChecker{
		allow: normalizeRules(allow),
		deny:  normalizeRules(deny),
		ask:   normalizeRules(ask),
	}
}

// Check evaluates one URL against deny, ask, and allow rules in that priority order.
func (c *PermissionChecker) Check(rawURL string) corepermission.Evaluation {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return corepermission.Evaluation{
			Decision: corepermission.DecisionAsk,
			Message:  "Claude requested permissions to use WebFetch, but the URL could not be parsed.",
		}
	}

	hostname := parsed.Hostname()

	if isPreapprovedHost(hostname, parsed.Path) {
		return corepermission.Evaluation{
			Decision: corepermission.DecisionAllow,
			Message:  "Preapproved host",
		}
	}

	if rule, ok := matchRule(c.deny, hostname); ok {
		return corepermission.Evaluation{
			Decision: corepermission.DecisionDeny,
			Rule: &corepermission.Rule{
				Source:   corepermission.RuleSourceUserSettings,
				Decision: corepermission.DecisionDeny,
				Pattern:  rule,
			},
			Message: fmt.Sprintf("WebFetch denied access to domain:%s.", hostname),
		}
	}

	if rule, ok := matchRule(c.ask, hostname); ok {
		return corepermission.Evaluation{
			Decision: corepermission.DecisionAsk,
			Rule: &corepermission.Rule{
				Source:   corepermission.RuleSourceUserSettings,
				Decision: corepermission.DecisionAsk,
				Pattern:  rule,
			},
			Message: "Claude requested permissions to use WebFetch, but you haven't granted it yet.",
		}
	}

	if rule, ok := matchRule(c.allow, hostname); ok {
		return corepermission.Evaluation{
			Decision: corepermission.DecisionAllow,
			Rule: &corepermission.Rule{
				Source:   corepermission.RuleSourceUserSettings,
				Decision: corepermission.DecisionAllow,
				Pattern:  rule,
			},
			Message: fmt.Sprintf("WebFetch allowed access to domain:%s by rule.", hostname),
		}
	}

	return corepermission.Evaluation{
		Decision: corepermission.DecisionAsk,
		Message:  "Claude requested permissions to use WebFetch, but you haven't granted it yet.",
	}
}

// normalizeRules trims and lower-cases rules for consistent matching.
func normalizeRules(rules []string) []string {
	if len(rules) == 0 {
		return nil
	}
	result := make([]string, 0, len(rules))
	for _, r := range rules {
		trimmed := strings.TrimSpace(r)
		if trimmed == "" {
			continue
		}
		result = append(result, strings.ToLower(trimmed))
	}
	return result
}

// matchRule returns the first rule that matches the given hostname.
func matchRule(rules []string, hostname string) (string, bool) {
	hostLower := strings.ToLower(hostname)
	for _, rule := range rules {
		if ruleMatchesHost(rule, hostLower) {
			return rule, true
		}
	}
	return "", false
}

// ruleMatchesHost reports whether a permission rule applies to the given hostname.
func ruleMatchesHost(rule, hostname string) bool {
	// Exact match.
	if rule == hostname {
		return true
	}
	// WebFetch(domain:...) or domain:... match.
	if strings.Contains(rule, "domain:"+hostname) {
		return true
	}
	// Wildcard: "*" matches everything.
	if rule == "*" {
		return true
	}
	return false
}
