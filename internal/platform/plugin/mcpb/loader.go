package mcpb

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// ConfigLoader provides the dependencies needed to load an MCPB file and
// manage user configuration. It abstracts the plugin config store and secure
// storage so the MCPB handler does not depend directly on settings or
// keychain implementations.
type ConfigLoader struct {
	// PluginPath is the absolute path to the plugin root directory.
	PluginPath string
	// PluginID is the plugin identifier (e.g. "name@marketplace").
	PluginID string
	// NonSensitiveConfig holds non-sensitive plugin configuration loaded from
	// settings.json (pluginConfigs[pluginID].mcpServers[serverName]).
	NonSensitiveConfig UserConfigValues
	// SensitiveLoader loads sensitive values from secure storage. It receives
	// a composite key "pluginId/serverName" and returns stored key-value pairs.
	SensitiveLoader func(key string) UserConfigValues
	// SensitiveSaver saves sensitive values to secure storage.
	SensitiveSaver func(key string, values UserConfigValues) error
	// NonSensitiveSaver saves non-sensitive values to settings.json.
	NonSensitiveSaver func(values UserConfigValues) error
}

// LoadMcpbConfig loads an MCPB file from source (a local path or URL),
// extracts it, parses the manifest, builds an MCP server configuration, and
// manages caching. It returns either a load result with a ready-to-use server
// config, or a needs-config result when user configuration is required.
//
// onProgress is called with status messages (may be nil).
// providedUserConfig supplies user configuration values (may be nil).
func LoadMcpbConfig(
	source string,
	cl *ConfigLoader,
	onProgress ProgressCallback,
	providedUserConfig UserConfigValues,
) (*McpbLoadResult, *McpbNeedsConfigResult, error) {
	cacheDir := GetMcpbCacheDir(cl.PluginPath)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	logger.DebugCF("plugin.mcpb", "loading MCPB", map[string]any{
		"source": source,
	})

	// Check cache first.
	meta := LoadCacheMetadata(cacheDir, source)
	if meta != nil && !CheckMcpbChanged(source, cl.PluginPath) {
		logger.DebugCF("plugin.mcpb", "using cached MCPB", map[string]any{
			"path": meta.ExtractedPath,
			"hash": meta.ContentHash,
		})
		return loadFromCache(meta, cl, onProgress, providedUserConfig)
	}

	// Load MCPB file data.
	data, contentHash, err := loadMcpbData(source, cl.PluginPath, cacheDir, onProgress)
	if err != nil {
		return nil, nil, err
	}

	logger.DebugCF("plugin.mcpb", "MCPB content hash", map[string]any{
		"hash": contentHash,
	})

	if onProgress != nil {
		onProgress("Extracting MCPB archive...")
	}

	// Extract ZIP.
	extractPath := filepath.Join(cacheDir, contentHash)
	if err := ExtractMcpb(data, extractPath, onProgress); err != nil {
		return nil, nil, fmt.Errorf("failed to extract MCPB: %w", err)
	}

	// Parse manifest.
	manifest, err := ParseManifest(extractPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse MCPB manifest: %w", err)
	}

	// Handle user configuration if required.
	if manifest.UserConfig != nil && len(manifest.UserConfig) > 0 {
		return handleUserConfig(manifest, extractPath, contentHash, source, cacheDir, cl,
			onProgress, providedUserConfig)
	}

	// No user config needed — build server config directly.
	if onProgress != nil {
		onProgress("Generating MCP server configuration...")
	}

	serverConfig, err := BuildMcpServerConfig(manifest, extractPath, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build MCP server config: %w", err)
	}

	// Save cache metadata.
	now := time.Now().UTC().Format(time.RFC3339)
	if err := SaveCacheMetadata(cacheDir, source, &McpbCacheMetadata{
		Source:        source,
		ContentHash:   contentHash,
		ExtractedPath: extractPath,
		CachedAt:      now,
		LastChecked:   now,
	}); err != nil {
		logger.DebugCF("plugin.mcpb", "failed to save cache metadata", map[string]any{
			"error": err.Error(),
		})
	}

	logger.DebugCF("plugin.mcpb", "loaded MCPB successfully", map[string]any{
		"name": manifest.Name,
		"path": extractPath,
	})

	return &McpbLoadResult{
		Manifest:      *manifest,
		McpConfig:     *serverConfig,
		ExtractedPath: extractPath,
		ContentHash:   contentHash,
	}, nil, nil
}

// loadMcpbData loads the raw MCPB file bytes from a local path or URL.
func loadMcpbData(source, pluginPath, cacheDir string, onProgress ProgressCallback) ([]byte, string, error) {
	if IsURL(source) {
		h := fmt.Sprintf("%x", md5Hash([]byte(source))[:4])
		destPath := filepath.Join(cacheDir, h+".mcpb")
		data, hash, err := DownloadMcpb(source, destPath, onProgress)
		if err != nil {
			return nil, "", err
		}
		return data, hash, nil
	}

	localPath := filepath.Join(pluginPath, source)
	if onProgress != nil {
		onProgress("Loading " + source + "...")
	}

	data, err := os.ReadFile(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", fmt.Errorf("MCPB file not found: %s", localPath)
		}
		return nil, "", fmt.Errorf("failed to read MCPB file %s: %w", localPath, err)
	}

	return data, contentHash(data), nil
}

