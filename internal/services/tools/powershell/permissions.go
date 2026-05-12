package powershell

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	platformshell "github.com/sheepzhao/claude-code-go/internal/platform/shell"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// psRulePattern matches PowerShell(...) permission rules, case-insensitively.
var psRulePattern = regexp.MustCompile(`(?i)^\s*PowerShell\((.*)\)\s*$`)

// PermissionChecker evaluates PowerShell(...) allow/deny/ask permission rules,
// with PowerShell-specific normalization (case-insensitive cmdlet matching,
// alias resolution to canonical names). When fsPolicy is set, extracted file
// paths are also validated against the filesystem deny/allow rules.
type PermissionChecker struct {
	allow    []psPermissionRule
	deny     []psPermissionRule
	ask      []psPermissionRule
	fsPolicy *corepermission.FilesystemPolicy
}

// NewPermissionChecker builds a PowerShell permission evaluator from the
// resolved config snapshot, filtering for PowerShell(...) rules.
// The fsPolicy parameter is optional (nil means no filesystem policy checks).
func NewPermissionChecker(cfg coreconfig.PermissionConfig, fsPolicy *corepermission.FilesystemPolicy) *PermissionChecker {
	return &PermissionChecker{
		allow:    parsePSPermissionRules(cfg.Allow),
		deny:     parsePSPermissionRules(cfg.Deny),
		ask:      parsePSPermissionRules(cfg.Ask),
		fsPolicy: fsPolicy,
	}
}

// Check evaluates one PowerShell command against deny, ask, and allow rules
// in that priority order, with PS-specific normalization applied.
func (c *PermissionChecker) Check(command string) platformshell.PermissionEvaluation {
	normalized := normalizePSCommandForPermission(command)
	if normalized == "" {
		return platformshell.PermissionEvaluation{
			Decision:          corepermission.DecisionAsk,
			NormalizedCommand: normalized,
			Message:           "Claude requested permissions to execute an empty PowerShell command, but you haven't granted it yet.",
		}
	}

	if rule, ok := matchPSPermissionRule(c.deny, normalized); ok {
		logger.DebugCF("powershell_permissions", "powershell command denied by rule", map[string]any{
			"command": normalized,
			"rule":    rule.raw,
		})
		return platformshell.PermissionEvaluation{
			Decision:          corepermission.DecisionDeny,
			NormalizedCommand: normalized,
			Rule:              rule.raw,
			Message:           fmt.Sprintf("Permission to execute PowerShell command %q has been denied.", normalized),
		}
	}
	if rule, ok := matchPSPermissionRule(c.ask, normalized); ok {
		logger.DebugCF("powershell_permissions", "powershell command requires approval by rule", map[string]any{
			"command": normalized,
			"rule":    rule.raw,
		})
		return platformshell.PermissionEvaluation{
			Decision:          corepermission.DecisionAsk,
			NormalizedCommand: normalized,
			Rule:              rule.raw,
			Message:           fmt.Sprintf("Claude requested permissions to execute %q, but you haven't granted it yet.", normalized),
		}
	}
	if rule, ok := matchPSPermissionRule(c.allow, normalized); ok {
		logger.DebugCF("powershell_permissions", "powershell command allowed by rule", map[string]any{
			"command": normalized,
			"rule":    rule.raw,
		})
		return platformshell.PermissionEvaluation{
			Decision:          corepermission.DecisionAllow,
			NormalizedCommand: normalized,
			Rule:              rule.raw,
		}
	}

	logger.DebugCF("powershell_permissions", "powershell command requires approval by default", map[string]any{
		"command": normalized,
	})
	return platformshell.PermissionEvaluation{
		Decision:          corepermission.DecisionAsk,
		NormalizedCommand: normalized,
		Message:           fmt.Sprintf("Claude requested permissions to execute %q, but you haven't granted it yet.", normalized),
	}
}

// psPermissionMatchMode describes the rule shapes supported by the PowerShell permission checker.
type psPermissionMatchMode string

const (
	psPermissionMatchAll    psPermissionMatchMode = "all"
	psPermissionMatchExact  psPermissionMatchMode = "exact"
	psPermissionMatchPrefix psPermissionMatchMode = "prefix"
)

