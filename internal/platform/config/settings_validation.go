package config

import (
	"encoding/json"
	"fmt"
	"strings"

	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// ValidationCodeInvalidType mirrors Zod's invalid_type issue category.
	ValidationCodeInvalidType = "invalid_type"
	// ValidationCodeInvalidValue mirrors Zod's invalid_value issue category.
	ValidationCodeInvalidValue = "invalid_value"
	// ValidationCodeUnrecognizedKeys mirrors Zod's unrecognized_keys issue category.
	ValidationCodeUnrecognizedKeys = "unrecognized_keys"
	// ValidationCodeTooSmall mirrors Zod's too_small issue category.
	ValidationCodeTooSmall = "too_small"
	// ValidationCodeCustom is the fallback for custom/other validation failures.
	ValidationCodeCustom = "custom"
)

// SettingsValidationIssue describes one normalized validation issue used by settings-edit callers.
type SettingsValidationIssue struct {
	// Path stores the dotted field path (for example permissions.defaultMode).
	Path string
	// Code stores the normalized issue code, aligned to common Zod categories.
	Code string
	// Message stores the final caller-facing issue text.
	Message string
	// Expected stores the expected type/value summary when available.
	Expected string
	// InvalidValue stores the received type/value summary when available.
	InvalidValue string
}

// SettingsValidationResult carries the caller-facing validation outcome for one settings JSON payload.
type SettingsValidationResult struct {
	// IsValid reports whether the payload passed the currently supported schema subset.
	IsValid bool
	// Error stores the stable formatted validation message when IsValid is false.
	Error string
	// FullSchema stores the JSON schema string returned to callers on failure.
	FullSchema string
	// Issues stores normalized issue entries for structured error handling.
	Issues []SettingsValidationIssue
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
			Issues: []SettingsValidationIssue{{
				Path:    "",
				Code:    ValidationCodeCustom,
				Message: fmt.Sprintf("Failed to render settings schema: %v", err),
			}},
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
			Issues: []SettingsValidationIssue{{
				Path:    "",
				Code:    ValidationCodeCustom,
				Message: fmt.Sprintf("Invalid JSON: %v", err),
			}},
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
			Issues:     nil,
		}
	}

	normalizedIssues := normalizeValidationIssues(issues)
	logger.DebugCF("settings_config", "settings content validation failed", map[string]any{
		"issue_count": len(issues),
	})
	return SettingsValidationResult{
		IsValid:    false,
		Error:      formatValidationIssues(normalizedIssues),
		FullSchema: fullSchema,
		Issues:     normalizedIssues,
	}
}

// formatValidationIssues converts validation issues into the stable multi-line caller-facing message.
func formatValidationIssues(issues []SettingsValidationIssue) string {
	lines := make([]string, 0, len(issues)+1)
	lines = append(lines, "Settings validation failed:")
	for _, issue := range issues {
		lines = append(lines, fmt.Sprintf("- %s: %s", issue.Path, issue.Message))
	}
	return strings.Join(lines, "\n")
}

// normalizeValidationIssues maps core config validation issues into a structured issue model.
func normalizeValidationIssues(issues []coreconfig.ValidationIssue) []SettingsValidationIssue {
	normalized := make([]SettingsValidationIssue, 0, len(issues))
	for _, issue := range issues {
		entry := SettingsValidationIssue{
			Path:    issue.Path,
			Code:    ValidationCodeCustom,
			Message: issue.Message,
		}
		message := issue.Message

		switch {
		case strings.HasPrefix(message, "Expected ") && strings.Contains(message, ", but received "):
			entry.Code = ValidationCodeInvalidType
			expected, received := splitExpectedReceived(message)
			entry.Expected = expected
			entry.InvalidValue = received
		case strings.HasPrefix(message, "Invalid value. Expected one of: "):
			entry.Code = ValidationCodeInvalidValue
			entry.Expected = strings.TrimPrefix(message, "Invalid value. Expected one of: ")
		case strings.HasPrefix(message, "Number must be greater than or equal to "):
			entry.Code = ValidationCodeTooSmall
			entry.Expected = strings.TrimPrefix(message, "Number must be greater than or equal to ")
		case message == "Unknown field.":
			entry.Code = ValidationCodeUnrecognizedKeys
		}

		normalized = append(normalized, entry)
	}
	return normalized
}

// splitExpectedReceived parses messages like `Expected string, but received number`.
func splitExpectedReceived(message string) (string, string) {
	const prefix = "Expected "
	const middle = ", but received "
	if !strings.HasPrefix(message, prefix) || !strings.Contains(message, middle) {
		return "", ""
	}
	body := strings.TrimPrefix(message, prefix)
	parts := strings.SplitN(body, middle, 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}
