package claudeailimits

import (
	"sync"
)

// SettingsStore is the persistence contract used by the claudeailimits
// package. The contract is satisfied by `internal/platform/config.GlobalSettingsStore`
// in production and by tests with an in-memory fake.
//
// The store works on JSON-friendly maps so this package never imports the
// platform/config layer (avoids a layering cycle).
type SettingsStore interface {
	// SaveLastClaudeAILimits persists a JSON-friendly snapshot under the
	// global settings `lastClaudeAILimits` key. nil clears the entry.
	SaveLastClaudeAILimits(snapshot map[string]any) error
	// GetLastClaudeAILimits returns the persisted snapshot map, or nil when
	// no snapshot has been persisted yet.
	GetLastClaudeAILimits() map[string]any
}

var (
	// settingsStoreMu guards the package-level settings store handle. The
	// store is set once at bootstrap and read by hot-path consumers.
	settingsStoreMu sync.RWMutex
	// settingsStore is the persistence backend used by SaveClaudeAILimits
	// and LoadClaudeAILimits. nil means persistence is not configured.
	settingsStore SettingsStore
)

// SetSettingsStore registers the global persistence backend. Called from
// bootstrap; safe to call again (subsequent calls override the prior value).
func SetSettingsStore(store SettingsStore) {
	settingsStoreMu.Lock()
	defer settingsStoreMu.Unlock()
	settingsStore = store
}

// getSettingsStore returns the registered persistence backend or nil when
// none has been registered.
func getSettingsStore() SettingsStore {
	settingsStoreMu.RLock()
	defer settingsStoreMu.RUnlock()
	return settingsStore
}

// SaveClaudeAILimits persists the supplied snapshot through the registered
// settings store. Returns nil silently when the feature flag is disabled or
// no store is registered, mirroring the TS-side gating semantics.
func SaveClaudeAILimits(limits *ClaudeAILimits) error {
	if !IsClaudeAILimitsEnabled() {
		return nil
	}
	store := getSettingsStore()
	if store == nil {
		return nil
	}
	return store.SaveLastClaudeAILimits(encodeLimits(limits))
}

// LoadClaudeAILimits retrieves the persisted snapshot through the registered
// settings store. Returns nil silently when the feature flag is disabled,
// no store is registered, or no snapshot has been persisted yet.
func LoadClaudeAILimits() (*ClaudeAILimits, error) {
	if !IsClaudeAILimitsEnabled() {
		return nil, nil
	}
	store := getSettingsStore()
	if store == nil {
		return nil, nil
	}
	raw := store.GetLastClaudeAILimits()
	if raw == nil {
		return nil, nil
	}
	return decodeLimits(raw), nil
}

// encodeLimits projects a ClaudeAILimits onto a JSON-friendly map. Returns nil
// when the input is nil so callers can clear the persisted entry.
func encodeLimits(limits *ClaudeAILimits) map[string]any {
	if limits == nil {
		return nil
	}
	out := map[string]any{
		"status":                            string(limits.Status),
		"unifiedRateLimitFallbackAvailable": limits.UnifiedRateLimitFallbackAvailable,
		"isUsingOverage":                    limits.IsUsingOverage,
	}
	if limits.ResetsAt != 0 {
		out["resetsAt"] = limits.ResetsAt
	}
	if limits.RateLimitType != "" {
		out["rateLimitType"] = string(limits.RateLimitType)
	}
	if limits.HasUtilization {
		out["utilization"] = limits.Utilization
	}
	if limits.OverageStatus != "" {
		out["overageStatus"] = string(limits.OverageStatus)
	}
	if limits.OverageResetsAt != 0 {
		out["overageResetsAt"] = limits.OverageResetsAt
	}
	if limits.OverageDisabledReason != "" {
		out["overageDisabledReason"] = string(limits.OverageDisabledReason)
	}
	if limits.HasSurpassedThreshold {
		out["surpassedThreshold"] = limits.SurpassedThreshold
	}
	if limits.CachedExtraUsageDisabledReason != "" {
		out["cachedExtraUsageDisabledReason"] = string(limits.CachedExtraUsageDisabledReason)
	}
	return out
}

// decodeLimits projects a JSON-decoded map back onto ClaudeAILimits. Unknown
// fields are silently dropped, matching the open-string semantics elsewhere.
func decodeLimits(raw map[string]any) *ClaudeAILimits {
	if raw == nil {
		return nil
	}
	limits := &ClaudeAILimits{}
	if v, ok := raw["status"].(string); ok {
		limits.Status = QuotaStatus(v)
	} else {
		limits.Status = QuotaStatusAllowed
	}
	if v, ok := raw["unifiedRateLimitFallbackAvailable"].(bool); ok {
		limits.UnifiedRateLimitFallbackAvailable = v
	}
	if v, ok := raw["isUsingOverage"].(bool); ok {
		limits.IsUsingOverage = v
	}
	limits.ResetsAt = numberToInt64(raw["resetsAt"])
	if v, ok := raw["rateLimitType"].(string); ok {
		limits.RateLimitType = RateLimitType(v)
	}
	if v, ok := raw["utilization"].(float64); ok {
		limits.Utilization = v
		limits.HasUtilization = true
	}
	if v, ok := raw["overageStatus"].(string); ok {
		limits.OverageStatus = QuotaStatus(v)
	}
	limits.OverageResetsAt = numberToInt64(raw["overageResetsAt"])
	if v, ok := raw["overageDisabledReason"].(string); ok {
		limits.OverageDisabledReason = OverageDisabledReason(v)
	}
	if v, ok := raw["surpassedThreshold"].(float64); ok {
		limits.SurpassedThreshold = v
		limits.HasSurpassedThreshold = true
	}
	if v, ok := raw["cachedExtraUsageDisabledReason"].(string); ok {
		limits.CachedExtraUsageDisabledReason = OverageDisabledReason(v)
	}
	return limits
}

// numberToInt64 coerces a JSON-decoded numeric value (float64 or int) into
// int64. Returns 0 when the input is nil or not numeric.
func numberToInt64(value any) int64 {
	switch v := value.(type) {
	case float64:
		return int64(v)
	case int:
		return int64(v)
	case int64:
		return v
	}
	return 0
}
