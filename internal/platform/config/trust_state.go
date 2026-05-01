package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TrustStateDir is the subdirectory under the user's home directory where trust state is stored.
const TrustStateDir = ".claude"

// TrustStateFile is the filename for persisted trust state.
const TrustStateFile = "trust.json"

// ProjectTrust records the trust decision for a single project directory.
type ProjectTrust struct {
	Trusted   bool   `json:"trusted"`
	TrustedAt string `json:"trustedAt,omitempty"`
}

// TrustState holds the persistent trust decisions for project directories.
type TrustState struct {
	Projects map[string]ProjectTrust `json:"projects"`
}

// NewTrustState creates an empty trust state.
func NewTrustState() *TrustState {
	return &TrustState{
		Projects: make(map[string]ProjectTrust),
	}
}

// LoadTrustState reads the trust state from disk.
// If the file does not exist, it returns an empty state without error.
func LoadTrustState(homeDir string) (*TrustState, error) {
	path := trustStatePath(homeDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewTrustState(), nil
		}
		return nil, fmt.Errorf("read trust state: %w", err)
	}

	state := NewTrustState()
	if err := json.Unmarshal(data, state); err != nil {
		return nil, fmt.Errorf("parse trust state: %w", err)
	}
	if state.Projects == nil {
		state.Projects = make(map[string]ProjectTrust)
	}
	return state, nil
}

// SaveTrustState persists the trust state to disk.
func SaveTrustState(homeDir string, state *TrustState) error {
	if state == nil {
		return nil
	}

	dir := filepath.Join(homeDir, TrustStateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create trust state directory: %w", err)
	}

	path := trustStatePath(homeDir)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("serialize trust state: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write trust state: %w", err)
	}
	return nil
}

// IsTrustAccepted reports whether the given directory or any of its ancestors
// has been marked as trusted. An empty or home directory is treated as trusted
// to avoid unnecessary prompts in the user's personal space.
func IsTrustAccepted(state *TrustState, cwd, homeDir string) bool {
	if state == nil {
		return false
	}

	// Always trust empty or home directory.
	if cwd == "" || cwd == homeDir {
		return true
	}

	normalized := normalizeTrustPath(cwd)
	if normalized == "" {
		return false
	}

	// Check the directory itself and all parent directories.
	current := normalized
	for {
		if t, ok := state.Projects[current]; ok && t.Trusted {
			return true
		}

		parent := normalizeTrustPath(filepath.Dir(current))
		if parent == current || parent == "" {
			break
		}
		current = parent
	}

	return false
}

// AcceptTrust marks the given directory as trusted in the provided state.
func AcceptTrust(state *TrustState, cwd string) {
	if state == nil || state.Projects == nil {
		return
	}

	normalized := normalizeTrustPath(cwd)
	if normalized == "" {
		return
	}

	state.Projects[normalized] = ProjectTrust{
		Trusted:   true,
		TrustedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// trustStatePath returns the full path to the trust state file.
func trustStatePath(homeDir string) string {
	return filepath.Join(homeDir, TrustStateDir, TrustStateFile)
}

// normalizeTrustPath produces a stable, cross-platform path key for trust lookups.
func normalizeTrustPath(p string) string {
	p = filepath.Clean(p)
	if p == "." {
		return ""
	}

	// Use forward slashes for consistent JSON keys across platforms.
	p = filepath.ToSlash(p)

	// Remove trailing slash for consistency (except root).
	p = strings.TrimSuffix(p, "/")

	return p
}
