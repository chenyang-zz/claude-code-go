package permission

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// FilesystemPolicy provides the minimal filesystem permission checks required by batch-01 tools.
type FilesystemPolicy struct {
	// Rules keeps the normalized read/write rule collections for later migration stages.
	Rules RuleSet
}

// NewFilesystemPolicy constructs the minimal filesystem policy after validating the provided rules.
func NewFilesystemPolicy(rules RuleSet) (*FilesystemPolicy, error) {
	if err := rules.Validate(); err != nil {
		return nil, err
	}

	return &FilesystemPolicy{Rules: rules}, nil
}

// EvaluateFilesystem evaluates one filesystem access request against the current minimal policy.
func (p *FilesystemPolicy) EvaluateFilesystem(ctx context.Context, req FilesystemRequest) Evaluation {
	_ = ctx

	logger.DebugCF("permission", "evaluating filesystem permission", map[string]any{
		"tool_name":   req.ToolName,
		"path":        req.Path,
		"working_dir": req.WorkingDir,
		"access":      string(req.Access),
	})

	if err := req.Validate(); err != nil {
		logger.DebugCF("permission", "filesystem permission request is invalid", map[string]any{
			"error": err.Error(),
		})
		return Evaluation{
			Decision: DecisionDeny,
			Message:  fmt.Sprintf("permission: invalid filesystem request: %v", err),
		}
	}

	normalizedPath, normalizedWorkingDir, err := normalizeFilesystemRequestPath(req.Path, req.WorkingDir)
	if err != nil {
		logger.DebugCF("permission", "filesystem permission path normalization failed", map[string]any{
			"path":        req.Path,
			"working_dir": req.WorkingDir,
			"error":       err.Error(),
		})
		return Evaluation{
			Decision: DecisionDeny,
			Message:  fmt.Sprintf("permission: normalize filesystem path: %v", err),
		}
	}

	logger.DebugCF("permission", "filesystem permission path normalized", map[string]any{
		"normalized_path":        normalizedPath,
		"normalized_working_dir": normalizedWorkingDir,
	})

	switch req.Access {
	case AccessRead:
		return p.evaluateRead(req, normalizedPath, normalizedWorkingDir)
	case AccessWrite:
		return p.evaluateWrite(req, normalizedPath, normalizedWorkingDir)
	default:
		return Evaluation{
			Decision: DecisionDeny,
			Message:  fmt.Sprintf("permission: unsupported filesystem access %q", req.Access),
		}
	}
}

// evaluateRead applies the minimal batch-01 read policy: explicit rules win first, then the working-directory default.
func (p *FilesystemPolicy) evaluateRead(req FilesystemRequest, normalizedPath string, normalizedWorkingDir string) Evaluation {
	if matchedRule := matchingRuleForInput(normalizedPath, normalizedWorkingDir, p.Rules.Read, DecisionDeny); matchedRule != nil {
		logger.DebugCF("permission", "filesystem read denied by rule", map[string]any{
			"path":     normalizedPath,
			"pattern":  matchedRule.Pattern,
			"base_dir": matchedRule.BaseDir,
		})
		return Evaluation{
			Decision: DecisionDeny,
			Rule:     matchedRule,
			Message:  fmt.Sprintf("Permission to read %s has been denied.", req.Path),
		}
	}

	if matchedRule := matchingRuleForInput(normalizedPath, normalizedWorkingDir, p.Rules.Read, DecisionAsk); matchedRule != nil {
		logger.DebugCF("permission", "filesystem read requires approval by rule", map[string]any{
			"path":     normalizedPath,
			"pattern":  matchedRule.Pattern,
			"base_dir": matchedRule.BaseDir,
		})
		return Evaluation{
			Decision: DecisionAsk,
			Rule:     matchedRule,
			Message:  fmt.Sprintf("Claude requested permissions to read from %s, but you haven't granted it yet.", req.Path),
		}
	}

	if pathWithinRoot(normalizedWorkingDir, normalizedPath) {
		logger.DebugCF("permission", "filesystem read allowed inside working directory", map[string]any{
			"path":        normalizedPath,
			"working_dir": normalizedWorkingDir,
		})
		return Evaluation{Decision: DecisionAllow}
	}

	logger.DebugCF("permission", "filesystem read requires approval outside working directory", map[string]any{
		"path":        normalizedPath,
		"working_dir": normalizedWorkingDir,
	})
	if matchedRule := matchingRuleForInput(normalizedPath, normalizedWorkingDir, p.Rules.Read, DecisionAllow); matchedRule != nil {
		logger.DebugCF("permission", "filesystem read allowed by rule", map[string]any{
			"path":     normalizedPath,
			"pattern":  matchedRule.Pattern,
			"base_dir": matchedRule.BaseDir,
		})
		return Evaluation{
			Decision: DecisionAllow,
			Rule:     matchedRule,
		}
	}

	return Evaluation{
		Decision: DecisionAsk,
		Message:  fmt.Sprintf("Claude requested permissions to read from %s, but you haven't granted it yet.", req.Path),
	}
}

