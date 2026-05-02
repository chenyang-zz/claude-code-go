package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// GlobalSettingsStore updates the user-scoped Claude Code settings file while preserving unrelated fields.
type GlobalSettingsStore struct {
	// Path stores the absolute global settings JSON path.
	Path string
}

// NewGlobalSettingsStore builds a user-scoped settings writer from the resolved home directory.
func NewGlobalSettingsStore(homeDir string) *GlobalSettingsStore {
	if strings.TrimSpace(homeDir) == "" {
		return &GlobalSettingsStore{}
	}
	return &GlobalSettingsStore{
		Path: filepath.Join(homeDir, ".claude", "settings.json"),
	}
}

// SaveEditorMode writes the normalized editor mode into the global settings file without dropping unrelated fields.
func (s *GlobalSettingsStore) SaveEditorMode(ctx context.Context, mode string) error {
	_ = ctx

	if s == nil || strings.TrimSpace(s.Path) == "" {
		return fmt.Errorf("global settings path is not configured")
	}

	normalized := coreconfig.NormalizeEditorMode(mode)
	document, err := loadSettingsDocument(s.Path)
	if err != nil {
		return err
	}
	document["editorMode"] = normalized

	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return fmt.Errorf("create global settings directory %s: %w", filepath.Dir(s.Path), err)
	}

	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return fmt.Errorf("encode global settings %s: %w", s.Path, err)
	}
	encoded = append(encoded, '\n')

	if err := os.WriteFile(s.Path, encoded, 0o644); err != nil {
		return fmt.Errorf("write global settings %s: %w", s.Path, err)
	}

	logger.DebugCF("settings_config", "updated global editor mode", map[string]any{
		"path":        s.Path,
		"editor_mode": normalized,
	})
	return nil
}

// SaveModel writes the requested model override into the global settings file without dropping unrelated fields.
func (s *GlobalSettingsStore) SaveModel(ctx context.Context, model string) error {
	_ = ctx

	if s == nil || strings.TrimSpace(s.Path) == "" {
		return fmt.Errorf("global settings path is not configured")
	}

	document, err := loadSettingsDocument(s.Path)
	if err != nil {
		return err
	}

	trimmed := strings.TrimSpace(model)
	if trimmed == "" {
		delete(document, "model")
	} else {
		document["model"] = trimmed
	}

	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return fmt.Errorf("create global settings directory %s: %w", filepath.Dir(s.Path), err)
	}

	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return fmt.Errorf("encode global settings %s: %w", s.Path, err)
	}
	encoded = append(encoded, '\n')

	if err := os.WriteFile(s.Path, encoded, 0o644); err != nil {
		return fmt.Errorf("write global settings %s: %w", s.Path, err)
	}

	logger.DebugCF("settings_config", "updated global model setting", map[string]any{
		"path":         s.Path,
		"model":        trimmed,
		"cleared":      trimmed == "",
		"has_override": trimmed != "",
	})
	return nil
}

// SaveEffortLevel writes the requested effort override into the global settings file without dropping unrelated fields.
func (s *GlobalSettingsStore) SaveEffortLevel(ctx context.Context, effort string) error {
	_ = ctx

	if s == nil || strings.TrimSpace(s.Path) == "" {
		return fmt.Errorf("global settings path is not configured")
	}

	normalized := coreconfig.NormalizeEffortLevel(strings.TrimSpace(effort))
	if normalized != "" && !coreconfig.IsSupportedEffortLevel(normalized) {
		return fmt.Errorf("unsupported effort level %q", effort)
	}

	document, err := loadSettingsDocument(s.Path)
	if err != nil {
		return err
	}
	if normalized == "" {
		delete(document, "effortLevel")
	} else {
		document["effortLevel"] = normalized
	}

	if err := writeSettingsDocument(s.Path, document); err != nil {
		return err
	}

	logger.DebugCF("settings_config", "updated global effort setting", map[string]any{
		"path":         s.Path,
		"effort_level": normalized,
		"cleared":      normalized == "",
	})
	return nil
}

// SaveFastMode writes the requested fast-mode preference into the global settings file without dropping unrelated fields.
func (s *GlobalSettingsStore) SaveFastMode(ctx context.Context, enabled bool) error {
	_ = ctx

	if s == nil || strings.TrimSpace(s.Path) == "" {
		return fmt.Errorf("global settings path is not configured")
	}

	document, err := loadSettingsDocument(s.Path)
	if err != nil {
		return err
	}
	if enabled {
		document["fastMode"] = true
	} else {
		delete(document, "fastMode")
	}

	if err := writeSettingsDocument(s.Path, document); err != nil {
		return err
	}

	logger.DebugCF("settings_config", "updated global fast mode setting", map[string]any{
		"path":      s.Path,
		"fast_mode": enabled,
		"cleared":   !enabled,
	})
	return nil
}

