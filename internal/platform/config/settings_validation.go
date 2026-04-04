package config

import (
	"encoding/json"
	"fmt"
	"strings"

	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// SettingsValidationResult carries the caller-facing validation outcome for one settings JSON payload.
type SettingsValidationResult struct {
	// IsValid reports whether the payload passed the currently supported schema subset.
	IsValid bool
	// Error stores the stable formatted validation message when IsValid is false.
	Error string
	// FullSchema stores the JSON schema string returned to callers on failure.
	FullSchema string
}

// ValidateSettingsContent validates raw settings JSON content against the current migrated schema subset.
func ValidateSettingsContent(content string) SettingsValidationResult {
	fullSchema, err := coreconfig.SettingsSchemaString()
	if err != nil {
		logger.ErrorCF("settings_config", "failed to render settings schema", map[string]any{
			"error": err.Error(),
		})
		return SettingsValidationResult{
			IsValid:    false,
			Error:      fmt.Sprintf("Failed to render settings schema: %v", err),
			FullSchema: "",
		}
	}

	var decoded any
	if err := json.Unmarshal([]byte(content), &decoded); err != nil {
		logger.DebugCF("settings_config", "settings json parsing failed", map[string]any{
			"error": err.Error(),
		})
		return SettingsValidationResult{
			IsValid:    false,
			Error:      fmt.Sprintf("Invalid JSON: %v", err),
			FullSchema: fullSchema,
		}
	}

	issues := coreconfig.ValidateSettingsDocument(decoded)
	if len(issues) == 0 {
		logger.DebugCF("settings_config", "settings content validated", map[string]any{
			"valid": true,
		})
		return SettingsValidationResult{
			IsValid:    true,
			FullSchema: fullSchema,
		}
	}

	logger.DebugCF("settings_config", "settings content validation failed", map[string]any{
		"issue_count": len(issues),
	})
	return SettingsValidationResult{
		IsValid:    false,
		Error:      formatValidationIssues(issues),
		FullSchema: fullSchema,
	}
}

// formatValidationIssues converts validation issues into the stable multi-line caller-facing message.
func formatValidationIssues(issues []coreconfig.ValidationIssue) string {
	lines := make([]string, 0, len(issues)+1)
	lines = append(lines, "Settings validation failed:")
	for _, issue := range issues {
		lines = append(lines, fmt.Sprintf("- %s: %s", issue.Path, issue.Message))
	}
	return strings.Join(lines, "\n")
}
