package policylimits

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const loadingPromiseTimeout = 30 * time.Second

// essentialTrafficDenyOnMiss lists policies that default to denied when
// essential-traffic-only mode is active and the policy cache is unavailable.
// Without this, a cache miss or network timeout would silently re-enable
// these features for HIPAA orgs.
var essentialTrafficDenyOnMiss = map[string]bool{
	"allow_product_feedback": true,
}

// IsEssentialTrafficOnly reports whether the host is running in
// essential-traffic-only mode. This is a minimal placeholder until the full
// privacyLevel / HIPAA mode system is migrated.
func IsEssentialTrafficOnly() bool {
	return os.Getenv("CLAUDE_CODE_ESSENTIAL_TRAFFIC_ONLY") == "1"
}

// IsAllowed reports whether the given policy action is permitted.
//
// Returns true (fail-open) when:
//   - the policy limits system is disabled by feature flag
//   - the user is not eligible for policy limits
//   - the policy cache is unavailable
//   - the policy is unknown
//
// The only exception is essential-traffic-only mode combined with a policy
// listed in essentialTrafficDenyOnMiss; in that case a missing cache
// returns denied.
//
// The returned string is a human-friendly denial reason when allowed is false.
func IsAllowed(action PolicyAction) (bool, string) {
	if !IsPolicyLimitsEnabled() {
		return true, ""
	}

	restrictions, err := LoadCache()
	if err != nil || restrictions == nil {
		if IsEssentialTrafficOnly() && essentialTrafficDenyOnMiss[string(action)] {
			return false, "This feature is disabled in essential-traffic-only mode."
		}
		return true, ""
	}

	r, ok := restrictions[string(action)]
	if !ok {
		return true, "" // unknown policy = allowed
	}

	if r.Allowed {
		return true, ""
	}

	return false, "This feature is not available for your account."
}

// IsAllowedString is a convenience wrapper that accepts a plain string
// policy identifier. Used by call sites that only know the policy name.
func IsAllowedString(policy string) (bool, string) {
	return IsAllowed(PolicyAction(policy))
}

var (
	loadMu              sync.Mutex
	loadingPromise      chan struct{}
	loadingPromiseReady bool
)

// InitializeLoadingPromise creates the loading promise early so that other
// systems can await policy limits loading even before LoadAndStart has been
// called. Only creates the promise when the user is eligible.
func InitializeLoadingPromise() {
	loadMu.Lock()
	defer loadMu.Unlock()
	if loadingPromiseReady {
		return
	}
	if !IsEligible() {
		return
	}
	loadingPromise = make(chan struct{})
	loadingPromiseReady = true

	// 30-second timeout to prevent deadlocks.
	go func() {
		select {
		case <-loadingPromise:
			// resolved normally
		case <-time.After(loadingPromiseTimeout):
			logger.DebugCF("policylimits", "loading promise timed out, resolving anyway", nil)
			resolveLoadingPromise()
		}
	}()
}

// WaitForLoad blocks until the initial policy limits loading completes.
// Returns immediately if the user is not eligible or loading has already
// completed.
func WaitForLoad() {
	loadMu.Lock()
	p := loadingPromise
	loadMu.Unlock()

	if p == nil {
		return
	}
	<-p
}

// LoadAndStart fetches policy limits and starts background polling.
// Safe to call multiple times.
func LoadAndStart(ctx context.Context) {
	loadMu.Lock()
	if !loadingPromiseReady {
		loadingPromise = make(chan struct{})
		loadingPromiseReady = true
	}
	loadMu.Unlock()

	defer resolveLoadingPromise()

	if !IsEligible() {
		return
	}

	restrictions, err := fetchAndLoad(ctx)
	if err != nil {
		logger.DebugCF("policylimits", "initial load failed, continuing without restrictions", map[string]any{
			"error": err.Error(),
		})
	} else if restrictions != nil {
		logger.DebugCF("policylimits", "initial load succeeded", map[string]any{
			"restrictions_count": len(restrictions),
		})
	}

	if IsEligible() {
		StartPolling(ctx)
	}
}

// Refresh clears the cache and re-fetches policy limits. Used after auth
// state changes (e.g. login / logout).
func Refresh(ctx context.Context) {
	_ = ClearCache()
	if !IsEligible() {
		return
	}
	_, _ = fetchAndLoad(ctx)
	logger.DebugCF("policylimits", "refreshed after auth change", nil)
}

func resolveLoadingPromise() {
	loadMu.Lock()
	defer loadMu.Unlock()
	if loadingPromise != nil {
		close(loadingPromise)
		loadingPromise = nil
	}
}

func fetchAndLoad(ctx context.Context) (map[string]Restriction, error) {
	if !IsEligible() {
		return nil, nil
	}

	cached, err := LoadCache()
	if err != nil {
		cached = nil
	}

	var cachedChecksum string
	if cached != nil {
		cachedChecksum = computeChecksum(cached)
	}

	c := getConfig()
	var apiKey, accessToken, baseURL string
	if c != nil {
		apiKey = c.APIKey
		baseURL = c.APIBaseURL
	}
	if tokens, _ := loadCurrentTokens(); tokens != nil {
		accessToken = tokens.AccessToken
	}

	resp, err := Fetch(ctx, apiKey, accessToken, baseURL, cachedChecksum)
	if err != nil {
		if cached != nil {
			logger.DebugCF("policylimits", "using stale cache after fetch failure", nil)
			_ = SaveCache(cached)
			return cached, nil
		}
		return nil, err
	}

	// 304 Not Modified — cache is still valid.
	if resp == nil {
		logger.DebugCF("policylimits", "cache still valid (304)", nil)
		if cached != nil {
			_ = SaveCache(cached)
		}
		return cached, nil
	}

	// Empty restrictions (404-equivalent) — delete cache.
	if len(resp.Restrictions) == 0 {
		_ = ClearCache()
		logger.DebugCF("policylimits", "empty restrictions, cache cleared", nil)
		return resp.Restrictions, nil
	}

	_ = SaveCache(resp.Restrictions)
	logger.DebugCF("policylimits", "new restrictions applied", map[string]any{
		"count": len(resp.Restrictions),
	})
	return resp.Restrictions, nil
}
