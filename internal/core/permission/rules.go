package permission

import "fmt"

// RuleSource records where a permission rule originated so later layers can explain decisions consistently.
type RuleSource string

const (
	// RuleSourceUserSettings identifies rules loaded from user-scoped settings.
	RuleSourceUserSettings RuleSource = "userSettings"
	// RuleSourceProjectSettings identifies rules loaded from project-shared settings.
	RuleSourceProjectSettings RuleSource = "projectSettings"
	// RuleSourceLocalSettings identifies rules loaded from machine-local project settings.
	RuleSourceLocalSettings RuleSource = "localSettings"
	// RuleSourceFlagSettings identifies rules synthesized from process flags.
	RuleSourceFlagSettings RuleSource = "flagSettings"
	// RuleSourcePolicySettings identifies rules forced by managed policy settings.
	RuleSourcePolicySettings RuleSource = "policySettings"
	// RuleSourceCLIArg identifies rules injected directly by CLI arguments.
	RuleSourceCLIArg RuleSource = "cliArg"
	// RuleSourceCommand identifies rules attached to a command invocation.
	RuleSourceCommand RuleSource = "command"
	// RuleSourceSession identifies rules granted only for the current session.
	RuleSourceSession RuleSource = "session"
)

// Valid reports whether the rule source is part of the supported minimal model.
func (s RuleSource) Valid() bool {
	switch s {
	case RuleSourceUserSettings,
		RuleSourceProjectSettings,
		RuleSourceLocalSettings,
		RuleSourceFlagSettings,
		RuleSourcePolicySettings,
		RuleSourceCLIArg,
		RuleSourceCommand,
		RuleSourceSession:
		return true
	default:
		return false
	}
}

// Rule describes one normalized filesystem permission rule.
type Rule struct {
	// Source records where the rule came from for later diagnostics and error reporting.
	Source RuleSource
	// Decision declares whether a match allows, denies, or prompts for approval.
	Decision Decision
	// BaseDir scopes the rule matcher root; an empty string means the caller must supply the effective root later.
	BaseDir string
	// Pattern stores the minimal path-matching expression that later matching logic will interpret.
	Pattern string
}

// Validate ensures the rule shape is usable by later matching and error-reporting steps.
func (r Rule) Validate() error {
	if !r.Source.Valid() {
		return fmt.Errorf("permission: unsupported rule source %q", r.Source)
	}
	if !r.Decision.Valid() {
		return fmt.Errorf("permission: unsupported rule decision %q", r.Decision)
	}
	if r.Pattern == "" {
		return fmt.Errorf("permission: rule pattern is required")
	}
	return nil
}

// RuleSet groups read and write rules separately so tool checks can evaluate the correct branch directly.
type RuleSet struct {
	// Read contains rules considered during read-style permission checks.
	Read []Rule
	// Write contains rules considered during write-style permission checks.
	Write []Rule
}

// Validate ensures both read and write rule collections only contain supported rule definitions.
func (s RuleSet) Validate() error {
	for _, rule := range s.Read {
		if err := rule.Validate(); err != nil {
			return err
		}
	}
	for _, rule := range s.Write {
		if err := rule.Validate(); err != nil {
			return err
		}
	}
	return nil
}
