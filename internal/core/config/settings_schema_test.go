package config

import (
	"strings"
	"testing"
)

// TestSettingsSchemaStringIncludesExpandedFields verifies the JSON schema output includes the fields added in batch-06.
func TestSettingsSchemaStringIncludesExpandedFields(t *testing.T) {
	schema, err := SettingsSchemaString()
	if err != nil {
		t.Fatalf("SettingsSchemaString() error = %v", err)
	}
	for _, needle := range []string{
		`"apiKeyHelper"`,
		`"sessionDbPath"`,
		`"cleanupPeriodDays"`,
		`"defaultShell"`,
		`"theme"`,
		`"defaultMode"`,
		`"additionalDirectories"`,
	} {
		if !strings.Contains(schema, needle) {
			t.Fatalf("SettingsSchemaString() = %q, want substring %q", schema, needle)
		}
	}
}

// TestValidateSettingsDocumentAcceptsExpandedSubset verifies the migrated validator accepts the batch-06 settings subset.
func TestValidateSettingsDocumentAcceptsExpandedSubset(t *testing.T) {
	issues := ValidateSettingsDocument(map[string]any{
		"apiKeyHelper":           "/tmp/auth.sh",
		"sessionDbPath":          "/tmp/session.db",
		"theme":                  "dark",
		"respectGitignore":       true,
		"cleanupPeriodDays":      float64(30),
		"includeCoAuthoredBy":    false,
		"includeGitInstructions": true,
		"defaultShell":           "bash",
		"permissions": map[string]any{
			"allow":                        []any{"Read(*)"},
			"defaultMode":                  "plan",
			"disableBypassPermissionsMode": "disable",
			"additionalDirectories":        []any{"packages/app"},
		},
	})
	if len(issues) != 0 {
		t.Fatalf("ValidateSettingsDocument() issues = %#v, want none", issues)
	}
}

// TestValidateSettingsDocumentRejectsInvalidTheme verifies invalid theme values produce a stable enum error.
func TestValidateSettingsDocumentRejectsInvalidTheme(t *testing.T) {
	issues := ValidateSettingsDocument(map[string]any{
		"theme": "solarized",
	})
	if len(issues) != 1 {
		t.Fatalf("ValidateSettingsDocument() issues = %#v, want 1 issue", issues)
	}
	if issues[0].Path != "theme" {
		t.Fatalf("ValidateSettingsDocument() path = %q, want theme", issues[0].Path)
	}
	if issues[0].Message != `Invalid value. Expected one of: "auto", "dark", "light", "light-daltonized", "dark-daltonized", "light-ansi", "dark-ansi"` {
		t.Fatalf("ValidateSettingsDocument() message = %q, want theme enum error", issues[0].Message)
	}
}

// TestValidateSettingsDocumentRejectsInvalidDefaultShell verifies invalid enum values produce a stable enum error.
func TestValidateSettingsDocumentRejectsInvalidDefaultShell(t *testing.T) {
	issues := ValidateSettingsDocument(map[string]any{
		"defaultShell": "zsh",
	})
	if len(issues) != 1 {
		t.Fatalf("ValidateSettingsDocument() issues = %#v, want 1 issue", issues)
	}
	if issues[0].Path != "defaultShell" {
		t.Fatalf("ValidateSettingsDocument() path = %q, want defaultShell", issues[0].Path)
	}
	if issues[0].Message != `Invalid value. Expected one of: "bash", "powershell"` {
		t.Fatalf("ValidateSettingsDocument() message = %q, want enum error", issues[0].Message)
	}
}

// TestValidateSettingsDocumentRejectsInvalidCleanupPeriod verifies non-negative integer constraints are enforced.
func TestValidateSettingsDocumentRejectsInvalidCleanupPeriod(t *testing.T) {
	testCases := []struct {
		name    string
		value   any
		message string
	}{
		{
			name:    "negative",
			value:   float64(-1),
			message: "Number must be greater than or equal to 0",
		},
		{
			name:    "fractional",
			value:   float64(1.5),
			message: "Expected integer, but received number",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			issues := ValidateSettingsDocument(map[string]any{
				"cleanupPeriodDays": tc.value,
			})
			if len(issues) != 1 {
				t.Fatalf("ValidateSettingsDocument() issues = %#v, want 1 issue", issues)
			}
			if issues[0].Path != "cleanupPeriodDays" {
				t.Fatalf("ValidateSettingsDocument() path = %q, want cleanupPeriodDays", issues[0].Path)
			}
			if issues[0].Message != tc.message {
				t.Fatalf("ValidateSettingsDocument() message = %q, want %q", issues[0].Message, tc.message)
			}
		})
	}
}

// TestValidateSettingsDocumentRejectsPermissionFieldErrors verifies nested permission field errors keep precise paths.
func TestValidateSettingsDocumentRejectsPermissionFieldErrors(t *testing.T) {
	issues := ValidateSettingsDocument(map[string]any{
		"permissions": map[string]any{
			"defaultMode":           "auto",
			"additionalDirectories": []any{"ok", true},
		},
	})
	if len(issues) != 2 {
		t.Fatalf("ValidateSettingsDocument() issues = %#v, want 2 issues", issues)
	}
	if issues[0].Path != "permissions.additionalDirectories.1" && issues[1].Path != "permissions.additionalDirectories.1" {
		t.Fatalf("ValidateSettingsDocument() issues = %#v, want nested additionalDirectories path", issues)
	}
	foundDefaultMode := false
	for _, issue := range issues {
		if issue.Path == "permissions.defaultMode" && strings.Contains(issue.Message, `"acceptEdits"`) {
			foundDefaultMode = true
		}
	}
	if !foundDefaultMode {
		t.Fatalf("ValidateSettingsDocument() issues = %#v, want defaultMode enum error", issues)
	}
}
