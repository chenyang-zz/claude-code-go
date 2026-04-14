package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

// TestParseEarlyCLIOptionsStripsSettingsFlag verifies bootstrap-time settings flags are removed before REPL parsing.
func TestParseEarlyCLIOptionsStripsSettingsFlag(t *testing.T) {
	options, runArgs, err := ParseEarlyCLIOptions([]string{"--settings", "./settings.json", "--continue", "hello"})
	if err != nil {
		t.Fatalf("ParseEarlyCLIOptions() error = %v", err)
	}

	if options.SettingsValue != "./settings.json" {
		t.Fatalf("ParseEarlyCLIOptions() settings = %q, want ./settings.json", options.SettingsValue)
	}
	if len(runArgs) != 2 || runArgs[0] != "--continue" || runArgs[1] != "hello" {
		t.Fatalf("ParseEarlyCLIOptions() run args = %#v, want [--continue hello]", runArgs)
	}
}

// TestParseEarlyCLIOptionsSupportsEqualsSyntax verifies `--settings=value` follows the same stripping path.
func TestParseEarlyCLIOptionsSupportsEqualsSyntax(t *testing.T) {
	options, runArgs, err := ParseEarlyCLIOptions([]string{"--settings=./settings.json", "/config"})
	if err != nil {
		t.Fatalf("ParseEarlyCLIOptions() error = %v", err)
	}

	if options.SettingsValue != "./settings.json" {
		t.Fatalf("ParseEarlyCLIOptions() settings = %q, want ./settings.json", options.SettingsValue)
	}
	if len(runArgs) != 1 || runArgs[0] != "/config" {
		t.Fatalf("ParseEarlyCLIOptions() run args = %#v, want [/config]", runArgs)
	}
}

// TestParseEarlyCLIOptionsRejectsMissingValue verifies `--settings` without a following value fails early.
func TestParseEarlyCLIOptionsRejectsMissingValue(t *testing.T) {
	_, _, err := ParseEarlyCLIOptions([]string{"--settings"})
	if err == nil {
		t.Fatal("ParseEarlyCLIOptions() error = nil, want missing value error")
	}
	if err.Error() != "missing value for --settings" {
		t.Fatalf("ParseEarlyCLIOptions() error = %q, want missing value for --settings", err.Error())
	}
}

// TestNewAppFromArgsLoadsFlagSettings verifies bootstrap applies `--settings` before building the runtime config.
func TestNewAppFromArgsLoadsFlagSettings(t *testing.T) {
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "project")
	homeDir := filepath.Join(tempDir, "home")

	t.Setenv("HOME", homeDir)
	t.Setenv("PWD", projectDir)

	if err := os.MkdirAll(filepath.Join(projectDir, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll(project) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "flag-settings.json"), []byte(`{"model":"flag-model","provider":"openai-compatible"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(flag settings) error = %v", err)
	}

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("Chdir(project) error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})

	app, runArgs, err := NewAppFromArgs([]string{"--settings", "./flag-settings.json", "hello"})
	if err != nil {
		t.Fatalf("NewAppFromArgs() error = %v", err)
	}

	if app.Config.Model != "flag-model" {
		t.Fatalf("NewAppFromArgs() model = %q, want flag-model", app.Config.Model)
	}
	if app.Config.Provider != "openai-compatible" {
		t.Fatalf("NewAppFromArgs() provider = %q, want openai-compatible", app.Config.Provider)
	}
	if len(runArgs) != 1 || runArgs[0] != "hello" {
		t.Fatalf("NewAppFromArgs() run args = %#v, want [hello]", runArgs)
	}
}
