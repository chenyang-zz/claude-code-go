package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
)

type recordingFastModeStore struct {
	saved []bool
	err   error
}

func (s *recordingFastModeStore) SaveFastMode(ctx context.Context, enabled bool) error {
	_ = ctx
	if s.err != nil {
		return s.err
	}
	s.saved = append(s.saved, enabled)
	return nil
}

// TestFastCommandMetadata verifies /fast exposes stable metadata.
func TestFastCommandMetadata(t *testing.T) {
	meta := FastCommand{}.Metadata()
	if meta.Name != "fast" {
		t.Fatalf("Metadata().Name = %q, want fast", meta.Name)
	}
	if meta.Description != "Toggle fast mode (Opus 4.6 only)" {
		t.Fatalf("Metadata().Description = %q, want stable fast description", meta.Description)
	}
	if meta.Usage != "/fast [on|off]" {
		t.Fatalf("Metadata().Usage = %q, want explicit fast usage", meta.Usage)
	}
}

// TestFastCommandExecuteWithoutArgsReportsCurrentState verifies /fast reports the current persisted state and fallback guidance.
func TestFastCommandExecuteWithoutArgsReportsCurrentState(t *testing.T) {
	result, err := FastCommand{
		Config: &coreconfig.Config{FastMode: true, HasFastModeSetting: true},
	}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Fast mode is currently ON.\nRun /fast on to persist fast mode, or /fast off to clear it.\nClaude Code Go does not provide the interactive fast-mode picker, subscription gating, model auto-switching, or cooldown and quota handling yet."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}

// TestFastCommandExecuteEnablesFastMode verifies /fast on persists the setting and updates the in-memory config snapshot.
func TestFastCommandExecuteEnablesFastMode(t *testing.T) {
	cfg := &coreconfig.Config{}
	store := &recordingFastModeStore{}

	result, err := FastCommand{Config: cfg, Store: store}.Execute(context.Background(), command.Args{
		Raw:     []string{"on"},
		RawLine: "on",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Fast mode enabled. Claude Code Go stores the preference now, but the interactive fast-mode picker, subscription gating, model auto-switching, and cooldown/quota handling are not implemented yet."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
	if len(store.saved) != 1 || !store.saved[0] {
		t.Fatalf("saved fast mode settings = %#v, want []bool{true}", store.saved)
	}
	if !cfg.FastMode || !cfg.HasFastModeSetting {
		t.Fatalf("config fast mode = %#v, want enabled explicit setting", cfg)
	}
}

// TestFastCommandExecuteDisablesFastMode verifies /fast off clears the persisted preference and updates the in-memory config snapshot.
func TestFastCommandExecuteDisablesFastMode(t *testing.T) {
	cfg := &coreconfig.Config{FastMode: true, HasFastModeSetting: true}
	store := &recordingFastModeStore{}

	result, err := FastCommand{Config: cfg, Store: store}.Execute(context.Background(), command.Args{
		Raw:     []string{"off"},
		RawLine: "off",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Fast mode disabled. Claude Code Go clears the persisted preference now, but the interactive fast-mode picker, subscription gating, model auto-switching, and cooldown/quota handling are not implemented yet."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
	if len(store.saved) != 1 || store.saved[0] {
		t.Fatalf("saved fast mode settings = %#v, want []bool{false}", store.saved)
	}
	if cfg.FastMode || cfg.HasFastModeSetting {
		t.Fatalf("config fast mode = %#v, want cleared setting", cfg)
	}
}
