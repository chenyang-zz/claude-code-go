package growthbook

import (
	"sync"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// ExposureLogger provides experiment exposure logging.
// The default implementation is a no-op stub. A real implementation
// should be wired to the 1P event logging pipeline when available.
type ExposureLogger interface {
	LogExposure(feature string, data StoredExperimentData, attrs UserAttributes)
}

type noopExposureLogger struct{}

func (n *noopExposureLogger) LogExposure(_ string, _ StoredExperimentData, _ UserAttributes) {}

var exposureLogger ExposureLogger = &noopExposureLogger{}

// SetExposureLogger sets the exposure logger implementation.
// When the 1P event logging pipeline is available, wire it here.
func SetExposureLogger(l ExposureLogger) {
	exposureLogger = l
}

// Exposure tracking state.
var (
	loggedExposures   = make(map[string]bool)
	loggedExposuresMu sync.RWMutex

	pendingExposures   = make(map[string]bool)
	pendingExposuresMu sync.Mutex
)

// isEnabled reports whether this feature's exposures have been logged.
func isExposureLogged(feature string) bool {
	loggedExposuresMu.RLock()
	defer loggedExposuresMu.RUnlock()
	return loggedExposures[feature]
}

// markExposureLogged marks a feature's exposure as logged.
func markExposureLogged(feature string) {
	loggedExposuresMu.Lock()
	defer loggedExposuresMu.Unlock()
	loggedExposures[feature] = true
}

// logExposureForFeature logs an experiment exposure for the given feature.
// Deduplicates within a session - each feature is logged at most once.
func logExposureForFeature(feature string) {
	if isExposureLogged(feature) {
		return
	}
	markExposureLogged(feature)

	data, ok := experimentData.get(feature)
	if !ok {
		return
	}

	// Exposure events are bridged to the analytics Emitter via SetExposureLogger
	// in bootstrap (internal/app/bootstrap/app.go:514). When the 1P event
	// logging pipeline is used directly, switch to NewFPExposureLogger.
	attrs := getUserAttributes()
	exposureLogger.LogExposure(feature, data, attrs)

	logger.DebugCF("growthbook", "logged exposure for feature", map[string]interface{}{
		"feature":      feature,
		"experimentId": data.ExperimentID,
		"variationId":  data.VariationID,
	})
}

// addPendingExposure records a feature that needs exposure logging after init.
func addPendingExposure(feature string) {
	pendingExposuresMu.Lock()
	defer pendingExposuresMu.Unlock()
	pendingExposures[feature] = true
}

// flushPendingExposures logs all pending exposures and clears the pending set.
func flushPendingExposures() {
	pendingExposuresMu.Lock()
	pendings := make([]string, 0, len(pendingExposures))
	for f := range pendingExposures {
		pendings = append(pendings, f)
	}
	pendingExposures = make(map[string]bool)
	pendingExposuresMu.Unlock()

	for _, f := range pendings {
		logExposureForFeature(f)
	}
}

// GetValue evaluates a feature flag using the full priority chain:
// env override > config override > remote eval > disk cache > default value
func GetValue(feature string, defaultValue interface{}) (interface{}, string) {
	// 1. Check env var overrides
	if v, ok := CheckEnvOverride(feature); ok {
		logger.DebugCF("growthbook", "feature from env override", map[string]interface{}{
			"feature": feature,
		})
		return v, "envOverride"
	}

	// 2. Check config overrides
	if v, ok := CheckConfigOverride(feature); ok {
		return v, "configOverride"
	}

	if !IsGrowthBookEnabled() {
		return defaultValue, "defaultValue"
	}

	// 3. Remote eval values (in-memory, from latest payload)
	if v, ok := remoteEvalValues.get(feature); ok {
		// Check for experiment data to log exposure
		if _, hasExp := experimentData.get(feature); hasExp {
			logExposureForFeature(feature)
		}
		return v, "remoteEval"
	}

	// 4. Disk cache
	if cache := loadDiskCache(); cache != nil {
		if v, ok := cache[feature]; ok {
			// Defer exposure logging until after init if experiment data isn't loaded yet
			if _, hasExp := experimentData.get(feature); hasExp {
				logExposureForFeature(feature)
			} else {
				addPendingExposure(feature)
			}
			return v, "diskCache"
		}
	}

	// 5. Default value
	return defaultValue, "defaultValue"
}

// GetCached is the non-blocking cached read (equivalent to
// getFeatureValue_CACHED_MAY_BE_STALE in TS). Prefer this for
// startup-critical paths.
func GetCached(feature string, defaultValue interface{}) interface{} {
	// 1. Check env var overrides
	if v, ok := CheckEnvOverride(feature); ok {
		return v
	}

	// 2. Check config overrides
	if v, ok := CheckConfigOverride(feature); ok {
		return v
	}

	if !IsGrowthBookEnabled() {
		return defaultValue
	}

	// Handle exposure logging: log now if experiment data is available,
	// defer to pending if not
	if _, hasExp := experimentData.get(feature); hasExp {
		logExposureForFeature(feature)
	} else {
		addPendingExposure(feature)
	}

	// 3. In-memory remote eval values
	if v, ok := remoteEvalValues.get(feature); ok {
		return v
	}

	// 4. Disk cache
	if cache := loadDiskCache(); cache != nil {
		if v, ok := cache[feature]; ok {
			return v
		}
	}

	// 5. Default
	return defaultValue
}

// CheckGate is a boolean gate checker equivalent to
// checkStatsigFeatureGate/checkSecurityRestrictionGate in TS.
// Returns false if the feature is not found or not a boolean.
func CheckGate(feature string) bool {
	v := GetCached(feature, false)
	b, ok := v.(bool)
	if !ok {
		return false
	}
	return b
}

// CheckGateOrBlocking checks a boolean gate with fallback-to-blocking semantics.
// If the cached value is true, returns immediately. Otherwise, blocks on
// initialization to get a fresh value.
func CheckGateOrBlocking(feature string) bool {
	// Check overrides first
	if v, ok := CheckEnvOverride(feature); ok {
		if b, ok := v.(bool); ok {
			return b
		}
		return false
	}
	if v, ok := CheckConfigOverride(feature); ok {
		if b, ok := v.(bool); ok {
			return b
		}
		return false
	}

	if !IsGrowthBookEnabled() {
		return false
	}

	// Fast path: cached value says true
	cached := GetCached(feature, false)
	if b, ok := cached.(bool); ok && b {
		return true
	}

	// Slow path: use the blocking init and re-fetch
	// If client isn't initialized yet, try init
	client := GetDefaultClient()
	if client == nil {
		return false
	}

	// Flush pending exposures after init
	flushPendingExposures()

	v, _ := GetValue(feature, false)
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

// IsGrowthBookEnabled returns whether the GrowthBook system is active.
// In the TS source this depends on is1PEventLoggingEnabled; in Go it is
// controlled by the FlagGrowthBook feature flag and environment variable.
// This function should be overridden during Init to check the feature flag.
var isGrowthBookEnabledFn = func() bool { return true }

// IsGrowthBookEnabled delegates to the configured check function.
func IsGrowthBookEnabled() bool {
	return isGrowthBookEnabledFn()
}

// SetEnabledFn sets the function used to determine if GrowthBook is enabled.
func SetEnabledFn(fn func() bool) {
	isGrowthBookEnabledFn = fn
}

// resetExposureTracking clears all exposure tracking state (for testing and reset).
func resetExposureTracking() {
	loggedExposuresMu.Lock()
	loggedExposures = make(map[string]bool)
	loggedExposuresMu.Unlock()

	pendingExposuresMu.Lock()
	pendingExposures = make(map[string]bool)
	pendingExposuresMu.Unlock()
}
