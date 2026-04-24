package bash

import (
	"regexp"
	"strings"
)

// RiskLevel describes the severity of a security scan finding.
type RiskLevel string

const (
	// RiskLevelSafe indicates the command passed all security checks.
	RiskLevelSafe RiskLevel = "safe"
	// RiskLevelWarning indicates the command contains patterns that should trigger approval.
	RiskLevelWarning RiskLevel = "warning"
	// RiskLevelDangerous indicates the command contains patterns that should be blocked.
	RiskLevelDangerous RiskLevel = "dangerous"
)

// ScanResult stores the outcome of one security scan.
type ScanResult struct {
	// RiskLevel reports the highest severity found across all validators.
	RiskLevel RiskLevel
	// MatchedPattern names the validator rule that triggered the finding.
	MatchedPattern string
	// Message provides a human-readable explanation of the finding.
	Message string
}

// SecurityScanner checks shell commands for dangerous patterns before execution.
type SecurityScanner interface {
	// Scan evaluates the command and returns the highest-risk result.
	Scan(command string) ScanResult
}

// Validator checks one specific class of dangerous patterns.
type Validator interface {
	// Validate inspects the command and returns the risk level, matched pattern name, and message.
	// When no dangerous pattern is found, Validate returns RiskLevelSafe with empty pattern and message.
	Validate(command string) (risk RiskLevel, pattern string, message string)
}

// DefaultSecurityScanner runs a configurable set of validators against a command.
// It returns the highest-risk result found by any validator.
type DefaultSecurityScanner struct {
	validators []Validator
}

// NewDefaultSecurityScanner builds a scanner with the standard validator set.
func NewDefaultSecurityScanner() *DefaultSecurityScanner {
	return &DefaultSecurityScanner{
		validators: []Validator{
			commandSubstitutionValidator{},
			downloadPipeValidator{},
			privilegeEscalationValidator{},
			forkBombValidator{},
			diskWipeValidator{},
			rootDeletionValidator{},
		},
	}
}

// Scan runs all validators and returns the highest-risk result.
// If no validators are configured or all return Safe, Scan returns RiskLevelSafe.
func (s *DefaultSecurityScanner) Scan(command string) ScanResult {
	if s == nil || len(s.validators) == 0 {
		return ScanResult{RiskLevel: RiskLevelSafe}
	}

	var highest ScanResult
	highest.RiskLevel = RiskLevelSafe

	for _, v := range s.validators {
		risk, pattern, message := v.Validate(command)
		if riskPriority(risk) > riskPriority(highest.RiskLevel) {
			highest = ScanResult{
				RiskLevel:      risk,
				MatchedPattern: pattern,
				Message:        message,
			}
		}
	}

	return highest
}

// riskPriority returns an integer priority for risk level comparison.
// Higher values represent more severe risk.
func riskPriority(r RiskLevel) int {
	switch r {
	case RiskLevelDangerous:
		return 3
	case RiskLevelWarning:
		return 2
	case RiskLevelSafe:
		return 1
	default:
		return 0
	}
}

// ---------------------------------------------------------------------------
// Validator implementations
// ---------------------------------------------------------------------------

// commandSubstitutionValidator detects shell command substitution patterns
// that could be used to execute arbitrary code.
type commandSubstitutionValidator struct{}

var (
	// commandSubstitutionPatterns matches common command substitution syntax.
	// Checks for: $(), <() process substitution, ${} parameter expansion.
	commandSubstitutionPatterns = []*regexp.Regexp{
		regexp.MustCompile(`\$\([^)]`),                    // $(command)
		regexp.MustCompile(`\$\{[^}]*[;|&<>$]`),           // ${...} with dangerous content
		regexp.MustCompile(`[<>]\(`),                       // <() or >() process substitution
	}
)

func (commandSubstitutionValidator) Validate(command string) (RiskLevel, string, string) {
	for _, re := range commandSubstitutionPatterns {
		if re.MatchString(command) {
			return RiskLevelDangerous, "command_substitution",
				"Command contains command substitution patterns ($(), backticks, or process substitution) which can execute arbitrary code"
		}
	}
	if hasUnescapedBacktickPair(command) {
		return RiskLevelDangerous, "command_substitution",
			"Command contains command substitution patterns ($(), backticks, or process substitution) which can execute arbitrary code"
	}
	return RiskLevelSafe, "", ""
}

// hasUnescapedBacktickPair reports whether the command contains an unescaped
// backtick that forms a command-substitution pair. Go regexp does not support
// negative lookbehind, so this is implemented as a simple scan.
func hasUnescapedBacktickPair(command string) bool {
	for i := 0; i < len(command); i++ {
		if command[i] == '`' {
			if i > 0 && command[i-1] == '\\' {
				continue // escaped
			}
			// Look for closing backtick
			for j := i + 1; j < len(command); j++ {
				if command[j] == '`' && command[j-1] != '\\' {
					return true
				}
			}
		}
	}
	return false
}