// SaveTheme writes the normalized theme setting into the global settings file without dropping unrelated fields.
func (s *GlobalSettingsStore) SaveTheme(ctx context.Context, theme string) error {
	_ = ctx

	if s == nil || strings.TrimSpace(s.Path) == "" {
		return fmt.Errorf("global settings path is not configured")
	}

	normalized := coreconfig.NormalizeThemeSetting(theme)
	if !coreconfig.IsSupportedThemeSetting(normalized) {
		return fmt.Errorf("unsupported theme setting %q", theme)
	}

	document, err := loadSettingsDocument(s.Path)
	if err != nil {
		return err
	}
	document["theme"] = normalized

	if err := writeSettingsDocument(s.Path, document); err != nil {
		return err
	}

	logger.DebugCF("settings_config", "updated global theme setting", map[string]any{
		"path":  s.Path,
		"theme": normalized,
	})
	return nil
}

// RecordTipShown writes a tip-id → current numStartups entry into the global
// settings file under the tipsHistory key.
func (s *GlobalSettingsStore) RecordTipShown(tipID string) error {
	if s == nil || strings.TrimSpace(s.Path) == "" {
		return fmt.Errorf("global settings path is not configured")
	}

	document, err := loadSettingsDocument(s.Path)
	if err != nil {
		return err
	}

	numStartups := 0
	if v, ok := document["numStartups"].(float64); ok {
		numStartups = int(v)
	}

	history := make(map[string]any)
	if h, ok := document["tipsHistory"].(map[string]any); ok {
		history = h
	}
	history[tipID] = numStartups
	document["tipsHistory"] = history

	if err := writeSettingsDocument(s.Path, document); err != nil {
		return err
	}

	logger.DebugCF("settings_config", "recorded tip shown", map[string]any{
		"path":    s.Path,
		"tip_id":  tipID,
		"session": numStartups,
	})
	return nil
}

// GetTipsHistory reads the tipsHistory map from the global settings file.
// Returns an empty map if the field is absent or malformed.
func (s *GlobalSettingsStore) GetTipsHistory() map[string]int {
	if s == nil || strings.TrimSpace(s.Path) == "" {
		return nil
	}

	document, err := loadSettingsDocument(s.Path)
	if err != nil {
		return nil
	}

	history := make(map[string]int)
	if h, ok := document["tipsHistory"].(map[string]any); ok {
		for k, v := range h {
			if n, ok := v.(float64); ok {
				history[k] = int(n)
			}
		}
	}
	return history
}

// GetNumStartups reads the numStartups counter from the global settings file.
func (s *GlobalSettingsStore) GetNumStartups() int {
	if s == nil || strings.TrimSpace(s.Path) == "" {
		return 0
	}

	document, err := loadSettingsDocument(s.Path)
	if err != nil {
		return 0
	}

	if v, ok := document["numStartups"].(float64); ok {
		return int(v)
	}
	return 0
}

// IncrementNumStartups bumps the numStartups counter in the global settings
// file by one. If the field is absent it is initialised to 1.
func (s *GlobalSettingsStore) IncrementNumStartups() error {
	if s == nil || strings.TrimSpace(s.Path) == "" {
		return fmt.Errorf("global settings path is not configured")
	}

	document, err := loadSettingsDocument(s.Path)
	if err != nil {
		return err
	}

	current := 0
	if v, ok := document["numStartups"].(float64); ok {
		current = int(v)
	}
	current++
	document["numStartups"] = current

	if err := writeSettingsDocument(s.Path, document); err != nil {
		return err
	}

	logger.DebugCF("settings_config", "incremented numStartups", map[string]any{
		"path":        s.Path,
		"num_startups": current,
	})
	return nil
}

// writeSettingsDocument encodes and writes one Claude Code settings document to disk.
func writeSettingsDocument(path string, document map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create settings directory %s: %w", filepath.Dir(path), err)
	}

	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings %s: %w", path, err)
	}
	encoded = append(encoded, '\n')

	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		return fmt.Errorf("write settings %s: %w", path, err)
	}
	return nil
}
