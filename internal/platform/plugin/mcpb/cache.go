package mcpb

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// cacheDirName is the subdirectory under a plugin's path where MCPB cache
// data is stored.
const cacheDirName = ".mcpb-cache"

// GetMcpbCacheDir returns the MCPB cache directory path for a plugin.
func GetMcpbCacheDir(pluginPath string) string {
	return filepath.Join(pluginPath, cacheDirName)
}

// metadataFileName returns the metadata file path for a cached MCPB source.
// The source is hashed with MD5 to produce a stable filename.
func metadataFileName(cacheDir, source string) string {
	h := md5.Sum([]byte(source))
	sourceHash := fmt.Sprintf("%x", h[:4])
	return filepath.Join(cacheDir, sourceHash+".metadata.json")
}

// LoadCacheMetadata reads cached MCPB metadata for a source. Returns nil if
// no cache metadata exists or the file cannot be read.
func LoadCacheMetadata(cacheDir, source string) *McpbCacheMetadata {
	path := metadataFileName(cacheDir, source)
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.DebugCF("plugin.mcpb", "failed to load cache metadata", map[string]any{
				"path":  path,
				"error": err.Error(),
			})
		}
		return nil
	}

	var meta McpbCacheMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		logger.DebugCF("plugin.mcpb", "failed to parse cache metadata", map[string]any{
			"path":  path,
			"error": err.Error(),
		})
		return nil
	}

	return &meta
}

// SaveCacheMetadata writes MCPB cache metadata to disk.
func SaveCacheMetadata(cacheDir, source string, meta *McpbCacheMetadata) error {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("failed to create cache directory %s: %w", cacheDir, err)
	}

	path := metadataFileName(cacheDir, source)
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache metadata: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write cache metadata %s: %w", path, err)
	}

	logger.DebugCF("plugin.mcpb", "saved cache metadata", map[string]any{
		"source": source,
		"hash":   meta.ContentHash,
		"path":   meta.ExtractedPath,
	})

	return nil
}

// CheckMcpbChanged determines whether an MCPB source needs to be re-loaded.
// It returns true if:
//   - No cache metadata exists
//   - The extraction directory is missing
//   - For local files, mtime is newer than cachedAt
func CheckMcpbChanged(source, pluginPath string) bool {
	cacheDir := GetMcpbCacheDir(pluginPath)
	meta := LoadCacheMetadata(cacheDir, source)
	if meta == nil {
		return true
	}

	// Check extraction directory still exists.
	if _, err := os.Stat(meta.ExtractedPath); err != nil {
		if os.IsNotExist(err) {
			logger.DebugCF("plugin.mcpb", "extraction path missing", map[string]any{
				"path": meta.ExtractedPath,
			})
		} else {
			logger.DebugCF("plugin.mcpb", "extraction path inaccessible", map[string]any{
				"path":  meta.ExtractedPath,
				"error": err.Error(),
			})
		}
		return true
	}

	// For local files, check mtime.
	if !IsURL(source) {
		localPath := filepath.Join(pluginPath, source)
		info, err := os.Stat(localPath)
		if err != nil {
			if os.IsNotExist(err) {
				logger.DebugCF("plugin.mcpb", "source file missing", map[string]any{
					"path": localPath,
				})
			} else {
				logger.DebugCF("plugin.mcpb", "source file inaccessible", map[string]any{
					"path":  localPath,
					"error": err.Error(),
				})
			}
			return true
		}

		cachedAt, err := time.Parse(time.RFC3339, meta.CachedAt)
		if err != nil {
			return true
		}

		fileTime := time.UnixMilli(info.ModTime().UnixMilli())
		if fileTime.After(cachedAt) {
			logger.DebugCF("plugin.mcpb", "source file modified since cache", map[string]any{
				"source":    source,
				"fileTime":  fileTime.Format(time.RFC3339),
				"cachedAt":  cachedAt.Format(time.RFC3339),
			})
			return true
		}
	}

	// For URLs, assume unchanged unless explicitly refreshed.
	return false
}
