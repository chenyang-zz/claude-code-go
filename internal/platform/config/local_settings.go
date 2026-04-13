package config

import (
	"context"
	"path/filepath"
	"strings"
)

// LocalSettingsStore updates the gitignored project-local Claude Code settings file while preserving unrelated fields.
type LocalSettingsStore struct {
	// Path stores the absolute local settings JSON path.
	Path string
}

// NewLocalSettingsStore builds a project-local settings writer from the resolved workspace directory.
func NewLocalSettingsStore(projectDir string) *LocalSettingsStore {
	if strings.TrimSpace(projectDir) == "" {
		return &LocalSettingsStore{}
	}
	return &LocalSettingsStore{
		Path: filepath.Join(projectDir, LocalConfigPath),
	}
}

// AddAdditionalDirectory appends one extra working directory into permissions.additionalDirectories for local settings.
func (s *LocalSettingsStore) AddAdditionalDirectory(ctx context.Context, directory string) error {
	return addAdditionalDirectoryToSettingsFile(ctx, settingsFileWriteRequest{
		Path:      s.Path,
		Directory: directory,
		LogKey:    "updated local additional directories",
	})
}
