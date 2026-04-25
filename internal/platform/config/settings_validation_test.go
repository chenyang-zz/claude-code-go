package config

import (
	"strings"
	"testing"
)

// TestValidateSettingsContentAcceptsSupportedSubset verifies the migrated validator accepts the supported settings subset.
func TestValidateSettingsContentAcceptsSupportedSubset(t *testing.T) {
	result := ValidateSettingsContent("{\n  \"$schema\": \"https://json.schemastore.org/claude-code-settings.json\",\n  \"model\": \"sonnet\",\n  \"effortLevel\": \"high\",\n  \"fastMode\": true,\n  \"theme\": \"auto\",\n  \"editorMode\": \"vim\",\n  \"permissions\": {\n    \"allow\": [\"Read(*)\"],\n    \"deny\": [\"Bash(rm -rf /)\"],\n    \"ask\": [\"Write(*)\"]\n  }\n}\n")
	if !result.IsValid {
		t.Fatalf("ValidateSettingsContent() valid = false, error = %q", result.Error)
	}
	if !strings.Contains(result.FullSchema, "\"permissions\"") || !strings.Contains(result.FullSchema, "\"editorMode\"") || !strings.Contains(result.FullSchema, "\"theme\"") || !strings.Contains(result.FullSchema, "\"effortLevel\"") || !strings.Contains(result.FullSchema, "\"fastMode\"") {
		t.Fatalf("ValidateSettingsContent() fullSchema = %q, want permissions, editorMode, theme, effortLevel, and fastMode schema", result.FullSchema)
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

// TestValidateSettingsContentAcceptsExpandedFields verifies the platform validator accepts the batch-06 settings subset.
func TestValidateSettingsContentAcceptsExpandedFields(t *testing.T) {
	result := ValidateSettingsContent("{\n  \"apiKeyHelper\": \"/tmp/auth.sh\",\n  \"sessionDbPath\": \"/tmp/session.db\",\n  \"respectGitignore\": true,\n  \"cleanupPeriodDays\": 7,\n  \"defaultShell\": \"bash\",\n  \"permissions\": {\n    \"defaultMode\": \"plan\",\n    \"disableBypassPermissionsMode\": \"disable\",\n    \"additionalDirectories\": [\"packages/app\"]\n  }\n}\n")
	if !result.IsValid {
		t.Fatalf("ValidateSettingsContent() valid = false, error = %q", result.Error)
	}
	if !strings.Contains(result.FullSchema, "\"defaultShell\"") || !strings.Contains(result.FullSchema, "\"sessionDbPath\"") {
		t.Fatalf("ValidateSettingsContent() fullSchema = %q, want defaultShell and sessionDbPath schema", result.FullSchema)
	}
}

// TestValidateSettingsContentAcceptsHooksPolicyFields verifies the hooks-related settings subset is schema-validated.
func TestValidateSettingsContentAcceptsHooksPolicyFields(t *testing.T) {
	result := ValidateSettingsContent("{\n  \"allowManagedHooksOnly\": true,\n  \"allowedHttpHookUrls\": [\"https://hooks.example.com/*\"],\n  \"httpHookAllowedEnvVars\": [\"MY_TOKEN\"]\n}\n")
	if !result.IsValid {
		t.Fatalf("ValidateSettingsContent() valid = false, error = %q", result.Error)
	}
	if !strings.Contains(result.FullSchema, "\"allowManagedHooksOnly\"") || !strings.Contains(result.FullSchema, "\"allowedHttpHookUrls\"") || !strings.Contains(result.FullSchema, "\"httpHookAllowedEnvVars\"") {
		t.Fatalf("ValidateSettingsContent() fullSchema = %q, want hooks policy fields schema", result.FullSchema)
	}
}

// TestValidateSettingsContentAcceptsEnvField verifies settings env is part of the supported schema subset.
func TestValidateSettingsContentAcceptsEnvField(t *testing.T) {
	result := ValidateSettingsContent("{\n  \"env\": {\n    \"COUNT\": 3,\n    \"ENABLED\": true,\n    \"NAME\": \"claude\"\n  }\n}\n")
	if !result.IsValid {
		t.Fatalf("ValidateSettingsContent() valid = false, error = %q", result.Error)
	}
	if !strings.Contains(result.FullSchema, "\"env\"") {
		t.Fatalf("ValidateSettingsContent() fullSchema = %q, want env schema", result.FullSchema)
	}
}

// TestValidateSettingsContentRejectsExpandedFieldErrors verifies enum and integer constraint errors keep stable caller-facing messages.
func TestValidateSettingsContentRejectsExpandedFieldErrors(t *testing.T) {
	result := ValidateSettingsContent("{\n  \"cleanupPeriodDays\": -1,\n  \"permissions\": {\n    \"defaultMode\": \"auto\"\n  }\n}\n")
	if result.IsValid {
		t.Fatal("ValidateSettingsContent() valid = true, want false")
	}
	if !strings.Contains(result.Error, "- cleanupPeriodDays: Number must be greater than or equal to 0") {
		t.Fatalf("ValidateSettingsContent() error = %q, want cleanupPeriodDays constraint error", result.Error)
	}
	if !strings.Contains(result.Error, "- permissions.defaultMode: Invalid value. Expected one of: \"acceptEdits\", \"bypassPermissions\", \"default\", \"dontAsk\", \"plan\"") {
		t.Fatalf("ValidateSettingsContent() error = %q, want defaultMode enum error", result.Error)
	}
}

// TestValidateSettingsContentRejectsUnsupportedEditorMode verifies editorMode uses a stable enum allowlist.
func TestValidateSettingsContentRejectsUnsupportedEditorMode(t *testing.T) {
	result := ValidateSettingsContent("{\n  \"editorMode\": \"vi\"\n}\n")
	if result.IsValid {
		t.Fatal("ValidateSettingsContent() valid = true, want false")
	}
	if !strings.Contains(result.Error, "- editorMode: Invalid value. Expected one of: \"emacs\", \"normal\", \"vim\"") {
		t.Fatalf("ValidateSettingsContent() error = %q, want editorMode enum error", result.Error)
	}
}

// TestValidateSettingsContentRejectsUnsupportedTheme verifies theme uses a stable enum allowlist.
func TestValidateSettingsContentRejectsUnsupportedTheme(t *testing.T) {
	result := ValidateSettingsContent("{\n  \"theme\": \"solarized\"\n}\n")
	if result.IsValid {
		t.Fatal("ValidateSettingsContent() valid = true, want false")
	}
	if !strings.Contains(result.Error, "- theme: Invalid value. Expected one of: \"auto\", \"dark\", \"light\", \"light-daltonized\", \"dark-daltonized\", \"light-ansi\", \"dark-ansi\"") {
		t.Fatalf("ValidateSettingsContent() error = %q, want theme enum error", result.Error)
	}
}

// TestValidateSettingsContentRejectsUnsupportedEffort verifies effortLevel uses a stable enum allowlist.
func TestValidateSettingsContentRejectsUnsupportedEffort(t *testing.T) {
	result := ValidateSettingsContent("{\n  \"effortLevel\": \"turbo\"\n}\n")
	if result.IsValid {
		t.Fatal("ValidateSettingsContent() valid = true, want false")
	}
	if !strings.Contains(result.Error, "- effortLevel: Invalid value. Expected one of: \"low\", \"medium\", \"high\", \"max\"") {
		t.Fatalf("ValidateSettingsContent() error = %q, want effortLevel enum error", result.Error)
	}
}

func TestValidateSettingsContentRejectsInvalidHooks(t *testing.T) {
	result := ValidateSettingsContent("{\n  \"hooks\": {\n    \"Stop\": \"not-an-array\"\n  }\n}\n")
	if result.IsValid {
		t.Fatal("ValidateSettingsContent() valid = true, want false")
	}
	if !strings.Contains(result.Error, "- hooks: parse hooks for event Stop:") {
		t.Fatalf("ValidateSettingsContent() error = %q, want hook parse failure", result.Error)
	}
}