// CheckReadPermissionForTool evaluates one read-style filesystem request for a tool.
func (p *FilesystemPolicy) CheckReadPermissionForTool(ctx context.Context, toolName string, path string, workingDir string) Evaluation {
	return p.EvaluateFilesystem(ctx, FilesystemRequest{
		ToolName:   toolName,
		Path:       path,
		WorkingDir: workingDir,
		Access:     AccessRead,
	})
}

// CheckWritePermissionForTool evaluates one write-style filesystem request for a tool.
func (p *FilesystemPolicy) CheckWritePermissionForTool(ctx context.Context, toolName string, path string, workingDir string) Evaluation {
	return p.EvaluateFilesystem(ctx, FilesystemRequest{
		ToolName:   toolName,
		Path:       path,
		WorkingDir: workingDir,
		Access:     AccessWrite,
	})
}

// evaluateWrite applies the minimal batch-01 write policy: explicit rules win, otherwise writes require approval.
func (p *FilesystemPolicy) evaluateWrite(req FilesystemRequest, normalizedPath string, normalizedWorkingDir string) Evaluation {
	if matchedRule := matchingRuleForInput(normalizedPath, normalizedWorkingDir, p.Rules.Write, DecisionDeny); matchedRule != nil {
		logger.DebugCF("permission", "filesystem write denied by rule", map[string]any{
			"path":     normalizedPath,
			"pattern":  matchedRule.Pattern,
			"base_dir": matchedRule.BaseDir,
		})
		return Evaluation{
			Decision: DecisionDeny,
			Rule:     matchedRule,
			Message:  fmt.Sprintf("Permission to write %s has been denied.", req.Path),
		}
	}

	if matchedRule := matchingRuleForInput(normalizedPath, normalizedWorkingDir, p.Rules.Write, DecisionAsk); matchedRule != nil {
		logger.DebugCF("permission", "filesystem write requires approval by rule", map[string]any{
			"path":     normalizedPath,
			"pattern":  matchedRule.Pattern,
			"base_dir": matchedRule.BaseDir,
		})
		return Evaluation{
			Decision: DecisionAsk,
			Rule:     matchedRule,
			Message:  fmt.Sprintf("Claude requested permissions to write to %s, but you haven't granted it yet.", req.Path),
		}
	}

	if matchedRule := matchingRuleForInput(normalizedPath, normalizedWorkingDir, p.Rules.Write, DecisionAllow); matchedRule != nil {
		logger.DebugCF("permission", "filesystem write allowed by rule", map[string]any{
			"path":     normalizedPath,
			"pattern":  matchedRule.Pattern,
			"base_dir": matchedRule.BaseDir,
		})
		return Evaluation{
			Decision: DecisionAllow,
			Rule:     matchedRule,
		}
	}

	logger.DebugCF("permission", "filesystem write requires approval", map[string]any{
		"path":        normalizedPath,
		"working_dir": normalizedWorkingDir,
	})
	return Evaluation{
		Decision: DecisionAsk,
		Message:  fmt.Sprintf("Claude requested permissions to write to %s, but you haven't granted it yet.", req.Path),
	}
}

// normalizeFilesystemRequestPath expands the request path and working directory into comparable absolute paths.
func normalizeFilesystemRequestPath(path string, workingDir string) (string, string, error) {
	normalizedWorkingDir, err := expandPermissionPath(workingDir, "")
	if err != nil {
		return "", "", err
	}

	normalizedPath, err := expandPermissionPath(path, normalizedWorkingDir)
	if err != nil {
		return "", "", err
	}

	return normalizedPath, normalizedWorkingDir, nil
}

