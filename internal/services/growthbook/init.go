package growthbook

import (
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// Config holds the initialization configuration for the GrowthBook system.
type Config struct {
	Enabled     bool
	ClientKey   string
	APIHost     string
	AuthHeaders map[string]string
	// UserAttributesProvider returns the current user attributes for targeting.
	UserAttributesProvider func() UserAttributes
	// ConfigOverrideProvider provides config-level overrides (e.g., from Gates tab).
	ConfigOverrideProvider configOverrideProvider
	// ExposureLogger receives experiment exposure events.
	ExposureLogger ExposureLogger
}

// Init initialises the GrowthBook system.
//
// If the system is not enabled, it returns immediately without registering
// any state. When enabled, it creates the HTTP client, sets up providers,
// and starts periodic refresh.
func Init(cfg Config) {
	if !cfg.Enabled {
		logger.DebugCF("growthbook", "growthbook disabled, skipping init", nil)
		return
	}

	// Set the enabled check function
	SetEnabledFn(func() bool { return true })

	// Set user attributes provider
	if cfg.UserAttributesProvider != nil {
		SetUserAttributesProvider(cfg.UserAttributesProvider)
	}

	// Set config override provider
	if cfg.ConfigOverrideProvider != nil {
		SetConfigOverrideProvider(cfg.ConfigOverrideProvider)
	}

	// Set exposure logger
	if cfg.ExposureLogger != nil {
		SetExposureLogger(cfg.ExposureLogger)
	}

	// Create the HTTP client
	clientKey := cfg.ClientKey
	if clientKey == "" {
		clientKey = ClientKeyFromEnv()
	}
	if clientKey == "" {
		logger.WarnCF("growthbook", "no client key available, growthbook will use disk cache only", nil)
	}

	client := newClient(ClientConfig{
		APIHost:     cfg.APIHost,
		ClientKey:   clientKey,
		AuthHeaders: cfg.AuthHeaders,
		Enabled:     cfg.Enabled,
	})
	SetDefaultClient(client)

	logger.DebugCF("growthbook", "growthbook system initialised", map[string]interface{}{
		"hasClientKey": clientKey != "",
		"hasAuth":      len(cfg.AuthHeaders) > 0,
	})
}

// GetUserAttributes returns the current user attributes.
// Defaults to a minimal set if no provider is configured.
var getUserAttributes = func() UserAttributes {
	return UserAttributes{
		ID:       "unknown",
		DeviceID: "unknown",
		Platform: "unknown",
	}
}

// SetUserAttributesProvider sets the function that provides user attributes.
func SetUserAttributesProvider(fn func() UserAttributes) {
	getUserAttributes = fn
}
