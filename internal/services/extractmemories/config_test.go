package extractmemories

import (
	"os"
	"testing"
)

func TestIsExtractMemoriesEnabled(t *testing.T) {
	envKey := "CLAUDE_FEATURE_" + FlagExtractMemories

	t.Run("default true when env unset", func(t *testing.T) {
		os.Unsetenv(envKey)
		if !IsExtractMemoriesEnabled() {
			t.Error("expected enabled by default")
		}
	})

	t.Run("enabled with 1", func(t *testing.T) {
		os.Setenv(envKey, "1")
		defer os.Unsetenv(envKey)
		if !IsExtractMemoriesEnabled() {
			t.Error("expected enabled")
		}
	})

	t.Run("disabled with 0", func(t *testing.T) {
		os.Setenv(envKey, "0")
		defer os.Unsetenv(envKey)
		if IsExtractMemoriesEnabled() {
			t.Error("expected disabled")
		}
	})

	t.Run("disabled with false", func(t *testing.T) {
		os.Setenv(envKey, "false")
		defer os.Unsetenv(envKey)
		if IsExtractMemoriesEnabled() {
			t.Error("expected disabled")
		}
	})
}

func TestDefaultExtractionConfig(t *testing.T) {
	cfg := DefaultExtractionConfig()
	if cfg.ExtractIntervalTurns != DefaultExtractIntervalTurns {
		t.Errorf("ExtractIntervalTurns: got %d, want %d", cfg.ExtractIntervalTurns, DefaultExtractIntervalTurns)
	}
	if cfg.SkipIndex != DefaultSkipIndex {
		t.Errorf("SkipIndex: got %v, want %v", cfg.SkipIndex, DefaultSkipIndex)
	}
	if cfg.MaxTurns != DefaultMaxTurns {
		t.Errorf("MaxTurns: got %d, want %d", cfg.MaxTurns, DefaultMaxTurns)
	}
}

func TestStateCRUD(t *testing.T) {
	s := NewState()

	t.Run("initial cursor is empty", func(t *testing.T) {
		if s.GetLastMemoryMessageUuid() != "" {
			t.Error("expected empty cursor")
		}
	})

	t.Run("set and get cursor", func(t *testing.T) {
		s.SetLastMemoryMessageUuid("msg-uuid-123")
		if s.GetLastMemoryMessageUuid() != "msg-uuid-123" {
			t.Error("expected cursor to be set")
		}
	})

	t.Run("in progress flag", func(t *testing.T) {
		if s.IsInProgress() {
			t.Error("expected not in progress initially")
		}
		s.SetInProgress(true)
		if !s.IsInProgress() {
			t.Error("expected in progress after setting")
		}
		s.SetInProgress(false)
		if s.IsInProgress() {
			t.Error("expected not in progress after reset")
		}
	})

	t.Run("turns counter", func(t *testing.T) {
		s.ResetTurnsSinceLastExtraction()
		if s.TurnsSinceLastExtraction() != 0 {
			t.Error("expected 0 after reset")
		}
		s.IncrementTurnsSinceLastExtraction()
		s.IncrementTurnsSinceLastExtraction()
		if s.TurnsSinceLastExtraction() != 2 {
			t.Errorf("expected 2, got %d", s.TurnsSinceLastExtraction())
		}
	})

	t.Run("gate failure logged flag", func(t *testing.T) {
		if s.HasLoggedGateFailure() {
			t.Error("expected not logged initially")
		}
		s.SetHasLoggedGateFailure(true)
		if !s.HasLoggedGateFailure() {
			t.Error("expected logged after setting")
		}
	})

	t.Run("set and get config", func(t *testing.T) {
		customCfg := ExtractionConfig{
			ExtractIntervalTurns: 3,
			SkipIndex:            true,
			MaxTurns:             10,
		}
		s.SetConfig(customCfg)
		got := s.Config()
		if got.ExtractIntervalTurns != 3 || !got.SkipIndex || got.MaxTurns != 10 {
			t.Errorf("config mismatch: %+v", got)
		}
	})

	t.Run("reset state", func(t *testing.T) {
		s.Reset()
		if s.GetLastMemoryMessageUuid() != "" {
			t.Error("expected empty cursor after reset")
		}
		if s.IsInProgress() {
			t.Error("expected not in progress after reset")
		}
		if s.TurnsSinceLastExtraction() != 0 {
			t.Error("expected 0 turns after reset")
		}
	})
}

func TestFeatureFlagKeyConstants(t *testing.T) {
	if FlagExtractMemories != "EXTRACT_MEMORIES" {
		t.Errorf("FlagExtractMemories: got %q, want %q", FlagExtractMemories, "EXTRACT_MEMORIES")
	}
	if GBTenguPassportQuail != "tengu_passport_quail" {
		t.Errorf("GBTenguPassportQuail mismatch")
	}
	if GBTenguBrambleLintel != "tengu_bramble_lintel" {
		t.Errorf("GBTenguBrambleLintel mismatch")
	}
	if GBTenguMothCopse != "tengu_moth_copse" {
		t.Errorf("GBTenguMothCopse mismatch")
	}
	if GBTenguSlateThimble != "tengu_slate_thimble" {
		t.Errorf("GBTenguSlateThimble mismatch")
	}
}