// expandPermissionPath applies the minimal path normalization required by the permission layer without depending on platform packages.
func expandPermissionPath(path string, baseDir string) (string, error) {
	actualBaseDir := baseDir
	if actualBaseDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		actualBaseDir = cwd
	}

	if strings.ContainsRune(path, '\x00') || strings.ContainsRune(actualBaseDir, '\x00') {
		return "", fmt.Errorf("path contains null bytes")
	}

	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return filepath.Clean(actualBaseDir), nil
	}

	if trimmedPath == "~" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Clean(homeDir), nil
	}

	if strings.HasPrefix(trimmedPath, "~/") || strings.HasPrefix(trimmedPath, "~\\") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Clean(filepath.Join(homeDir, trimmedPath[2:])), nil
	}

	if filepath.IsAbs(trimmedPath) {
		return filepath.Clean(trimmedPath), nil
	}

	return filepath.Clean(filepath.Join(actualBaseDir, trimmedPath)), nil
}

// pathWithinRoot reports whether the target path is equal to or nested under the provided root.
func pathWithinRoot(root string, target string) bool {
	relativePath, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}

	if relativePath == "." {
		return true
	}

	if relativePath == ".." {
		return false
	}

	return !strings.HasPrefix(relativePath, ".."+string(filepath.Separator))
}

// matchingRuleForInput returns the first rule of the requested decision that matches the normalized path.
func matchingRuleForInput(normalizedPath string, normalizedWorkingDir string, rules []Rule, decision Decision) *Rule {
	for index := range rules {
		rule := &rules[index]
		if rule.Decision != decision {
			continue
		}

		matched, err := matchRule(normalizedPath, normalizedWorkingDir, *rule)
		if err != nil {
			logger.DebugCF("permission", "filesystem rule match failed", map[string]any{
				"path":     normalizedPath,
				"pattern":  rule.Pattern,
				"base_dir": rule.BaseDir,
				"error":    err.Error(),
			})
			continue
		}
		if matched {
			return rule
		}
	}

	return nil
}

// matchRule evaluates the batch-01 subset of matchingRuleForInput semantics using rule-relative glob matching.
func matchRule(normalizedPath string, normalizedWorkingDir string, rule Rule) (bool, error) {
	ruleRoot := normalizedWorkingDir
	if strings.TrimSpace(rule.BaseDir) != "" {
		expandedBaseDir, err := expandPermissionPath(rule.BaseDir, normalizedWorkingDir)
		if err != nil {
			return false, err
		}
		ruleRoot = expandedBaseDir
	}

	if !pathWithinRoot(ruleRoot, normalizedPath) {
		return false, nil
	}

	relativePath, err := filepath.Rel(ruleRoot, normalizedPath)
	if err != nil {
		return false, err
	}

	return matchPermissionPattern(filepath.ToSlash(relativePath), filepath.ToSlash(strings.TrimSpace(rule.Pattern))), nil
}

// matchPermissionPattern matches a normalized relative path against the minimal gitignore-like pattern subset needed by batch-01.
func matchPermissionPattern(relativePath string, patternValue string) bool {
	if patternValue == "" {
		return false
	}

	normalizedRelativePath := strings.TrimPrefix(path.Clean(relativePath), "./")
	normalizedPattern := strings.TrimPrefix(path.Clean(patternValue), "./")

	if normalizedRelativePath == "." {
		normalizedRelativePath = ""
	}
	if normalizedPattern == "." {
		normalizedPattern = ""
	}

	return matchPatternSegments(splitPermissionSegments(normalizedRelativePath), splitPermissionSegments(normalizedPattern))
}

// splitPermissionSegments converts a slash-normalized path into path segments while preserving the empty root case.
func splitPermissionSegments(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, "/")
}

// matchPatternSegments applies '*' and '?' within a segment and '**' across path segments.
func matchPatternSegments(pathSegments []string, patternSegments []string) bool {
	if len(patternSegments) == 0 {
		return len(pathSegments) == 0
	}

	if patternSegments[0] == "**" {
		if matchPatternSegments(pathSegments, patternSegments[1:]) {
			return true
		}
		if len(pathSegments) == 0 {
			return false
		}
		return matchPatternSegments(pathSegments[1:], patternSegments)
	}

	if len(pathSegments) == 0 {
		return false
	}

	matched, err := path.Match(patternSegments[0], pathSegments[0])
	if err != nil || !matched {
		return false
	}

	return matchPatternSegments(pathSegments[1:], patternSegments[1:])
}
