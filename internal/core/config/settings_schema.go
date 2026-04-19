package config

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/hook"
)

const (
	// SettingsSchemaURL is the schema store URL used by Claude Code settings files.
	SettingsSchemaURL = "https://json.schemastore.org/claude-code-settings.json"
)

var (
	// externalPermissionModes stores the currently migrated external permission-mode enum values.
	externalPermissionModes = []string{
		"acceptEdits",
		"bypassPermissions",
		"default",
		"dontAsk",
		"plan",
	}

	// defaultShellValues stores the currently migrated shell enum values.
	defaultShellValues = []string{
		"bash",
		"powershell",
	}

	// editorModeValues stores the currently migrated prompt editor mode values.
	editorModeValues = []string{
		EditorModeEmacs,
		EditorModeNormal,
		EditorModeVim,
	}

	// themeSettingValues stores the currently migrated terminal theme setting values.
	themeSettingValues = SupportedThemeSettings()

	// effortLevelValues stores the currently migrated persisted effort enum values.
	effortLevelValues = SupportedEffortLevels()
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
			"effortLevel": map[string]any{
				"type":        "string",
				"enum":        effortLevelValues,
				"description": "Persisted effort level for supported models",
			},
			"fastMode": map[string]any{
				"type":        "boolean",
				"description": "Whether fast mode should persist across sessions",
			},
			"theme": map[string]any{
				"type":        "string",
				"enum":        themeSettingValues,
				"description": "Terminal theme preference",
			},
			"editorMode": map[string]any{
				"type":        "string",
				"enum":        editorModeValues,
				"description": "Prompt editor keybinding mode",
			},
			"apiKeyHelper": map[string]any{
				"type":        "string",
				"description": "Path to a script that outputs authentication values",
			},
			"awsCredentialExport": map[string]any{
				"type":        "string",
				"description": "Path to a script that exports AWS credentials",
			},
			"awsAuthRefresh": map[string]any{
				"type":        "string",
				"description": "Path to a script that refreshes AWS authentication",
			},
			"gcpAuthRefresh": map[string]any{
				"type":        "string",
				"description": "Command used to refresh GCP authentication",
			},
			"sessionDbPath": map[string]any{
				"type":        "string",
				"description": "Override the SQLite path used by the Go host session store",
			},
			"respectGitignore": map[string]any{
				"type":        "boolean",
				"description": "Whether file discovery should respect .gitignore files",
			},
			"cleanupPeriodDays": map[string]any{
				"type":        "integer",
				"minimum":     0,
				"description": "Number of days to retain chat transcripts",
			},
			"includeCoAuthoredBy": map[string]any{
				"type":        "boolean",
				"description": "Whether Claude attribution should be included in commits and PRs",
			},
			"includeGitInstructions": map[string]any{
				"type":        "boolean",
				"description": "Whether built-in git workflow instructions should be included",
			},
			"defaultShell": map[string]any{
				"type":        "string",
				"enum":        defaultShellValues,
				"description": "Default shell for input-box ! commands",
			},
			"permissions": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"description":          "Tool usage permissions configuration",
				"properties": map[string]any{
					"allow":                        settingsRuleArraySchema("Allowed permission rules"),
					"deny":                         settingsRuleArraySchema("Denied permission rules"),
					"ask":                          settingsRuleArraySchema("Prompted permission rules"),
					"defaultMode":                  settingsEnumSchema("Default permission mode", externalPermissionModes),
					"disableBypassPermissionsMode": settingsDisableLiteralSchema("Disable the ability to bypass permission prompts"),
					"additionalDirectories":        settingsStringArraySchema("Additional directories to include in permission scope"),
				},
			},
			"hooks": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"description":          "Hook configuration mapped by event name",
				"properties":           map[string]any{},
			},
			"disableAllHooks": map[string]any{
				"type":        "boolean",
				"description": "Disable all hook execution",
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
		case "effortLevel":
			if issue, ok := validateEnumField(key, objectValue[key], effortLevelValues); ok {
				issues = append(issues, issue)
			}
		case "fastMode":
			if issue, ok := validateBooleanField(key, objectValue[key]); ok {
				issues = append(issues, issue)
			}
		case "theme":
			if issue, ok := validateEnumField(key, objectValue[key], themeSettingValues); ok {
				issues = append(issues, issue)
			}
		case "editorMode":
			if issue, ok := validateEnumField(key, objectValue[key], editorModeValues); ok {
				issues = append(issues, issue)
			}
		case "apiKeyHelper", "awsCredentialExport", "awsAuthRefresh", "gcpAuthRefresh", "sessionDbPath":
			if issue, ok := validateStringField(key, objectValue[key]); ok {
				issues = append(issues, issue)
			}
		case "respectGitignore", "includeCoAuthoredBy", "includeGitInstructions":
			if issue, ok := validateBooleanField(key, objectValue[key]); ok {
				issues = append(issues, issue)
			}
		case "cleanupPeriodDays":
			if issue, ok := validateNonNegativeIntegerField(key, objectValue[key]); ok {
				issues = append(issues, issue)
			}
		case "defaultShell":
			if issue, ok := validateEnumField(key, objectValue[key], defaultShellValues); ok {
				issues = append(issues, issue)
			}
		case "permissions":
			issues = append(issues, validatePermissionsField(objectValue[key])...)
		case "hooks":
			issues = append(issues, validateHooksField(key, objectValue[key])...)
		case "disableAllHooks":
			if issue, ok := validateBooleanField(key, objectValue[key]); ok {
				issues = append(issues, issue)
			}
		default:
			issues = append(issues, ValidationIssue{
				Path:    key,
				Message: "Unknown field.",
			})
		}
	}

	return issues
}