// psPermissionRule stores one normalized PowerShell(...) permission rule.
type psPermissionRule struct {
	raw     string
	pattern string
	mode    psPermissionMatchMode
}

// parsePSPermissionRules filters and parses PowerShell(...) rules from a string slice.
func parsePSPermissionRules(values []string) []psPermissionRule {
	if len(values) == 0 {
		return nil
	}

	rules := make([]psPermissionRule, 0, len(values))
	for _, raw := range values {
		rule, ok := parsePSPermissionRule(raw)
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

// parsePSPermissionRule normalizes one PowerShell(...) rule into a matcher.
func parsePSPermissionRule(raw string) (psPermissionRule, bool) {
	matches := psRulePattern.FindStringSubmatch(raw)
	if len(matches) != 2 {
		return psPermissionRule{}, false
	}

	pattern := normalizePSCommandForPermission(matches[1])
	if pattern == "" {
		return psPermissionRule{}, false
	}
	if pattern == "*" {
		return psPermissionRule{raw: strings.TrimSpace(raw), mode: psPermissionMatchAll}, true
	}
	if strings.HasSuffix(pattern, ":*") {
		prefix := normalizePSCommandForPermission(strings.TrimSpace(strings.TrimSuffix(pattern, ":*")))
		if prefix == "" {
			return psPermissionRule{}, false
		}
		return psPermissionRule{raw: strings.TrimSpace(raw), pattern: prefix, mode: psPermissionMatchPrefix}, true
	}

	return psPermissionRule{raw: strings.TrimSpace(raw), pattern: pattern, mode: psPermissionMatchExact}, true
}

// matchPSPermissionRule returns the first rule matching the normalized command.
func matchPSPermissionRule(rules []psPermissionRule, command string) (psPermissionRule, bool) {
	for _, rule := range rules {
		if rule.matches(command) {
			return rule, true
		}
	}
	return psPermissionRule{}, false
}

// matches reports whether the rule matches the normalized PowerShell command string.
func (r psPermissionRule) matches(command string) bool {
	switch r.mode {
	case psPermissionMatchAll:
		return true
	case psPermissionMatchExact:
		return command == r.pattern
	case psPermissionMatchPrefix:
		return matchesPSCommandPrefix(command, r.pattern)
	default:
		return false
	}
}

// matchesPSCommandPrefix implements PowerShell prefix matching: the command
// must start with the prefix, followed by a separator (space, pipe, semicolon,
// chain operator).
func matchesPSCommandPrefix(command string, prefix string) bool {
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

// normalizePSCommandForPermission normalizes a PowerShell command for permission
// matching: lowercases the cmdlet name, resolves aliases to canonical form,
// and strips leading env variable assignments.
func normalizePSCommandForPermission(command string) string {
	tokens := strings.Fields(strings.TrimSpace(command))
	if len(tokens) == 0 {
		return ""
	}

	// Strip leading env assignments (VAR=value)
	tokens = stripPSEnvAssignments(tokens)

	// Lowercase and resolve alias for the first token
	if len(tokens) > 0 {
		first := strings.ToLower(tokens[0])
		if canonical, ok := psCommonAliases[first]; ok {
			tokens[0] = canonical
		} else {
			tokens[0] = first
		}
	}

	return strings.Join(tokens, " ")
}

// stripPSEnvAssignments removes leading VAR=value prefixes.
func stripPSEnvAssignments(tokens []string) []string {
	for len(tokens) > 0 && looksLikeEnvAssignment(tokens[0]) {
		tokens = tokens[1:]
	}
	return tokens
}

// looksLikeEnvAssignment reports whether the token is a simple VAR=value prefix.
func looksLikeEnvAssignment(token string) bool {
	if !strings.Contains(token, "=") {
		return false
	}
	key, _, ok := strings.Cut(token, "=")
	if !ok || key == "" {
		return false
	}
	for i, r := range key {
		switch {
		case r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z'):
			continue
		case i > 0 && r >= '0' && r <= '9':
			continue
		default:
			return false
		}
	}
	return true
}

// firstParsedCommandElement extracts the first CommandAst command element from
// a parsed PowerShell command. Returns an empty element when no command found.
func firstParsedCommandElement(parsed ParsedPowerShellCommand) ParsedCommandElement {
	for _, stmt := range parsed.Statements {
		for _, cmd := range stmt.Commands {
			if cmd.ElementType == "CommandAst" {
				return cmd
			}
		}
	}
	return ParsedCommandElement{}
}

// checkAcceptEditsMode performs the full acceptEdits mode check, mirroring TS
// modeValidation.ts checkPermissionMode. Returns a PermissionDecision when the
// command should be handled by acceptEdits mode (allow or passthrough), or nil
// when acceptEdits does not apply.
func checkAcceptEditsMode(command string, parsed ParsedPowerShellCommand) *PermissionDecision {
	// SECURITY: Check for subexpressions, script blocks, or member invocations
	// that could be used to smuggle arbitrary code through acceptEdits mode.
	secFlags := DeriveSecurityFlags(parsed)
	if secFlags.HasSubExpressions || secFlags.HasScriptBlocks ||
		secFlags.HasMemberInvocations || secFlags.HasSplatting ||
		secFlags.HasAssignments || secFlags.HasExpandableStrings {
		return &PermissionDecision{
			Evaluation: platformshell.PermissionEvaluation{
				Decision:          corepermission.DecisionAsk,
				NormalizedCommand: command,
				Message:           "Command contains subexpressions, script blocks, or member invocations that require approval in acceptEdits mode",
			},
			Reason: "acceptEdits: security flags",
		}
	}

	// Check each statement and each command for acceptEdits eligibility
	allSubCommands := getAcceptEditsSubCommands(parsed)
	if len(allSubCommands) == 0 {
		return &PermissionDecision{
			Evaluation: platformshell.PermissionEvaluation{
				Decision:          corepermission.DecisionAsk,
				NormalizedCommand: command,
				Message:           "No commands found to validate for acceptEdits mode",
			},
			Reason: "acceptEdits: no commands",
		}
	}

	// Check for cwd-changing cmdlets in compound commands (cd desync guard)
	// and symlink-create compounds
	hasCdCommand := false
	hasSymlinkCreate := false
	hasWriteCommand := false
	totalCommands := 0
	for _, stmt := range parsed.Statements {
		for _, cmd := range stmt.Commands {
			if cmd.ElementType != "CommandAst" {
				continue
			}
			totalCommands++
			if isCwdChangingCmdlet(cmd.Name) {
				hasCdCommand = true
			}
			if isSymlinkCreatingCmdlet(cmd.Name, cmd.Args) {
				hasSymlinkCreate = true
			}
			if isAcceptEditsCmdlet(resolvePSCommand(cmd.Name)) {
				hasWriteCommand = true
			}
		}
	}

	// Compound cwd desync guard
	if totalCommands > 1 && hasCdCommand && hasWriteCommand {
		return &PermissionDecision{
			Evaluation: platformshell.PermissionEvaluation{
				Decision:          corepermission.DecisionAsk,
				NormalizedCommand: command,
				Message:           "Compound command contains a directory-changing command (Set-Location/Push-Location/Pop-Location) with a write operation",
			},
			Reason: "acceptEdits: cd + write compound",
		}
	}

	// Symlink-create compound guard
	if totalCommands > 1 && hasSymlinkCreate {
		return &PermissionDecision{
			Evaluation: platformshell.PermissionEvaluation{
				Decision:          corepermission.DecisionAsk,
				NormalizedCommand: command,
				Message:           "Compound command creates a filesystem link (New-Item -ItemType SymbolicLink/Junction/HardLink)",
			},
			Reason: "acceptEdits: symlink compound",
		}
	}

	// Check each command for acceptEdits eligibility
	for _, cmd := range allSubCommands {
		if cmd.ElementType != "CommandAst" {
			return &PermissionDecision{
				Evaluation: platformshell.PermissionEvaluation{
					Decision:          corepermission.DecisionAsk,
					NormalizedCommand: command,
					Message:           "Pipeline contains expression source that cannot be statically validated",
				},
				Reason: "acceptEdits: non-CommandAst element",
			}
		}
		// nameType 'application' rejection
		if cmd.NameType == "application" {
			return &PermissionDecision{
				Evaluation: platformshell.PermissionEvaluation{
					Decision:          corepermission.DecisionAsk,
					NormalizedCommand: command,
					Message:           "Command resolved from a path-like name and requires approval in acceptEdits mode",
				},
				Reason: "acceptEdits: application nameType",
			}
		}
		// Element type whitelist check.
		// NOTE: ElementTypes[0] corresponds to Args[0] (first argument).
		// The parser strips the command name, so types are aligned 1:1 with args.
		if cmd.ElementTypes != nil {
			for i := 0; i < len(cmd.ElementTypes); i++ {
				t := cmd.ElementTypes[i]
				if t != "StringConstant" && t != "Parameter" {
					return &PermissionDecision{
						Evaluation: platformshell.PermissionEvaluation{
							Decision:          corepermission.DecisionAsk,
							NormalizedCommand: command,
							Message:           "Command argument has unvalidatable type in acceptEdits mode",
						},
						Reason: "acceptEdits: unsafe element type",
					}
				}
				// Colon-bound expression detection
				if t == "Parameter" {
					if i < len(cmd.Args) {
						arg := cmd.Args[i]
						colonIdx := strings.Index(arg, ":")
						if colonIdx > 0 {
							val := arg[colonIdx+1:]
							if strings.ContainsAny(val, "$(@{[") {
								return &PermissionDecision{
									Evaluation: platformshell.PermissionEvaluation{
										Decision:          corepermission.DecisionAsk,
										NormalizedCommand: command,
										Message:           "Colon-bound parameter contains an expression that cannot be statically validated",
									},
									Reason: "acceptEdits: colon-bound expression",
								}
							}
						}
					}
				}
			}
		}
		// Check if this is an acceptEdits-allowed cmdlet
		canonical := resolvePSCommand(cmd.Name)
		if !isAcceptEditsCmdlet(resolvePSCommand(cmd.Name)) && !isSafeOutputCmdlet(canonical) {
			return &PermissionDecision{
				Evaluation: platformshell.PermissionEvaluation{
					Decision:          corepermission.DecisionAsk,
					NormalizedCommand: command,
					Message:           "No mode-specific handling for this command in acceptEdits mode",
				},
				Reason: "acceptEdits: not an allowed cmdlet",
			}
		}
	}

	// Also check nested commands from control flow statements
	for _, stmt := range parsed.Statements {
		for _, cmd := range stmt.NestedCommands {
			if cmd.ElementType != "CommandAst" {
				return &PermissionDecision{
					Evaluation: platformshell.PermissionEvaluation{
						Decision:          corepermission.DecisionAsk,
						NormalizedCommand: command,
						Message:           "Nested expression element cannot be statically validated in acceptEdits mode",
					},
					Reason: "acceptEdits: nested non-CommandAst",
				}
			}
			if cmd.NameType == "application" {
				return &PermissionDecision{
					Evaluation: platformshell.PermissionEvaluation{
						Decision:          corepermission.DecisionAsk,
						NormalizedCommand: command,
						Message:           "Nested command resolved from a path-like name in acceptEdits mode",
					},
					Reason: "acceptEdits: nested application",
				}
			}
			canonical := resolvePSCommand(cmd.Name)
			if !isAcceptEditsCmdlet(canonical) && !isSafeOutputCmdlet(canonical) {
				return &PermissionDecision{
					Evaluation: platformshell.PermissionEvaluation{
						Decision:          corepermission.DecisionAsk,
						NormalizedCommand: command,
						Message:           "Nested command not allowed in acceptEdits mode",
					},
					Reason: "acceptEdits: nested not allowed",
				}
			}
		}
	}

	// All commands are acceptable -- auto-allow
	return &PermissionDecision{
		Evaluation: platformshell.PermissionEvaluation{
			Decision:          corepermission.DecisionAllow,
			NormalizedCommand: command,
			Message:           "Command is safe for acceptEdits mode",
		},
		Reason: "acceptEdits: all commands allowed",
	}
}

// getAcceptEditsSubCommands collects all CommandAst commands from the parsed
// command that are eligible for acceptEdits checking.
func getAcceptEditsSubCommands(parsed ParsedPowerShellCommand) []ParsedCommandElement {
	var cmds []ParsedCommandElement
	for _, stmt := range parsed.Statements {
		for _, cmd := range stmt.Commands {
			if cmd.ElementType == "CommandAst" {
				cmds = append(cmds, cmd)
			}
		}
	}
	return cmds
}

// isSymlinkCreatingCmdlet detects New-Item -ItemType SymbolicLink/Junction/HardLink.
// Ported from TS modeValidation.ts isSymlinkCreatingCommand.
func isSymlinkCreatingCmdlet(name string, args []string) bool {
	canonical := resolvePSCommand(name)
	if canonical != "new-item" {
		return false
	}
	linkTypes := map[string]bool{"symboliclink": true, "junction": true, "hardlink": true}
	for _, arg := range args {
		lower := strings.ToLower(arg)
		// Check -ItemType or -Type parameter
		if strings.HasPrefix(lower, "-itemtype") || strings.HasPrefix(lower, "-type") {
			// Check colon-bound value
			if colonIdx := strings.Index(lower, ":"); colonIdx > 0 {
				val := lower[colonIdx+1:]
				if linkTypes[val] {
					return true
				}
			}
		}
	}
	// Also check positional args for -ItemType value
	for i, arg := range args {
		lower := strings.ToLower(arg)
		if lower == "-itemtype" || lower == "-type" || lower == "-it" || lower == "-ty" {
			if i+1 < len(args) {
				val := strings.ToLower(args[i+1])
				if linkTypes[val] {
					return true
				}
			}
		}
	}
	return false
}

// Compile-time interface check.
var _ CommandPermissionChecker = (*PermissionChecker)(nil)

// CommandPermissionChecker describes the minimal PowerShell permission dependency.
type CommandPermissionChecker interface {
	// Check evaluates whether the provided PowerShell command is currently allowed to run.
	Check(command string) platformshell.PermissionEvaluation
}

// checkPermission performs the enhanced permission check with type-safe access
// to the PermissionChecker's CheckEnhanced method.
func checkPermission(cmdChecker CommandPermissionChecker, command string, scanResult ScanResult, approvalMode string, ctx context.Context, workingDir string) PermissionDecision {
	if pc, ok := cmdChecker.(*PermissionChecker); ok {
		return pc.CheckEnhanced(command, scanResult, approvalMode, ctx, workingDir)
	}
	// Fallback to basic Check for non-PermissionChecker implementations
	eval := cmdChecker.Check(command)
	return PermissionDecision{
		Evaluation: eval,
		Reason:     "basic check (no enriched permission checker)",
	}
}

// PermissionDecision aggregates multiple signals into a single permission outcome.
type PermissionDecision struct {
	// Evaluation is the base rule-based decision.
	Evaluation platformshell.PermissionEvaluation
	// Reason explains the final decision for tracing.
	Reason string
}

// CheckEnhanced evaluates a PowerShell command with full awareness of:
// - Compound sub-commands
// - PS cmdlet allowlists (read-only auto-allow)
// - Provider path detection
// - Approval mode integration
// - Security scan results
// - Filesystem policy (deny/allow rules for extracted file paths)
//
// Uses collect-then-reduce decision model (ported from TS powershellPermissions.ts):
// every post-parse check pushes its decision into a slice, then a single reduce
// applies priority: deny > ask > allow > passthrough. This structurally closes
// the ask-before-deny bug class: an 'ask' from an earlier check can no longer
// mask a 'deny' from a later check.
func (c *PermissionChecker) CheckEnhanced(command string, scanResult ScanResult, approvalMode string, ctx context.Context, workingDir string) PermissionDecision {
	normalized := normalizePSCommandForPermission(command)
	if normalized == "" {
		return PermissionDecision{
			Evaluation: platformshell.PermissionEvaluation{
				Decision:          corepermission.DecisionAsk,
				NormalizedCommand: normalized,
				Message:           "Claude requested permissions to execute an empty PowerShell command, but you haven't granted it yet.",
			},
			Reason: "empty command",
		}
	}

	// 0. Hard deny: dangerous removal of protected system paths
	// (Early return — non-negotiable security boundary)
	if isDangerousRemoval(command) {
		return PermissionDecision{
			Evaluation: platformshell.PermissionEvaluation{
				Decision:          corepermission.DecisionDeny,
				NormalizedCommand: normalized,
				Message:           fmt.Sprintf("Permission to execute %q has been denied: targets protected system path.", normalized),
			},
			Reason: "dangerous removal path",
		}
	}

	// 1. Rule-based check (deny/ask/allow rules)
	baseResult := c.Check(command)
	if baseResult.Decision == corepermission.DecisionDeny {
		return PermissionDecision{
			Evaluation: baseResult,
			Reason:     "rule deny",
		}
	}

	// Parse AST once, reused by all post-parse checks
	parsed := ParsePowerShellCommand(command)
	firstCmdElement := firstParsedCommandElement(parsed)

	// ========================================================================
	// COLLECT PHASE
	// ========================================================================
	// Every post-parse check pushes its decision into this slice. No early
	// returns — all checks run so that a 'deny' from a later check cannot
	// be masked by an 'ask' from an earlier check.
	// Ported from TS powershellPermissions.ts collect-then-reduce (step 3-4).
	var decisions []PermissionDecision

	// 2. Compound command: split into sub-commands and evaluate each independently.
	// This mirrors TS step 5: each pipeline segment or statement separator
	// creates an independent sub-command.
	subCmds := splitSubCommands(command)
	if len(subCmds) > 1 {
		var subAskReasons []string
		for _, subCmd := range subCmds {
			subResult := c.Check(subCmd)
			switch subResult.Decision {
			case corepermission.DecisionDeny:
				decisions = append(decisions, PermissionDecision{
					Evaluation: subResult,
					Reason:     "sub-command deny: " + subCmd,
				})
			case corepermission.DecisionAsk:
				// Check each gate independently for this sub-command
				subScan := NewSecurityScanner().Scan(subCmd)
				if subScan.Level >= RiskLevelDangerous {
					subAskReasons = append(subAskReasons, subCmd)
					continue
				}
				if hasProviderPath(subCmd) {
					subAskReasons = append(subAskReasons, subCmd)
					continue
				}
				if !isReadOnlyPSCmdlet(subCmd) {
					if checkArgLeaks(subCmd) {
						subAskReasons = append(subAskReasons, subCmd)
						continue
					}
					if !(approvalMode == "acceptEdits" && isAcceptEditsCmdlet(subCmd)) {
						subAskReasons = append(subAskReasons, subCmd)
						continue
					}
				}
				// Read-only or acceptEdits-allowed sub-command — auto-pass
			case corepermission.DecisionAllow:
				// Allowed sub-command — auto-pass
			}
		}

		if len(subAskReasons) > 0 {
			decisions = append(decisions, PermissionDecision{
				Evaluation: platformshell.PermissionEvaluation{
					Decision:          corepermission.DecisionAsk,
					NormalizedCommand: normalized,
					Message:           fmt.Sprintf("Compound command has %d sub-command(s) that require approval", len(subAskReasons)),
				},
				Reason: "compound: sub-commands need approval",
			})
		}
		// No early return: all sub-commands checked, continue collecting.
	}

	// 3. Security scan: dangerous commands always require approval
	if scanResult.Level >= RiskLevelDangerous {
		decisions = append(decisions, PermissionDecision{
			Evaluation: platformshell.PermissionEvaluation{
				Decision:          corepermission.DecisionAsk,
				NormalizedCommand: normalized,
				Message:           scanResult.Message,
			},
			Reason: "security: " + scanResult.Message,
		})
	}

	// 4. Post-parse checks (require valid AST)
	if parsed.Valid {
		// 4a. Path constraints: parse command and validate file paths
		pathResult := checkPathConstraints(command, &parsed)
		if pathResult.ShouldAsk {
			decisions = append(decisions, PermissionDecision{
				Evaluation: platformshell.PermissionEvaluation{
					Decision:          corepermission.DecisionAsk,
					NormalizedCommand: normalized,
					Message:           pathResult.Message,
				},
				Reason: "path constraint: " + pathResult.Message,
			})
		}

		// 4b. Filesystem policy: validate extracted paths against
		// filesystem deny/allow rules. Each extracted path is checked with
		// the appropriate access type (read/write) from CMDLET_PATH_CONFIG.
		// ALL paths are checked (no early return) to collect all signals.
		if c.fsPolicy != nil && len(pathResult.ExtractedPaths) > 0 {
			for _, ep := range pathResult.ExtractedPaths {
				access := corepermission.AccessWrite
				if ep.OperationType == opRead {
					access = corepermission.AccessRead
				}
				fsEval := c.fsPolicy.EvaluateFilesystem(ctx, corepermission.FilesystemRequest{
					ToolName:   Name,
					Path:       ep.Path,
					WorkingDir: workingDir,
					Access:     access,
				})
				if fsEval.Decision == corepermission.DecisionDeny || fsEval.Decision == corepermission.DecisionAsk {
					decisions = append(decisions, PermissionDecision{
						Evaluation: platformshell.PermissionEvaluation{
							Decision:          fsEval.Decision,
							NormalizedCommand: normalized,
							Message:           fsEval.Message,
						},
						Reason: "filesystem policy " + string(fsEval.Decision) + ": " + ep.Path,
					})
				}
			}
		}

		// 4c. Security flags from AST: reject commands with subexpressions,
		// script blocks, member invocations, splatting, assignments, etc.
		secFlags := DeriveSecurityFlags(parsed)
		if secFlags.HasScriptBlocks || secFlags.HasSubExpressions ||
			secFlags.HasMemberInvocations || secFlags.HasSplatting ||
			secFlags.HasAssignments || secFlags.HasExpandableStrings {
			decisions = append(decisions, PermissionDecision{
				Evaluation: platformshell.PermissionEvaluation{
					Decision:          corepermission.DecisionAsk,
					NormalizedCommand: normalized,
					Message:           "Command contains subexpressions, script blocks, or member invocations that require approval",
				},
				Reason: "security flags",
			})
		}

		// 4d. Using statements — invisible to command block walk
		if parsed.HasUsingStatements {
			decisions = append(decisions, PermissionDecision{
				Evaluation: platformshell.PermissionEvaluation{
					Decision:          corepermission.DecisionAsk,
					NormalizedCommand: normalized,
					Message:           "Command contains a using statement that may load external code (module or assembly)",
				},
				Reason: "using statement",
			})
		}
	}

	// 5. Provider path detection: commands accessing PSDrive non-filesystem
	// resources (env:, HKLM:, cert:) should always ask.
	if hasProviderPath(command) {
		decisions = append(decisions, PermissionDecision{
			Evaluation: platformshell.PermissionEvaluation{
				Decision:          corepermission.DecisionAsk,
				NormalizedCommand: normalized,
				Message:           "Command accesses non-filesystem provider paths (env:/HKLM:/cert:) and requires approval",
			},
			Reason: "provider path",
		})
	}

	// 6. UNC path detection: commands with UNC paths (\\server\share) can
	// leak NTLM/Kerberos credentials on Windows.
	if containsVulnerableUncPath(command) {
		decisions = append(decisions, PermissionDecision{
			Evaluation: platformshell.PermissionEvaluation{
				Decision:          corepermission.DecisionAsk,
				NormalizedCommand: normalized,
				Message:           "Command contains a UNC path that could trigger network requests",
			},
			Reason: "UNC path",
		})
	}

	// 7. Arg leaks detection: cmdlets that print/display their arguments can
	// leak sensitive values (e.g., Write-Output $env:SECRET).
	if checkArgLeaks(command) {
		decisions = append(decisions, PermissionDecision{
			Evaluation: platformshell.PermissionEvaluation{
				Decision:          corepermission.DecisionAsk,
				NormalizedCommand: normalized,
				Message:           "Command may expose sensitive values in its output",
			},
			Reason: "arg leaks",
		})
	}

	// 8. Git safety: detect writes to .git/ paths and bare-repo compounds
	if checkGitInternalWrite(command) {
		decisions = append(decisions, PermissionDecision{
			Evaluation: platformshell.PermissionEvaluation{
				Decision:          corepermission.DecisionAsk,
				NormalizedCommand: normalized,
				Message:           "Command writes to a git-internal path (.git/) which may compromise git security",
			},
			Reason: "git internal write",
		})
	}
	if checkBareRepoCompound(command) {
		decisions = append(decisions, PermissionDecision{
			Evaluation: platformshell.PermissionEvaluation{
				Decision:          corepermission.DecisionAsk,
				NormalizedCommand: normalized,
				Message:           "Command creates bare-repo paths before running git, which may execute malicious hooks",
			},
			Reason: "bare repo compound",
		})
	}

	// 9. If an explicit allow rule matched, push it
	if baseResult.Decision == corepermission.DecisionAllow {
		decisions = append(decisions, PermissionDecision{
			Evaluation: baseResult,
			Reason:     "rule allow",
		})
	}

	// 10. Read-only cmdlet auto-allow in default mode (with AST element types)
	if isReadOnlyPSCmdletChecked(command, firstCmdElement) {
		// Also check nested commands — if any exist, the statement is not
		// a simple read-only invocation (it contains executables inside
		// script blocks, control flow, etc.)
		if parsed.Valid {
			hasNested := false
			for _, stmt := range parsed.Statements {
				if len(stmt.NestedCommands) > 0 {
					hasNested = true
					break
				}
			}
			if !hasNested {
				decisions = append(decisions, PermissionDecision{
					Evaluation: platformshell.PermissionEvaluation{
						Decision:          corepermission.DecisionAllow,
						NormalizedCommand: normalized,
						Message:           "Command is read-only and safe to execute",
					},
					Reason: "read-only cmdlet + nested check",
				})
			}
		} else {
			decisions = append(decisions, PermissionDecision{
				Evaluation: platformshell.PermissionEvaluation{
					Decision:          corepermission.DecisionAllow,
					NormalizedCommand: normalized,
					Message:           "Command is read-only and safe to execute",
				},
				Reason: "read-only cmdlet",
			})
		}
	}

	// 11. acceptEdits mode: full AST-based check with security guards.
	// Mirrors TS checkPermissionMode (modeValidation.ts).
	if approvalMode == "acceptEdits" && parsed.Valid {
		permDecision := checkAcceptEditsMode(command, parsed)
		if permDecision != nil {
			decisions = append(decisions, *permDecision)
		}
	}

	// 12. dontAsk mode: anything not explicitly allowed or read-only is denied
	if approvalMode == "dontAsk" {
		decisions = append(decisions, PermissionDecision{
			Evaluation: platformshell.PermissionEvaluation{
				Decision:          corepermission.DecisionDeny,
				NormalizedCommand: normalized,
				Message:           fmt.Sprintf("Permission to execute %q was not granted.", normalized),
			},
			Reason: "dontAsk mode: not explicitly allowed",
		})
	}

	// ========================================================================
	// REDUCE PHASE: deny > ask > allow > passthrough
	// ========================================================================
	// First of each behavior type wins (preserves step-order messaging for
	// single-check cases). If nothing decided, fall through to default.
	// Ported from TS powershellPermissions.ts:1354-1368.
	if result := reducePermissionDecisions(decisions); result != nil {
		return *result
	}

	// Default: ask for approval
	return PermissionDecision{
		Evaluation: baseResult,
		Reason:     "default: requires approval",
	}
}

// reducePermissionDecisions applies deny > ask > allow priority to a slice
// of permission decisions. First of each behavior type wins. Returns the
// winning decision or nil when the slice is empty.
// Ported from TS powershellPermissions.ts reduce phase (step 4).
func reducePermissionDecisions(decisions []PermissionDecision) *PermissionDecision {
	for _, d := range decisions {
		if d.Evaluation.Decision == corepermission.DecisionDeny {
			return &d
		}
	}
	for _, d := range decisions {
		if d.Evaluation.Decision == corepermission.DecisionAsk {
			return &d
		}
	}
	for _, d := range decisions {
		if d.Evaluation.Decision == corepermission.DecisionAllow {
			return &d
		}
	}
	return nil
}
