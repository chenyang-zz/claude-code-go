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
	document, err := s.loadDocument()
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

	document, err := s.loadDocument()
	if err != nil {
		return err
	}
	document["theme"] = normalized

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

	logger.DebugCF("settings_config", "updated global theme setting", map[string]any{
		"path":  s.Path,
		"theme": normalized,
	})
	return nil
}

// loadDocument reads the current settings JSON document or returns an empty object when it does not exist.
func (s *GlobalSettingsStore) loadDocument() (map[string]any, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("read global settings %s: %w", s.Path, err)
	}

	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return map[string]any{}, nil
	}

	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("parse global settings %s: %w", s.Path, err)
	}
	if document == nil {
		return map[string]any{}, nil
	}
	return document, nil
}
