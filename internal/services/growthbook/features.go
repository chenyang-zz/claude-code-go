package growthbook

import (
	"encoding/json"
	"os"
	"sync"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// featureState holds the parsed state for a single feature.
type featureState struct {
	key           string
	defaultValue  interface{}
	hasExperiment bool
}

// featuresByKey stores all parsed feature definitions.
var (
	featuresByKey   = make(map[string]*featureState)
	featuresByKeyMu sync.RWMutex
)

// remoteEvalValues provides in-memory caching of remote eval results.
type remoteEvalCache struct {
	mu     sync.RWMutex
	values map[string]interface{}
}

func newRemoteEvalCache() *remoteEvalCache {
	return &remoteEvalCache{values: make(map[string]interface{})}
}

func (c *remoteEvalCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values = make(map[string]interface{})
}

func (c *remoteEvalCache) get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.values[key]
	return v, ok
}

func (c *remoteEvalCache) set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values[key] = value
}

func (c *remoteEvalCache) snapshot() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make(map[string]interface{}, len(c.values))
	for k, v := range c.values {
		result[k] = v
	}
	return result
}

var remoteEvalValues = newRemoteEvalCache()

// experimentStore stores experiment metadata for exposure logging.
type experimentStore struct {
	mu       sync.RWMutex
	features map[string]StoredExperimentData
}

func newExperimentStore() *experimentStore {
	return &experimentStore{features: make(map[string]StoredExperimentData)}
}

func (e *experimentStore) clear() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.features = make(map[string]StoredExperimentData)
}

func (e *experimentStore) get(key string) (StoredExperimentData, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	v, ok := e.features[key]
	return v, ok
}

func (e *experimentStore) set(key string, data StoredExperimentData) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.features[key]; !exists {
		e.features[key] = data
	}
}

var experimentData = newExperimentStore()

// syncRemoteEvalToDisk persists the current remote eval values to disk cache.
func syncRemoteEvalToDisk() {
	values := remoteEvalValues.snapshot()

	// Write to a JSON file for cross-process persistence
	cachePath := growthBookCachePath()
	if cachePath == "" {
		return
	}

	data, err := json.Marshal(values)
	if err != nil {
		logger.WarnCF("growthbook", "failed to marshal cache", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		logger.WarnCF("growthbook", "failed to write cache", map[string]interface{}{
			"error": err.Error(),
		})
	}
}

// loadDiskCache reads persisted feature values from the JSON disk cache.
// Returns nil if the cache file doesn't exist or is corrupted.
func loadDiskCache() map[string]interface{} {
	cachePath := growthBookCachePath()
	if cachePath == "" {
		return nil
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil
	}

	var values map[string]interface{}
	if err := json.Unmarshal(data, &values); err != nil {
		logger.WarnCF("growthbook", "corrupted disk cache", map[string]interface{}{
			"error": err.Error(),
		})
		return nil
	}
	return values
}

// growthBookCachePath returns the filesystem path for the GrowthBook disk cache.
func growthBookCachePath() string {
	cacheDir := os.Getenv("CLAUDE_CODE_CACHE_DIR")
	if cacheDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		cacheDir = home + "/.claude"
	}
	return cacheDir + "/growthbook_features.json"
}

// getAllFeatureKeys returns all known feature keys from the remote eval payload.
func getAllFeatureKeys() []string {
	keys := make([]string, 0)
	remoteEvalValues.mu.RLock()
	for k := range remoteEvalValues.values {
		keys = append(keys, k)
	}
	remoteEvalValues.mu.RUnlock()
	return keys
}

// resetCaches clears all internal caches (for testing and full reset).
func resetCaches() {
	featuresByKeyMu.Lock()
	featuresByKey = make(map[string]*featureState)
	featuresByKeyMu.Unlock()

	remoteEvalValues.clear()
	experimentData.clear()
}
