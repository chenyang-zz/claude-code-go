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

// SettingsWriter provides generalized read/write access to Claude Code settings files
// across user, project, and local scopes. It replaces the per-field save methods on
// the individual stores with a single key-path-based API.
type SettingsWriter struct {
	userPath    string
	projectPath string
	localPath   string
}

// NewSettingsWriter creates a SettingsWriter with resolved paths for all three scopes.
// Empty homeDir or projectDir disables the corresponding scopes.
func NewSettingsWriter(homeDir, projectDir string) *SettingsWriter {
	w := &SettingsWriter{}
	if strings.TrimSpace(homeDir) != "" {
		w.userPath = filepath.Join(homeDir, ".claude", "settings.json")
	}
	if strings.TrimSpace(projectDir) != "" {
		w.projectPath = filepath.Join(projectDir, ProjectConfigPath)
		w.localPath = filepath.Join(projectDir, LocalConfigPath)
	}
	return w
}

// resolvePath maps a scope identifier to the corresponding settings file path.
func (w *SettingsWriter) resolvePath(scope string) (string, error) {
	switch scope {
	case "user":
		if w.userPath == "" {
			return "", fmt.Errorf("user scope is not available: home directory not configured")
		}
		return w.userPath, nil
	case "project":
		if w.projectPath == "" {
			return "", fmt.Errorf("project scope is not available: project directory not configured")
		}
		return w.projectPath, nil
	case "local":
		if w.localPath == "" {
			return "", fmt.Errorf("local scope is not available: project directory not configured")
		}
		return w.localPath, nil
	default:
		return "", fmt.Errorf("unknown scope %q: must be user, project, or local", scope)
	}
}

// Get reads the current value of a settings key from the specified scope.
// The key may be a simple field name ("model") or a dotted path ("permissions.defaultMode").
func (w *SettingsWriter) Get(ctx context.Context, scope, key string) (any, error) {
	_ = ctx

	path, err := w.resolvePath(scope)
	if err != nil {
		return nil, err
	}

	doc, err := loadSettingsDocument(path)
	if err != nil {
		return nil, err
	}

	keyPath := strings.Split(key, ".")
	val, _ := getNestedValue(doc, keyPath)
	return val, nil
}

// Set writes a value to a settings key in the specified scope.
// The key may be a simple field name or a dotted path. The modified document is validated
// against the settings JSON Schema before writing.
func (w *SettingsWriter) Set(ctx context.Context, scope, key string, value any) error {
	_ = ctx

	path, err := w.resolvePath(scope)
	if err != nil {
		return err
	}

	doc, err := loadSettingsDocument(path)
	if err != nil {
		return err
	}

	keyPath := strings.Split(key, ".")
	setNestedKey(doc, keyPath, value)

	// Validate the modified document against the settings schema
	encoded, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("encode settings for validation: %w", err)
	}
	result := ValidateSettingsContent(string(encoded))
	if !result.IsValid {
		return fmt.Errorf("settings validation failed: %s", result.Error)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create settings directory %s: %w", filepath.Dir(path), err)
	}

	logger.DebugCF("settings_config", "settings writer set key", map[string]any{
		"scope": scope,
		"path":  path,
		"key":   key,
	})
	return writeSettingsDocument(path, doc)
}

// Unset removes a key (and its subtree) from the settings document at the
// specified scope. The key may be a simple field name or a dotted path.
// Intermediate maps that become empty after deletion are also removed.
func (w *SettingsWriter) Unset(ctx context.Context, scope, key string) error {
	_ = ctx

	path, err := w.resolvePath(scope)
	if err != nil {
		return err
	}

	doc, err := loadSettingsDocument(path)
	if err != nil {
		return err
	}

	keyPath := strings.Split(key, ".")
	if !unsetNestedKey(doc, keyPath) {
		// Key not found — nothing to do, but we still validate and write
		// so the call is idempotent.
	}

	encoded, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("encode settings for validation: %w", err)
	}
	result := ValidateSettingsContent(string(encoded))
	if !result.IsValid {
		return fmt.Errorf("settings validation failed: %s", result.Error)
	}

	if len(doc) == 0 {
		// Document is empty after deletion — remove the file entirely.
		if rmErr := os.Remove(path); rmErr != nil && !os.IsNotExist(rmErr) {
			return fmt.Errorf("remove empty settings file %s: %w", path, rmErr)
		}
		return nil
	}

	return writeSettingsDocument(path, doc)
}

// setNestedKey sets a value at a dot-separated key path within a settings document.
// Intermediate maps are created as needed.
func setNestedKey(doc map[string]any, path []string, value any) {
	if len(path) == 0 {
		return
	}
	if len(path) == 1 {
		doc[path[0]] = value
		return
	}

	key := path[0]
	existing, ok := doc[key]
	if !ok {
		doc[key] = buildNestedMap(path[1:], value)
		return
	}
	childMap, ok := existing.(map[string]any)
	if !ok {
		// Overwrite incompatible intermediate value with a new nested map
		doc[key] = buildNestedMap(path[1:], value)
		return
	}
	setNestedKey(childMap, path[1:], value)
}

// unsetNestedKey removes a key at a dot-separated path. Returns true if a key
// was actually deleted.
func unsetNestedKey(doc map[string]any, path []string) bool {
	if len(path) == 0 {
		return false
	}
	if len(path) == 1 {
		_, existed := doc[path[0]]
		delete(doc, path[0])
		return existed
	}

	childMap, ok := doc[path[0]].(map[string]any)
	if !ok {
		return false
	}
	deleted := unsetNestedKey(childMap, path[1:])
	if deleted && len(childMap) == 0 {
		delete(doc, path[0])
	}
	return deleted
}

// getNestedValue retrieves a value at a dot-separated key path from a settings document.
// The second return value reports whether the key was found.
func getNestedValue(doc map[string]any, path []string) (any, bool) {
	if len(path) == 0 {
		return nil, false
	}

	val, ok := doc[path[0]]
	if !ok {
		return nil, false
	}
	if len(path) == 1 {
		return val, true
	}

	childMap, ok := val.(map[string]any)
	if !ok {
		return nil, false
	}
	return getNestedValue(childMap, path[1:])
}

// buildNestedMap creates a nested map from a key path and a terminal value.
func buildNestedMap(path []string, value any) map[string]any {
	if len(path) == 1 {
		return map[string]any{path[0]: value}
	}
	return map[string]any{path[0]: buildNestedMap(path[1:], value)}
}
