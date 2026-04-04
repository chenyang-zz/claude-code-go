package config

import (
	"encoding/json"
	"fmt"
	"sort"
)

const (
	// SettingsSchemaURL is the schema store URL used by Claude Code settings files.
	SettingsSchemaURL = "https://json.schemastore.org/claude-code-settings.json"
)

// ValidationIssue describes one caller-facing settings validation failure.
type ValidationIssue struct {
	// Path identifies the offending settings path using dotted notation.
	Path string
	// Message stores the stable validation message for the path.
	Message string
}

// SettingsSchemaDocument returns the minimal JSON schema document supported in the current migration pass.
func SettingsSchemaDocument() map[string]any {
	return map[string]any{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"title":                "Claude Code Settings",
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"$schema": map[string]any{
				"type":        "string",
				"const":       SettingsSchemaURL,
				"description": "JSON Schema reference for Claude Code settings",
			},
			"model": map[string]any{
				"type":        "string",
				"description": "Override the default model used by Claude Code",
			},
			"permissions": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"description":          "Tool usage permissions configuration",
				"properties": map[string]any{
					"allow": settingsRuleArraySchema("Allowed permission rules"),
					"deny":  settingsRuleArraySchema("Denied permission rules"),
					"ask":   settingsRuleArraySchema("Prompted permission rules"),
				},
			},
		},
	}
}

// SettingsSchemaString returns the indented JSON schema string used in tool-facing errors.
func SettingsSchemaString() (string, error) {
	encoded, err := json.MarshalIndent(SettingsSchemaDocument(), "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal settings schema: %w", err)
	}
	return string(encoded), nil
}

// ValidateSettingsDocument validates the minimal migrated settings subset against the current schema model.
func ValidateSettingsDocument(value any) []ValidationIssue {
	objectValue, ok := value.(map[string]any)
	if !ok {
		return []ValidationIssue{{
			Path:    "",
			Message: fmt.Sprintf("Expected object, but received %s", jsonTypeName(value)),
		}}
	}

	issues := make([]ValidationIssue, 0)
	keys := make([]string, 0, len(objectValue))
	for key := range objectValue {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		switch key {
		case "$schema":
			if issue, ok := validateSchemaURL(objectValue[key]); ok {
				issues = append(issues, issue)
			}
		case "model":
			if issue, ok := validateStringField("model", objectValue[key]); ok {
				issues = append(issues, issue)
			}
		case "permissions":
			issues = append(issues, validatePermissionsField(objectValue[key])...)
		default:
			issues = append(issues, ValidationIssue{
				Path:    key,
				Message: "Unknown field.",
			})
		}
	}

	return issues
}

// settingsRuleArraySchema builds the shared JSON schema for allow/deny/ask rule arrays.
func settingsRuleArraySchema(description string) map[string]any {
	return map[string]any{
		"type":        "array",
		"description": description,
		"items": map[string]any{
			"type": "string",
		},
	}
}

// validateSchemaURL verifies the optional $schema field matches the supported schema URL.
func validateSchemaURL(value any) (ValidationIssue, bool) {
	text, ok := value.(string)
	if !ok {
		return ValidationIssue{
			Path:    "$schema",
			Message: fmt.Sprintf("Expected string, but received %s", jsonTypeName(value)),
		}, true
	}
	if text != SettingsSchemaURL {
		return ValidationIssue{
			Path:    "$schema",
			Message: fmt.Sprintf("Expected %q, but received %q", SettingsSchemaURL, text),
		}, true
	}
	return ValidationIssue{}, false
}

// validateStringField verifies a top-level string field.
func validateStringField(path string, value any) (ValidationIssue, bool) {
	if _, ok := value.(string); ok {
		return ValidationIssue{}, false
	}
	return ValidationIssue{
		Path:    path,
		Message: fmt.Sprintf("Expected string, but received %s", jsonTypeName(value)),
	}, true
}

// validatePermissionsField verifies the minimal permissions object supported by the current migration pass.
func validatePermissionsField(value any) []ValidationIssue {
	objectValue, ok := value.(map[string]any)
	if !ok {
		return []ValidationIssue{{
			Path:    "permissions",
			Message: fmt.Sprintf("Expected object, but received %s", jsonTypeName(value)),
		}}
	}

	issues := make([]ValidationIssue, 0)
	keys := make([]string, 0, len(objectValue))
	for key := range objectValue {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		switch key {
		case "allow", "deny", "ask":
			issues = append(issues, validateStringArrayField("permissions."+key, objectValue[key])...)
		default:
			issues = append(issues, ValidationIssue{
				Path:    "permissions." + key,
				Message: "Unknown field.",
			})
		}
	}

	return issues
}

// validateStringArrayField verifies an array of strings and reports the first invalid element path precisely.
func validateStringArrayField(path string, value any) []ValidationIssue {
	items, ok := value.([]any)
	if !ok {
		return []ValidationIssue{{
			Path:    path,
			Message: fmt.Sprintf("Expected array, but received %s", jsonTypeName(value)),
		}}
	}

	issues := make([]ValidationIssue, 0)
	for index, item := range items {
		if _, ok := item.(string); ok {
			continue
		}
		issues = append(issues, ValidationIssue{
			Path:    fmt.Sprintf("%s.%d", path, index),
			Message: fmt.Sprintf("Expected string, but received %s", jsonTypeName(item)),
		})
	}
	return issues
}

// jsonTypeName renders JSON-compatible type names for validation messages.
func jsonTypeName(value any) string {
	switch value.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case bool:
		return "boolean"
	case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return fmt.Sprintf("%T", value)
	}
}
