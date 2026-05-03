package policylimits

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const cacheFilename = "policy-limits.json"

var (
	cacheMu     sync.RWMutex
	cacheHomeDir string
	// sessionCache is the in-memory hot cache for the current process.
	sessionCache map[string]Restriction
)

// SetCacheHomeDir configures the directory where the policy limits cache file
// is stored. Safe to call multiple times; the most recent value wins.
func SetCacheHomeDir(homeDir string) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cacheHomeDir = homeDir
}

func cachePath() string {
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	if cacheHomeDir == "" {
		return cacheFilename
	}
	return filepath.Join(cacheHomeDir, ".claude", cacheFilename)
}

// LoadCache reads the persistent cache file and returns the restrictions.
// Returns nil when the file does not exist or is unreadable/invalid.
func LoadCache() (map[string]Restriction, error) {
	cacheMu.RLock()
	if sessionCache != nil {
		cp := make(map[string]Restriction, len(sessionCache))
		for k, v := range sessionCache {
			cp[k] = v
		}
		cacheMu.RUnlock()
		return cp, nil
	}
	cacheMu.RUnlock()

	path := cachePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var resp PolicyLimitsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("invalid cache format: %w", err)
	}

	cacheMu.Lock()
	sessionCache = resp.Restrictions
	cacheMu.Unlock()
	return resp.Restrictions, nil
}

// SaveCache persists the supplied restrictions to the cache file.
func SaveCache(restrictions map[string]Restriction) error {
	path := cachePath()
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}

	data, err := json.MarshalIndent(PolicyLimitsResponse{Restrictions: restrictions}, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}

	cacheMu.Lock()
	sessionCache = restrictions
	cacheMu.Unlock()

	logger.DebugCF("policylimits", "cache saved", map[string]any{
		"path": path,
	})
	return nil
}

// ClearCache removes both the session cache and the persistent cache file.
func ClearCache() error {
	cacheMu.Lock()
	sessionCache = nil
	cacheMu.Unlock()

	path := cachePath()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// computeChecksum generates a stable checksum for the given restrictions.
// Used as an ETag value for conditional HTTP requests.
func computeChecksum(restrictions map[string]Restriction) string {
	// Simple stable JSON marshalling for checksum consistency.
	data, _ := json.Marshal(restrictions)
	return fmt.Sprintf("%x", data)
}
