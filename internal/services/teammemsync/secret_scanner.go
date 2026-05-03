// Package teammemsync implements the Team Memory Sync service.
package teammemsync

import (
	"regexp"
	"strings"
	"sync"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// SecretMatch represents a detected secret in scanned content.
// The matched text is intentionally NOT returned — we never log or expose
// secret values.
type SecretMatch struct {
	RuleID string
	Label  string
}

// secretRule defines a detection pattern for a single secret type.
type secretRule struct {
	id     string
	source string
}

// compiledRule holds a lazily compiled regex for a single rule.
type compiledRule struct {
	id string
	re *regexp.Regexp
}

// secretRules is the curated list of secret detection patterns ported from
// gitleaks (https://github.com/gitleaks/gitleaks, MIT license). Only rules
// with distinctive prefixes and near-zero false-positive rates are included.
// Generic keyword-context rules are omitted.
var secretRules = []secretRule{
	// ── Cloud providers ───────────────────────────────────────────────
	{
		id:     "aws-access-token",
		source: `\b((?:A3T[A-Z0-9]|AKIA|ASIA|ABIA|ACCA)[A-Z2-7]{16})\b`,
	},
	{
		id: "gcp-api-key",
		source: `\b(AIza[\w-]{35})(?:[\x60'"\s;]|\\[nr]|$)`,
	},
	{
		id: "azure-ad-client-secret",
		source: `(?:^|[\\'\x60\s>=:(,)])([a-zA-Z0-9_~.]{3}\dQ~[a-zA-Z0-9_~.-]{31,34})(?:$|[\\'\x60\s<),])`,
	},
	{
		id: "digitalocean-pat",
		source: `\b(dop_v1_[a-f0-9]{64})(?:[\x60'"\s;]|\\[nr]|$)`,
	},
	{
		id: "digitalocean-access-token",
		source: `\b(doo_v1_[a-f0-9]{64})(?:[\x60'"\s;]|\\[nr]|$)`,
	},

	// ── AI APIs ──────────────────────────────────────────────────────
	{
		id: "anthropic-api-key",
		source: `\b(sk-ant-api03-[a-zA-Z0-9_-]{93}AA)(?:[\x60'"\s;]|\\[nr]|$)`,
	},
	{
		id: "anthropic-admin-api-key",
		source: `\b(sk-ant-admin01-[a-zA-Z0-9_-]{93}AA)(?:[\x60'"\s;]|\\[nr]|$)`,
	},
	{
		id: "openai-api-key",
		source: `\b(sk-(?:proj|svcacct|admin)-(?:[A-Za-z0-9_-]{74}|[A-Za-z0-9_-]{58})T3BlbkFJ(?:[A-Za-z0-9_-]{74}|[A-Za-z0-9_-]{58})\b|sk-[a-zA-Z0-9]{20}T3BlbkFJ[a-zA-Z0-9]{20})(?:[\x60'"\s;]|\\[nr]|$)`,
	},
	{
		id: "huggingface-access-token",
		source: `\b(hf_[a-zA-Z]{34})(?:[\x60'"\s;]|\\[nr]|$)`,
	},

	// ── Version control ──────────────────────────────────────────────
	{
		id:     "github-pat",
		source: `ghp_[0-9a-zA-Z]{36}`,
	},
	{
		id:     "github-fine-grained-pat",
		source: `github_pat_\w{82}`,
	},
	{
		id:     "github-app-token",
		source: `(?:ghu|ghs)_[0-9a-zA-Z]{36}`,
	},
	{
		id:     "github-oauth",
		source: `gho_[0-9a-zA-Z]{36}`,
	},
	{
		id:     "github-refresh-token",
		source: `ghr_[0-9a-zA-Z]{36}`,
	},
	{
		id:     "gitlab-pat",
		source: `glpat-[\w-]{20}`,
	},
	{
		id: "gitlab-deploy-token",
		source: `gldt-[0-9a-zA-Z_\-]{20}`,
	},

	// ── Communication ────────────────────────────────────────────────
	{
		id:     "slack-bot-token",
		source: `xoxb-[0-9]{10,13}-[0-9]{10,13}[a-zA-Z0-9-]*`,
	},
	{
		id:     "slack-user-token",
		source: `xox[pe](?:-[0-9]{10,13}){3}-[a-zA-Z0-9-]{28,34}`,
	},
	{
		id:     "slack-app-token",
		source: `(?i)xapp-\d-[A-Z0-9]+-\d+-[a-z0-9]+`,
	},
	{
		id:     "twilio-api-key",
		source: `SK[0-9a-fA-F]{32}`,
	},
	{
		id: "sendgrid-api-token",
		source: `\b(SG\.[a-zA-Z0-9=_\-.]{66})(?:[\x60'"\s;]|\\[nr]|$)`,
	},

	// ── Dev tooling ──────────────────────────────────────────────────
	{
		id: "npm-access-token",
		source: `\b(npm_[a-zA-Z0-9]{36})(?:[\x60'"\s;]|\\[nr]|$)`,
	},
	{
		id:     "pypi-upload-token",
		source: `pypi-AgEIcHlwaS5vcmc[\w-]{50,1000}`,
	},
	{
		id: "databricks-api-token",
		source: `\b(dapi[a-f0-9]{32}(?:-\d)?)(?:[\x60'"\s;]|\\[nr]|$)`,
	},
	{
		id:     "hashicorp-tf-api-token",
		source: `[a-zA-Z0-9]{14}\.atlasv1\.[a-zA-Z0-9\-_=]{60,70}`,
	},
	{
		id: "pulumi-api-token",
		source: `\b(pul-[a-f0-9]{40})(?:[\x60'"\s;]|\\[nr]|$)`,
	},
	{
		id: "postman-api-token",
		source: `\b(PMAK-[a-fA-F0-9]{24}-[a-fA-F0-9]{34})(?:[\x60'"\s;]|\\[nr]|$)`,
	},

	// ── Observability ────────────────────────────────────────────────
	{
		id: "grafana-api-key",
		source: `\b(eyJrIjoi[A-Za-z0-9+/]{70,400}={0,3})(?:[\x60'"\s;]|\\[nr]|$)`,
	},
	{
		id: "grafana-cloud-api-token",
		source: `\b(glc_[A-Za-z0-9+/]{32,400}={0,3})(?:[\x60'"\s;]|\\[nr]|$)`,
	},
	{
		id: "grafana-service-account-token",
		source: `\b(glsa_[A-Za-z0-9]{32}_[A-Fa-f0-9]{8})(?:[\x60'"\s;]|\\[nr]|$)`,
	},
	{
		id: "sentry-user-token",
		source: `\b(sntryu_[a-f0-9]{64})(?:[\x60'"\s;]|\\[nr]|$)`,
	},
	{
		id: "sentry-org-token",
		source: `\bsntrys_eyJpYXQiO[a-zA-Z0-9+/]{10,200}(?:LCJyZWdpb25fdXJs|InJlZ2lvbl91cmwi|cmVnaW9uX3VybCI6)[a-zA-Z0-9+/]{10,200}={0,2}_[a-zA-Z0-9+/]{43}`,
	},

	// ── Payment / commerce ───────────────────────────────────────────
	{
		id: "stripe-access-token",
		source: `\b((?:sk|rk)_(?:test|live|prod)_[a-zA-Z0-9]{10,99})(?:[\x60'"\s;]|\\[nr]|$)`,
	},
	{
		id:     "shopify-access-token",
		source: `shpat_[a-fA-F0-9]{32}`,
	},
	{
		id:     "shopify-shared-secret",
		source: `shpss_[a-fA-F0-9]{32}`,
	},

	// ── Crypto ───────────────────────────────────────────────────────
	{
		id: "private-key",
		source: `(?i)-----BEGIN[ A-Z0-9_-]{0,100}PRIVATE KEY(?: BLOCK)?-----[\s\S-]{64,}-----END[ A-Z0-9_-]{0,100}PRIVATE KEY(?: BLOCK)?-----`,
	},
}

var (
	compiledRules []compiledRule
	compileOnce   sync.Once
)

// getCompiledRules returns the lazily compiled regex rules.
// Compilation happens exactly once on first call.
func getCompiledRules() []compiledRule {
	compileOnce.Do(func() {
		compiledRules = make([]compiledRule, len(secretRules))
		for i, r := range secretRules {
			compiledRules[i] = compiledRule{
				id: r.id,
				re: regexp.MustCompile(r.source),
			}
			logger.DebugCF("teammemsync", "compiled secret rule", map[string]any{
				"ruleId": r.id,
			})
		}
	})
	return compiledRules
}

// ScanForSecrets scans content for potential secrets and returns deduplicated
// matches. Returns one match per unique rule ID that fired. The matched text
// is intentionally NOT returned — we never log or expose secret values.
func ScanForSecrets(content string) []SecretMatch {
	rules := getCompiledRules()
	seen := make(map[string]bool, len(rules))
	var matches []SecretMatch

	for _, rule := range rules {
		if seen[rule.id] {
			continue
		}
		if rule.re.MatchString(content) {
			seen[rule.id] = true
			matches = append(matches, SecretMatch{
				RuleID: rule.id,
				Label:  GetSecretLabel(rule.id),
			})
		}
	}

	return matches
}

// GetSecretLabel converts a kebab-case rule ID to a human-readable label.
// For example, "github-pat" becomes "GitHub PAT".
func GetSecretLabel(ruleID string) string {
	// Words where the canonical capitalization differs from title case.
	specialCase := map[string]string{
		"aws":          "AWS",
		"gcp":          "GCP",
		"api":          "API",
		"pat":          "PAT",
		"ad":           "AD",
		"tf":           "TF",
		"oauth":        "OAuth",
		"npm":          "NPM",
		"pypi":         "PyPI",
		"jwt":          "JWT",
		"github":       "GitHub",
		"gitlab":       "GitLab",
		"openai":       "OpenAI",
		"digitalocean": "DigitalOcean",
		"huggingface":  "HuggingFace",
		"hashicorp":    "HashiCorp",
		"sendgrid":     "SendGrid",
	}

	parts := strings.Split(ruleID, "-")
	for i, part := range parts {
		if replacement, ok := specialCase[part]; ok {
			parts[i] = replacement
		} else {
			parts[i] = capitalize(part)
		}
	}
	return strings.Join(parts, " ")
}

// capitalize returns s with the first character converted to uppercase.
func capitalize(s string) string {
	if len(s) == 0 {
		return ""
	}
	runes := []rune(s)
	return strings.ToUpper(string(runes[0])) + string(runes[1:])
}
