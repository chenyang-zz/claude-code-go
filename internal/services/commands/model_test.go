package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
)

type recordingModelStore struct {
	saved []string
	err   error
}

func (s *recordingModelStore) SaveModel(ctx context.Context, model string) error {
	_ = ctx
	if s.err != nil {
		return s.err
	}
	s.saved = append(s.saved, model)
	return nil
}

// TestModelCommandMetadata verifies /model exposes stable metadata.
func TestModelCommandMetadata(t *testing.T) {
	meta := ModelCommand{}.Metadata()
	if meta.Name != "model" {
		t.Fatalf("Metadata().Name = %q, want model", meta.Name)
	}
	if meta.Description != "Change the model" {
		t.Fatalf("Metadata().Description = %q, want stable model description", meta.Description)
	}
	if meta.Usage != "/model [model]" {
		t.Fatalf("Metadata().Usage = %q, want explicit model usage", meta.Usage)
	}
}

// TestModelCommandExecuteWithoutArgsReportsCurrentModel verifies /model reports the current model and stable fallback guidance.
func TestModelCommandExecuteWithoutArgsReportsCurrentModel(t *testing.T) {
	result, err := ModelCommand{
		Config: &coreconfig.Config{Model: coreconfig.DefaultConfig().Model},
	}.Execute(context.Background(), command.Args{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Current model: claude-sonnet-4-5 (default)\nRun /model <model> to persist a global model override, or /model default to restore the default.\nClaude Code Go does not provide the interactive model picker or model availability checks yet."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
}

// TestModelCommandExecutePersistsModel verifies /model saves an explicit model setting and updates the in-memory config snapshot.
func TestModelCommandExecutePersistsModel(t *testing.T) {
	cfg := &coreconfig.Config{Model: coreconfig.DefaultConfig().Model}
	store := &recordingModelStore{}

	result, err := ModelCommand{Config: cfg, Store: store}.Execute(context.Background(), command.Args{
		Raw:     []string{"claude-opus-4-1"},
		RawLine: "claude-opus-4-1",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Model set to claude-opus-4-1. Claude Code Go stores the preference now, but the interactive model picker and model availability checks are not implemented yet."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
	if len(store.saved) != 1 || store.saved[0] != "claude-opus-4-1" {
		t.Fatalf("saved models = %#v, want []string{\"claude-opus-4-1\"}", store.saved)
	}
	if cfg.Model != "claude-opus-4-1" {
		t.Fatalf("config model = %q, want claude-opus-4-1", cfg.Model)
	}
}

// TestModelCommandExecuteRestoresDefault verifies /model default clears the explicit override and restores the runtime default label.
func TestModelCommandExecuteRestoresDefault(t *testing.T) {
	cfg := &coreconfig.Config{Model: "claude-opus-4-1"}
	store := &recordingModelStore{}

	result, err := ModelCommand{Config: cfg, Store: store}.Execute(context.Background(), command.Args{
		Raw:     []string{"default"},
		RawLine: "default",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Model set to claude-sonnet-4-5 (default). Claude Code Go stores the preference now, but the interactive model picker and model availability checks are not implemented yet."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
	if len(store.saved) != 1 || store.saved[0] != "" {
		t.Fatalf("saved models = %#v, want []string{\"\"}", store.saved)
	}
	if cfg.Model != coreconfig.DefaultConfig().Model {
		t.Fatalf("config model = %q, want %q", cfg.Model, coreconfig.DefaultConfig().Model)
	}
}
