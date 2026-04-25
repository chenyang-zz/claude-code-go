package shared

import (
	"path/filepath"
	"regexp"
	"strings"
)

type secretRule struct {
	id      string
	label   string
	pattern *regexp.Regexp
}

// SecretMatch describes one secret rule that matched a piece of content.
type SecretMatch struct {
	// RuleID stores the stable detector identifier used by the scanner.
	RuleID string
	// Label stores the human-readable label shown in rejection messages.
	Label string
}

var teamMemorySecretRules = []secretRule{
	{id: "aws-access-token", label: "AWS access token", pattern: regexp.MustCompile(`\b((?:A3T[A-Z0-9]|AKIA|ASIA|ABIA|ACCA)[A-Z2-7]{16})\b`)},
	{id: "anthropic-api-key", label: "Anthropic API key", pattern: regexp.MustCompile(`\b(sk-ant-api03-[A-Za-z0-9_\-]{93}AA)(?:[\x60'"\s;]|\\[nr]|$)`)},
	{id: "anthropic-admin-api-key", label: "Anthropic admin API key", pattern: regexp.MustCompile(`\b(sk-ant-admin01-[A-Za-z0-9_\-]{93}AA)(?:[\x60'"\s;]|\\[nr]|$)`)},
	{id: "openai-api-key", label: "OpenAI API key", pattern: regexp.MustCompile(`\b(sk-(?:proj|svcacct|admin)-(?:[A-Za-z0-9_-]{74}|[A-Za-z0-9_-]{58})T3BlbkFJ(?:[A-Za-z0-9_-]{74}|[A-Za-z0-9_-]{58})|sk-[a-zA-Z0-9]{20}T3BlbkFJ[a-zA-Z0-9]{20})(?:[\x60'"\s;]|\\[nr]|$)`)},
	{id: "github-pat", label: "GitHub PAT", pattern: regexp.MustCompile(`ghp_[0-9a-zA-Z]{36}`)},
	{id: "slack-bot-token", label: "Slack bot token", pattern: regexp.MustCompile(`xoxb-[0-9]{10,13}-[0-9]{10,13}[a-zA-Z0-9-]*`)},
	{id: "gitlab-pat", label: "GitLab PAT", pattern: regexp.MustCompile(`glpat-[\w-]{20}`)},
	{id: "stripe-access-token", label: "Stripe access token", pattern: regexp.MustCompile(`\b((?:sk|rk)_(?:test|live|prod)_[a-zA-Z0-9]{10,99})(?:[\x60'"\s;]|\\[nr]|$)`)},
}

// IsTeamMemoryPath reports whether the given path points into a team memory tree.
func IsTeamMemoryPath(filePath string) bool {
	normalized := strings.ToLower(filepath.ToSlash(filepath.Clean(filePath)))
	return strings.Contains(normalized, "/memory/team/") || strings.HasSuffix(normalized, "/memory/team")
}

// ScanForSecrets returns the set of secret rules that match the provided content.
func ScanForSecrets(content string) []SecretMatch {
	if content == "" {
		return nil
	}

	matches := make([]SecretMatch, 0, len(teamMemorySecretRules))
	for _, rule := range teamMemorySecretRules {
		if rule.pattern.FindStringIndex(content) == nil {
			continue
		}
		matches = append(matches, SecretMatch{
			RuleID: rule.id,
			Label:  rule.label,
		})
	}
	return matches
}

// CheckTeamMemorySecrets rejects secret content when the target path is team memory.
func CheckTeamMemorySecrets(filePath string, content string) string {
	if !IsTeamMemoryPath(filePath) {
		return ""
	}

	matches := ScanForSecrets(content)
	if len(matches) == 0 {
		return ""
	}

	labels := make([]string, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		if _, ok := seen[match.Label]; ok {
			continue
		}
		seen[match.Label] = struct{}{}
		labels = append(labels, match.Label)
	}

	return "Content contains potential secrets (" + strings.Join(labels, ", ") + ") and cannot be written to team memory. Team memory is shared with all repository collaborators. Remove the sensitive content and try again."
}
