package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
)

type recordingEffortStore struct {
	saved []string
	err   error
}

func (s *recordingEffortStore) SaveEffortLevel(ctx context.Context, effort string) error {
	_ = ctx
	if s.err != nil {
		return s.err
	}
	s.saved = append(s.saved, effort)
	return nil
}

// TestEffortCommandMetadata verifies /effort exposes stable metadata.
func TestEffortCommandMetadata(t *testing.T) {
	meta := EffortCommand{}.Metadata()
	if meta.Name != "effort" {
		t.Fatalf("Metadata().Name = %q, want effort", meta.Name)
	}
	if meta.Description != "Set effort level for model usage" {
		t.Fatalf("Metadata().Description = %q, want stable effort description", meta.Description)
	}
	if meta.Usage != "/effort [low|medium|high|max|auto]" {
		t.Fatalf("Metadata().Usage = %q, want explicit effort usage", meta.Usage)
	}
}

// TestEffortCommandExecuteWithoutArgsReportsCurrentState verifies /effort reports the current persisted state and fallback guidance.
func TestEffortCommandExecuteWithoutArgsReportsCurrentState(t *testing.T) {
	result, err := EffortCommand{
		Config: &coreconfig.Config{EffortLevel: "high", HasEffortLevelSetting: true},
	}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Current effort level: high\nRun /effort <low|medium|high|max> to persist an effort override, or /effort auto to clear it.\nClaude Code Go does not provide env overrides, session-only effort, model capability checks, or interactive effort controls yet."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}

// TestEffortCommandExecutePersistsLevel verifies /effort stores an explicit effort override and updates the config snapshot.
func TestEffortCommandExecutePersistsLevel(t *testing.T) {
	cfg := &coreconfig.Config{}
	store := &recordingEffortStore{}

	result, err := EffortCommand{Config: cfg, Store: store}.Execute(context.Background(), command.Args{
		Raw:     []string{"medium"},
		RawLine: "medium",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Effort level set to medium. Claude Code Go stores the preference now, but env overrides, session-only effort, model capability checks, and interactive effort controls are not implemented yet."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
	if len(store.saved) != 1 || store.saved[0] != "medium" {
		t.Fatalf("saved effort levels = %#v, want []string{\"medium\"}", store.saved)
	}
	if cfg.EffortLevel != "medium" || !cfg.HasEffortLevelSetting {
		t.Fatalf("config effort = %#v, want explicit medium setting", cfg)
	}
}

// TestEffortCommandExecuteClearsOverride verifies /effort auto removes the explicit effort override.
func TestEffortCommandExecuteClearsOverride(t *testing.T) {
	cfg := &coreconfig.Config{EffortLevel: "high", HasEffortLevelSetting: true}
	store := &recordingEffortStore{}

	result, err := EffortCommand{Config: cfg, Store: store}.Execute(context.Background(), command.Args{
		Raw:     []string{"auto"},
		RawLine: "auto",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Effort level set to auto. Claude Code Go clears the persisted effort override now, but env overrides, session-only effort, model capability checks, and interactive effort controls are not implemented yet."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
	if len(store.saved) != 1 || store.saved[0] != "" {
		t.Fatalf("saved effort levels = %#v, want []string{\"\"}", store.saved)
	}
	if cfg.EffortLevel != "" || cfg.HasEffortLevelSetting {
		t.Fatalf("config effort = %#v, want cleared setting", cfg)
	}
}
