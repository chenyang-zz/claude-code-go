package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

type settingsFileWriteRequest struct {
	Path      string
	Directory string
	LogKey    string
}

// ProjectSettingsStore updates the repository-scoped Claude Code settings file while preserving unrelated fields.
type ProjectSettingsStore struct {
	// Path stores the absolute project settings JSON path.
	Path string
}

// NewProjectSettingsStore builds a project-scoped settings writer from the resolved workspace directory.
func NewProjectSettingsStore(projectDir string) *ProjectSettingsStore {
	if strings.TrimSpace(projectDir) == "" {
		return &ProjectSettingsStore{}
	}
	return &ProjectSettingsStore{
		Path: filepath.Join(projectDir, ProjectConfigPath),
	}
}

// AddAdditionalDirectory appends one extra working directory into permissions.additionalDirectories without dropping unrelated fields.
func (s *ProjectSettingsStore) AddAdditionalDirectory(ctx context.Context, directory string) error {
	return addAdditionalDirectoryToSettingsFile(ctx, settingsFileWriteRequest{
		Path:      s.Path,
		Directory: directory,
		LogKey:    "updated project additional directories",
	})
}

func addAdditionalDirectoryToSettingsFile(ctx context.Context, request settingsFileWriteRequest) error {
	_ = ctx

	if strings.TrimSpace(request.Path) == "" {
		return fmt.Errorf("settings path is not configured")
	}

	trimmed := strings.TrimSpace(request.Directory)
	if trimmed == "" {
		return fmt.Errorf("additional directory is empty")
	}

	document, err := loadSettingsDocument(request.Path)
	if err != nil {
		return err
	}

	permissions, ok := document["permissions"].(map[string]any)
	if !ok || permissions == nil {
		permissions = map[string]any{}
		document["permissions"] = permissions
	}

	existing := stringSliceFromAny(permissions["additionalDirectories"])
	if !containsString(existing, trimmed) {
		existing = append(existing, trimmed)
	}
	permissions["additionalDirectories"] = existing

	if err := os.MkdirAll(filepath.Dir(request.Path), 0o755); err != nil {
		return fmt.Errorf("create settings directory %s: %w", filepath.Dir(request.Path), err)
	}

	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings %s: %w", request.Path, err)
	}
	encoded = append(encoded, '\n')

	if err := os.WriteFile(request.Path, encoded, 0o644); err != nil {
		return fmt.Errorf("write settings %s: %w", request.Path, err)
	}

	logger.DebugCF("settings_config", request.LogKey, map[string]any{
		"path":      request.Path,
		"directory": trimmed,
		"dir_count": len(existing),
		"persisted": true,
	})
	return nil
}

// loadSettingsDocument reads one Claude Code settings document or returns an empty object when it does not exist.
func loadSettingsDocument(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("read settings %s: %w", path, err)
	}

	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return map[string]any{}, nil
	}

	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("parse settings %s: %w", path, err)
	}
	if document == nil {
		return map[string]any{}, nil
	}
	return document, nil
}

// stringSliceFromAny normalizes one JSON-loaded array into a string slice while discarding incompatible entries.
func stringSliceFromAny(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				continue
			}
			trimmed := strings.TrimSpace(text)
			if trimmed == "" {
				continue
			}
			result = append(result, trimmed)
		}
		return result
	default:
		return nil
	}
}

// containsString reports whether the slice already includes one exact string value.
func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