func validateHooksField(path string, value any) []ValidationIssue {
	raw, err := json.Marshal(value)
	if err != nil {
		return []ValidationIssue{{
			Path:    path,
			Message: fmt.Sprintf("Invalid hooks configuration: %v", err),
		}}
	}

	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return []ValidationIssue{{
			Path:    path,
			Message: fmt.Sprintf("Expected object, but received invalid hooks value: %v", err),
		}}
	}

	if _, err := hook.ParseHooksConfig(decoded); err != nil {
		return []ValidationIssue{{
			Path:    path,
			Message: err.Error(),
		}}
	}
	return nil
}

// settingsStringArraySchema builds the shared JSON schema for string-array fields.
func settingsStringArraySchema(description string) map[string]any {
	return map[string]any{
		"type":        "array",
		"description": description,
		"items": map[string]any{
			"type": "string",
		},
	}
}

// settingsRuleArraySchema builds the shared JSON schema for allow/deny/ask rule arrays.
func settingsRuleArraySchema(description string) map[string]any {
	return settingsStringArraySchema(description)
}

// settingsEnumSchema builds a string enum schema for the JSON schema document.
func settingsEnumSchema(description string, values []string) map[string]any {
	return map[string]any{
		"type":        "string",
		"enum":        values,
		"description": description,
	}
}

// settingsDisableLiteralSchema builds the schema for settings fields that only accept the literal "disable".
func settingsDisableLiteralSchema(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"const":       "disable",
		"description": description,
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

// validateBooleanField verifies a top-level boolean field.
func validateBooleanField(path string, value any) (ValidationIssue, bool) {
	if _, ok := value.(bool); ok {
		return ValidationIssue{}, false
	}
	return ValidationIssue{
		Path:    path,
		Message: fmt.Sprintf("Expected boolean, but received %s", jsonTypeName(value)),
	}, true
}

// validateEnumField verifies a string enum field against a fixed allowlist.
func validateEnumField(path string, value any, allowed []string) (ValidationIssue, bool) {
	text, ok := value.(string)
	if !ok {
		return ValidationIssue{
			Path:    path,
			Message: fmt.Sprintf("Expected string, but received %s", jsonTypeName(value)),
		}, true
	}
	for _, candidate := range allowed {
		if text == candidate {
			return ValidationIssue{}, false
		}
	}
	return ValidationIssue{
		Path:    path,
		Message: fmt.Sprintf("Invalid value. Expected one of: %s", joinQuotedValues(allowed)),
	}, true
}

// validateDisableLiteralField verifies fields that only allow the literal string "disable".
func validateDisableLiteralField(path string, value any) (ValidationIssue, bool) {
	text, ok := value.(string)
	if !ok {
		return ValidationIssue{
			Path:    path,
			Message: fmt.Sprintf("Expected string, but received %s", jsonTypeName(value)),
		}, true
	}
	if text == "disable" {
		return ValidationIssue{}, false
	}
	return ValidationIssue{
		Path:    path,
		Message: `Invalid value. Expected one of: "disable"`,
	}, true
}

// validateNonNegativeIntegerField verifies a non-negative integer field.
func validateNonNegativeIntegerField(path string, value any) (ValidationIssue, bool) {
	number, ok := value.(float64)
	if !ok {
		return ValidationIssue{
			Path:    path,
			Message: fmt.Sprintf("Expected number, but received %s", jsonTypeName(value)),
		}, true
	}
	if math.Trunc(number) != number {
		return ValidationIssue{
			Path:    path,
			Message: "Expected integer, but received number",
		}, true
	}
	if number < 0 {
		return ValidationIssue{
			Path:    path,
			Message: "Number must be greater than or equal to 0",
		}, true
	}
	return ValidationIssue{}, false
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
		case "defaultMode":
			if issue, ok := validateEnumField("permissions.defaultMode", objectValue[key], externalPermissionModes); ok {
				issues = append(issues, issue)
			}
		case "disableBypassPermissionsMode":
			if issue, ok := validateDisableLiteralField("permissions.disableBypassPermissionsMode", objectValue[key]); ok {
				issues = append(issues, issue)
			}
		case "additionalDirectories":
			issues = append(issues, validateStringArrayField("permissions.additionalDirectories", objectValue[key])...)
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

// joinQuotedValues renders a stable comma-separated enum list using quoted string values.
func joinQuotedValues(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, fmt.Sprintf("%q", value))
	}
	return strings.Join(quoted, ", ")
}