// loadFromCache attempts to produce a result from cached MCPB data.
func loadFromCache(
	meta *McpbCacheMetadata,
	cl *ConfigLoader,
	onProgress ProgressCallback,
	providedUserConfig UserConfigValues,
) (*McpbLoadResult, *McpbNeedsConfigResult, error) {
	manifest, err := ParseManifest(meta.ExtractedPath)
	if err != nil {
		return nil, nil, fmt.Errorf("cached manifest error: %w", err)
	}

	// Handle user configuration if required.
	if manifest.UserConfig != nil && len(manifest.UserConfig) > 0 {
		savedConfig := LoadMcpServerUserConfig(
			cl.PluginID,
			manifest.Name,
			cl.NonSensitiveConfig,
			cl.SensitiveLoader,
		)
		userConfig := providedUserConfig
		if userConfig == nil {
			userConfig = savedConfig
		} else {
			// Merge provided over saved.
			if savedConfig != nil {
				for k, v := range savedConfig {
					if _, ok := userConfig[k]; !ok {
						userConfig[k] = v
					}
				}
			}
		}
		if userConfig == nil {
			userConfig = make(UserConfigValues)
		}

		validationErrors := ValidateUserConfig(userConfig, manifest.UserConfig)
		if len(validationErrors) > 0 {
			return nil, &McpbNeedsConfigResult{
				Status:           "needs-config",
				Manifest:         *manifest,
				ExtractedPath:    meta.ExtractedPath,
				ContentHash:      meta.ContentHash,
				ConfigSchema:     manifest.UserConfig,
				ExistingConfig:   savedConfig,
				ValidationErrors: validationErrors,
			}, nil
		}

		// Save provided config if supplied.
		if providedUserConfig != nil {
			if err := SaveMcpServerUserConfig(
				cl.PluginID,
				manifest.Name,
				providedUserConfig,
				manifest.UserConfig,
				cl.NonSensitiveSaver,
				cl.SensitiveSaver,
			); err != nil {
				logger.DebugCF("plugin.mcpb", "failed to save user config", map[string]any{
					"error": err.Error(),
				})
			}
		}

		serverConfig, err := BuildMcpServerConfig(manifest, meta.ExtractedPath, userConfig)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to build MCP server config: %w", err)
		}

		return &McpbLoadResult{
			Manifest:      *manifest,
			McpConfig:     *serverConfig,
			ExtractedPath: meta.ExtractedPath,
			ContentHash:   meta.ContentHash,
		}, nil, nil
	}

	serverConfig, err := BuildMcpServerConfig(manifest, meta.ExtractedPath, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build MCP server config: %w", err)
	}

	return &McpbLoadResult{
		Manifest:      *manifest,
		McpConfig:     *serverConfig,
		ExtractedPath: meta.ExtractedPath,
		ContentHash:   meta.ContentHash,
	}, nil, nil
}

// handleUserConfig processes the user_config path for first-time loading.
func handleUserConfig(
	manifest *McpbManifest,
	extractPath, contentHash, source, cacheDir string,
	cl *ConfigLoader,
	onProgress ProgressCallback,
	providedUserConfig UserConfigValues,
) (*McpbLoadResult, *McpbNeedsConfigResult, error) {
	savedConfig := LoadMcpServerUserConfig(
		cl.PluginID,
		manifest.Name,
		cl.NonSensitiveConfig,
		cl.SensitiveLoader,
	)

	userConfig := providedUserConfig
	if userConfig == nil {
		userConfig = savedConfig
	} else if savedConfig != nil {
		for k, v := range savedConfig {
			if _, ok := userConfig[k]; !ok {
				userConfig[k] = v
			}
		}
	}
	if userConfig == nil {
		userConfig = make(UserConfigValues)
	}

	validationErrors := ValidateUserConfig(userConfig, manifest.UserConfig)
	if len(validationErrors) > 0 {
		// Save cache metadata even with incomplete config.
		now := time.Now().UTC().Format(time.RFC3339)
		if err := SaveCacheMetadata(cacheDir, source, &McpbCacheMetadata{
			Source:        source,
			ContentHash:   contentHash,
			ExtractedPath: extractPath,
			CachedAt:      now,
			LastChecked:   now,
		}); err != nil {
			logger.DebugCF("plugin.mcpb", "failed to save cache metadata", map[string]any{
				"error": err.Error(),
			})
		}

		return nil, &McpbNeedsConfigResult{
			Status:           "needs-config",
			Manifest:         *manifest,
			ExtractedPath:    extractPath,
			ContentHash:      contentHash,
			ConfigSchema:     manifest.UserConfig,
			ExistingConfig:   savedConfig,
			ValidationErrors: validationErrors,
		}, nil
	}

	// Save provided config if supplied.
	if providedUserConfig != nil {
		if err := SaveMcpServerUserConfig(
			cl.PluginID,
			manifest.Name,
			providedUserConfig,
			manifest.UserConfig,
			cl.NonSensitiveSaver,
			cl.SensitiveSaver,
		); err != nil {
			logger.DebugCF("plugin.mcpb", "failed to save user config", map[string]any{
				"error": err.Error(),
			})
		}
	}

	if onProgress != nil {
		onProgress("Generating MCP server configuration...")
	}

	serverConfig, err := BuildMcpServerConfig(manifest, extractPath, userConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build MCP server config: %w", err)
	}

	// Save cache metadata.
	now := time.Now().UTC().Format(time.RFC3339)
	if err := SaveCacheMetadata(cacheDir, source, &McpbCacheMetadata{
		Source:        source,
		ContentHash:   contentHash,
		ExtractedPath: extractPath,
		CachedAt:      now,
		LastChecked:   now,
	}); err != nil {
		logger.DebugCF("plugin.mcpb", "failed to save cache metadata", map[string]any{
			"error": err.Error(),
		})
	}

	return &McpbLoadResult{
		Manifest:      *manifest,
		McpConfig:     *serverConfig,
		ExtractedPath: extractPath,
		ContentHash:   contentHash,
	}, nil, nil
}

// md5Hash returns the MD5 hash of data.
func md5Hash(data []byte) []byte {
	h := md5.Sum(data)
	return h[:]
}
