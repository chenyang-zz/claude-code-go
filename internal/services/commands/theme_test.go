package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
)

type recordingThemeStore struct {
	saved []string
	err   error
}

func (s *recordingThemeStore) SaveTheme(ctx context.Context, theme string) error {
	_ = ctx
	if s.err != nil {
		return s.err
	}
	s.saved = append(s.saved, theme)
	return nil
}

// TestThemeCommandMetadata verifies /theme exposes stable metadata.
func TestThemeCommandMetadata(t *testing.T) {
	meta := ThemeCommand{}.Metadata()
	if meta.Name != "theme" {
		t.Fatalf("Metadata().Name = %q, want theme", meta.Name)
	}
	if meta.Description != "Change the theme" {
		t.Fatalf("Metadata().Description = %q, want stable theme description", meta.Description)
	}
	if meta.Usage != "/theme <auto|dark|light|light-daltonized|dark-daltonized|light-ansi|dark-ansi>" {
		t.Fatalf("Metadata().Usage = %q, want explicit theme usage", meta.Usage)
	}
}

// TestThemeCommandExecuteWithoutArgsReportsCurrentTheme verifies /theme reports the current theme and stable fallback guidance.
func TestThemeCommandExecuteWithoutArgsReportsCurrentTheme(t *testing.T) {
	result, err := ThemeCommand{
		Config: &coreconfig.Config{Theme: coreconfig.ThemeSettingLight},
	}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Current theme: light\nAvailable themes: auto, dark, light, light-daltonized, dark-daltonized, light-ansi, dark-ansi\nClaude Code Go does not provide the interactive theme picker yet. Run /theme <theme> to persist a theme setting."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}

// TestThemeCommandExecutePersistsTheme verifies /theme saves an explicit theme setting and updates the in-memory config snapshot.
func TestThemeCommandExecutePersistsTheme(t *testing.T) {
	cfg := &coreconfig.Config{Theme: coreconfig.ThemeSettingDark}
	store := &recordingThemeStore{}

	result, err := ThemeCommand{Config: cfg, Store: store}.Execute(context.Background(), command.Args{
		Raw:     []string{coreconfig.ThemeSettingLightANSI},
		RawLine: coreconfig.ThemeSettingLightANSI,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Theme set to light-ansi. Claude Code Go stores the preference now, but the interactive theme picker and TUI theme rendering are not implemented yet."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
	if len(store.saved) != 1 || store.saved[0] != coreconfig.ThemeSettingLightANSI {
		t.Fatalf("saved themes = %#v, want []string{\"light-ansi\"}", store.saved)
	}
	if cfg.Theme != coreconfig.ThemeSettingLightANSI {
		t.Fatalf("config theme = %q, want light-ansi", cfg.Theme)
	}
}

// TestThemeCommandExecuteRejectsUnsupportedTheme verifies /theme preserves a stable validation error for unsupported values.
func TestThemeCommandExecuteRejectsUnsupportedTheme(t *testing.T) {
	_, err := ThemeCommand{}.Execute(context.Background(), command.Args{
		Raw:     []string{"solarized"},
		RawLine: "solarized",
	})
	if err == nil {
		t.Fatal("Execute() error = nil, want unsupported theme error")
	}
	want := "unsupported theme setting \"solarized\". Expected one of: auto, dark, light, light-daltonized, dark-daltonized, light-ansi, dark-ansi"
	if err.Error() != want {
		t.Fatalf("Execute() error = %q, want %q", err.Error(), want)
	}
}
