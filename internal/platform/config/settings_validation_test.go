package config

import (
	"strings"
	"testing"
)

// TestValidateSettingsContentAcceptsSupportedSubset verifies the migrated validator accepts the supported settings subset.
func TestValidateSettingsContentAcceptsSupportedSubset(t *testing.T) {
	result := ValidateSettingsContent("{\n  \"$schema\": \"https://json.schemastore.org/claude-code-settings.json\",\n  \"model\": \"sonnet\",\n  \"permissions\": {\n    \"allow\": [\"Read(*)\"],\n    \"deny\": [\"Bash(rm -rf /)\"],\n    \"ask\": [\"Write(*)\"]\n  }\n}\n")
	if !result.IsValid {
		t.Fatalf("ValidateSettingsContent() valid = false, error = %q", result.Error)
	}
	if !strings.Contains(result.FullSchema, "\"permissions\"") {
		t.Fatalf("ValidateSettingsContent() fullSchema = %q, want permissions schema", result.FullSchema)
	}
}

// TestValidateSettingsContentRejectsUnknownFields verifies strict-mode unknown keys are rejected.
func TestValidateSettingsContentRejectsUnknownFields(t *testing.T) {
	result := ValidateSettingsContent("{\n  \"model\": \"sonnet\",\n  \"mystery\": true\n}\n")
	if result.IsValid {
		t.Fatal("ValidateSettingsContent() valid = true, want false")
	}
	if !strings.Contains(result.Error, "- mystery: Unknown field.") {
		t.Fatalf("ValidateSettingsContent() error = %q, want unknown field message", result.Error)
	}
}

// TestValidateSettingsContentRejectsWrongPermissionShape verifies nested permission fields keep precise paths.
func TestValidateSettingsContentRejectsWrongPermissionShape(t *testing.T) {
	result := ValidateSettingsContent("{\n  \"permissions\": {\n    \"allow\": [1]\n  }\n}\n")
	if result.IsValid {
		t.Fatal("ValidateSettingsContent() valid = true, want false")
	}
	if !strings.Contains(result.Error, "- permissions.allow.0: Expected string, but received number") {
		t.Fatalf("ValidateSettingsContent() error = %q, want nested path message", result.Error)
	}
}
