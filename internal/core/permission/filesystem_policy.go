package permission

import (
	"context"
	"fmt"
	"os"
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
		return Evaluation{
			Decision: DecisionAsk,
			Message:  fmt.Sprintf("Claude requested permissions to write to %s, but you haven't granted it yet.", req.Path),
		}
	default:
		return Evaluation{
			Decision: DecisionDeny,
			Message:  fmt.Sprintf("permission: unsupported filesystem access %q", req.Access),
		}
	}
}

// evaluateRead applies the minimal batch-01 read policy: allow inside the working directory and ask outside it.
func (p *FilesystemPolicy) evaluateRead(req FilesystemRequest, normalizedPath string, normalizedWorkingDir string) Evaluation {
	_ = p

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
	return Evaluation{
		Decision: DecisionAsk,
		Message:  fmt.Sprintf("Claude requested permissions to read from %s, but you haven't granted it yet.", req.Path),
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
