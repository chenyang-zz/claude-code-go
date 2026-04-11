package commands

import (
	"context"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
)

type recordingEditorModeStore struct {
	saved []string
	err   error
}

func (s *recordingEditorModeStore) SaveEditorMode(ctx context.Context, mode string) error {
	_ = ctx
	if s.err != nil {
		return s.err
	}
	s.saved = append(s.saved, mode)
	return nil
}

// TestVimCommandMetadata verifies /vim exposes stable metadata.
func TestVimCommandMetadata(t *testing.T) {
	meta := VimCommand{}.Metadata()
	if meta.Name != "vim" {
		t.Fatalf("Metadata().Name = %q, want vim", meta.Name)
	}
	if meta.Usage != "/vim" {
		t.Fatalf("Metadata().Usage = %q, want /vim", meta.Usage)
	}
}

// TestVimCommandExecuteTogglesNormalToVim verifies /vim flips the default editor mode and persists it.
func TestVimCommandExecuteTogglesNormalToVim(t *testing.T) {
	cfg := &coreconfig.Config{EditorMode: coreconfig.EditorModeNormal}
	store := &recordingEditorModeStore{}

	result, err := VimCommand{Config: cfg, Store: store}.Execute(context.Background(), commandArgsForTest())
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Editor mode set to vim. Claude Code Go stores the setting now, but prompt-editor Vim keybindings are not implemented yet."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
	if len(store.saved) != 1 || store.saved[0] != coreconfig.EditorModeVim {
		t.Fatalf("saved modes = %#v, want []string{\"vim\"}", store.saved)
	}
	if cfg.EditorMode != coreconfig.EditorModeVim {
		t.Fatalf("config editor mode = %q, want vim", cfg.EditorMode)
	}
}

// TestVimCommandExecuteTreatsLegacyEmacsAsNormal verifies legacy emacs config values toggle into vim mode.
func TestVimCommandExecuteTreatsLegacyEmacsAsNormal(t *testing.T) {
	cfg := &coreconfig.Config{EditorMode: coreconfig.EditorModeEmacs}
	store := &recordingEditorModeStore{}

	if _, err := (VimCommand{Config: cfg, Store: store}).Execute(context.Background(), commandArgsForTest()); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(store.saved) != 1 || store.saved[0] != coreconfig.EditorModeVim {
		t.Fatalf("saved modes = %#v, want []string{\"vim\"}", store.saved)
	}
}

// TestVimCommandExecuteTogglesVimToNormal verifies /vim flips vim mode back to normal.
func TestVimCommandExecuteTogglesVimToNormal(t *testing.T) {
	cfg := &coreconfig.Config{EditorMode: coreconfig.EditorModeVim}
	store := &recordingEditorModeStore{}

	result, err := VimCommand{Config: cfg, Store: store}.Execute(context.Background(), commandArgsForTest())
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "Editor mode set to normal. Claude Code Go stores the setting now, but prompt-editor Vim keybindings are not implemented yet."
	if result.Output != want {
		t.Fatalf("Execute() output = %q, want %q", result.Output, want)
	}
	if len(store.saved) != 1 || store.saved[0] != coreconfig.EditorModeNormal {
		t.Fatalf("saved modes = %#v, want []string{\"normal\"}", store.saved)
	}
}

func commandArgsForTest() command.Args {
	return command.Args{}
}