// downloadPipeValidator detects commands that download and immediately execute
// remote code via shell pipes (e.g., curl | sh).
type downloadPipeValidator struct{}

var (
	// downloadThenExecutePattern matches download tools followed by execution in the same command.
	downloadThenExecutePattern = regexp.MustCompile(`(?i)(?:curl|wget|fetch)\b.*?(?:\|)\s*(?:sh|bash|zsh|fish)`)
)

func (downloadPipeValidator) Validate(command string) (RiskLevel, string, string) {
	// Check for pipe-to-shell pattern
	if downloadThenExecutePattern.MatchString(command) {
		return RiskLevelDangerous, "download_pipe",
			"Command downloads remote code and pipes it directly to a shell, which can execute arbitrary code"
	}

	// Additional check: any download tool piped to shell interpreter
	lower := strings.ToLower(command)
	if strings.Contains(lower, "curl") || strings.Contains(lower, "wget") || strings.Contains(lower, "fetch") {
		if strings.Contains(lower, "| sh") || strings.Contains(lower, "| bash") || strings.Contains(lower, "| zsh") || strings.Contains(lower, "| fish") {
			return RiskLevelDangerous, "download_pipe",
				"Command downloads remote code and pipes it directly to a shell, which can execute arbitrary code"
		}
	}

	return RiskLevelSafe, "", ""
}

// privilegeEscalationValidator detects commands that attempt to elevate privileges.
type privilegeEscalationValidator struct{}

var (
	// privilegeEscalationPattern matches sudo, su, doas, pkexec and common privilege escalation.
	privilegeEscalationPattern = regexp.MustCompile(`(?:^|[\s;|&])(?:sudo|su\b|doas|pkexec|sudoedit)(?:\s|$)`)
)

func (privilegeEscalationValidator) Validate(command string) (RiskLevel, string, string) {
	if privilegeEscalationPattern.MatchString(command) {
		return RiskLevelWarning, "privilege_escalation",
			"Command attempts to elevate privileges (sudo, su, doas, pkexec). This requires approval."
	}
	return RiskLevelSafe, "", ""
}

// forkBombValidator detects classic fork bomb patterns.
type forkBombValidator struct{}

var (
	// forkBombPattern matches classic bash fork bomb patterns.
	// Matches: :(){ :|:& };:  or  bomb(){ bomb|bomb& }; bomb
	forkBombPattern = regexp.MustCompile(`(?::|\w+)\s*\(\s*\)\s*\{\s*(?::|\w+)\s*\|`)
)

func (forkBombValidator) Validate(command string) (RiskLevel, string, string) {
	if forkBombPattern.MatchString(command) {
		return RiskLevelDangerous, "fork_bomb",
			"Command contains a fork bomb pattern which would exhaust system resources"
	}
	return RiskLevelSafe, "", ""
}

// diskWipeValidator detects commands that could erase or corrupt disk data.
type diskWipeValidator struct{}

var (
	// diskWipePattern matches dd with output to a block device.
	// Only flags the command dangerous when of= points to a block device.
	diskWipePattern = regexp.MustCompile(`(?i)\bdd\b.*?\bof=/dev/(?:sda|sdb|nvme|hd|mmcblk|vd|loop|disk)`)
	// formatPattern matches direct disk formatting commands targeting block devices.
	formatPattern = regexp.MustCompile(`(?i)(?:^|[\s;|&])(?:mkfs|mkfs\.(?:ext[2-4]|xfs|btrfs|ntfs|vfat)|fdisk|parted)\b.*?(?:/dev/\w+)`)
)

func (diskWipeValidator) Validate(command string) (RiskLevel, string, string) {
	if diskWipePattern.MatchString(command) {
		return RiskLevelDangerous, "disk_wipe",
			"Command appears to write to a block device or use disk formatting tools, which could destroy data"
	}
	if formatPattern.MatchString(command) {
		return RiskLevelDangerous, "disk_format",
			"Command uses disk formatting tools (mkfs, fdisk, parted) which could destroy data"
	}
	return RiskLevelSafe, "", ""
}

// rootDeletionValidator detects commands that recursively delete the root directory.
type rootDeletionValidator struct{}

var (
	// rootDeletionPattern matches rm -rf /, rm -rf /*, and similar root deletion patterns.
	// Ensures the / is followed by end-of-string, separator, *, .*, or --no-preserve-root,
	// not by normal path characters like /tmp.
	rootDeletionPattern = regexp.MustCompile(`(?:^|[\s;|&])rm\s+(?:-[a-zA-Z]*f[a-zA-Z]*\s+)?(?:/\s*(?:$|[\s;|&])|/\*|/\.\*|/\s+--no-preserve-root)`)
)

func (rootDeletionValidator) Validate(command string) (RiskLevel, string, string) {
	if rootDeletionPattern.MatchString(command) {
		return RiskLevelDangerous, "root_deletion",
			"Command recursively deletes the root directory or its contents, which would destroy the system"
	}
	return RiskLevelSafe, "", ""
}
