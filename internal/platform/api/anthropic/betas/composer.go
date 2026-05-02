// Package betas provides the Anthropic beta header composition engine.
package betas

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// composerCacheKey builds a cache key from the inputs that affect beta composition.
func composerCacheKey(model string, opts BetaOptions) string {
	var ffParts []string
	for k, v := range opts.FeatureFlags {
		if v {
			ffParts = append(ffParts, k+"=1")
		} else {
			ffParts = append(ffParts, k+"=0")
		}
	}
	sort.Strings(ffParts)

	return fmt.Sprintf("%s|%s|%s|%s|%v|%v|%v|%v|%v|%s|%s|%s",
		model, opts.Provider, opts.UserType, opts.Entrypoint,
		opts.DisableExperimentalBetas, opts.DisableInterleavedThinking,
		opts.UseAPIContextManagement, opts.IsSubscriber, opts.IsAgenticQuery,
		opts.AnthropicBetas, opts.UseConnectorTextSummarization,
		strings.Join(ffParts, ","),
	)
}

var (
	composeCacheMu sync.RWMutex
	composeCache   = make(map[string][]string)
)

// ClearCache evicts all memoized beta composition results.
func ClearCache() {
	composeCacheMu.Lock()
	composeCache = make(map[string][]string)
	composeCacheMu.Unlock()
}

// Compose returns the complete list of beta headers for the given model and
// options. The result is memoized; call ClearCache to invalidate.
func Compose(model string, opts BetaOptions) []string {
	key := composerCacheKey(model, opts)

	composeCacheMu.RLock()
	if cached, ok := composeCache[key]; ok {
		composeCacheMu.RUnlock()
		return cached
	}
	composeCacheMu.RUnlock()

	result := compose(model, opts)

	composeCacheMu.Lock()
	composeCache[key] = result
	composeCacheMu.Unlock()

	return result
}

// compose implements the core beta header selection logic, mirroring
// src/utils/betas.ts::getAllModelBetas.
func compose(model string, opts BetaOptions) []string {
	var betas []string
	isHaiku := IsHaiku(model)
	provider := opts.Provider
	includeFirstPartyOnly := shouldIncludeFirstPartyOnlyBetas(provider, opts.DisableExperimentalBetas)

	// 1. General Claude Code beta (non-Haiku).
	if !isHaiku {
		betas = append(betas, ClaudeCodeBeta)

		// CLI-internal beta for ant users in CLI entrypoint.
		if opts.UserType == "ant" && opts.Entrypoint == "cli" {
			betas = append(betas, CLIInternalBeta)
		}
	}

	// 2. OAuth beta for Claude.ai subscribers.
	if opts.IsSubscriber {
		betas = append(betas, OAuthBeta)
	}

	// 3. 1M context window.
	if ModelSupports1M(model) {
		betas = append(betas, Context1MBeta)
	}

	// 4. Interleaved thinking (ISP).
	if !opts.DisableInterleavedThinking && ModelSupportsISP(model, provider) {
		betas = append(betas, InterleavedThinkingBeta)
	}

	// 5. Redact thinking (first-party-only, ISP supported, non-interactive,
	// showThinkingSummaries not explicitly enabled).
	// showThinkingSummaries is read from settings; default nil means "do not show"
	// and therefore redact-thinking is enabled. We approximate this with a
	// placeholder feature flag.
	if includeFirstPartyOnly &&
		ModelSupportsISP(model, provider) &&
		!opts.FeatureFlags["show_thinking_summaries"] {
		betas = append(betas, RedactThinkingBeta)
	}

	// 6. Connector text summarization (ant-only, feature-flag gated).
	// Empty UseConnectorTextSummarization means defer to feature flag.
	cts := strings.TrimSpace(opts.UseConnectorTextSummarization)
	ctsEnabled := opts.FeatureFlags["connector_text_summarization"]
	if cts != "" {
		ctsEnabled = isEnvTruthy(cts)
	}
	if opts.UserType == "ant" && includeFirstPartyOnly && !isEnvDefinedFalsy(cts) && ctsEnabled {
		betas = append(betas, SummarizeConnectorTextBeta)
	}

	// 7. Context management (tool clearing or thinking preservation).
	antOptedIntoToolClearing := opts.UseAPIContextManagement && opts.UserType == "ant"
	thinkingPreservation := ModelSupportsContextManagement(model, provider)
	if includeFirstPartyOnly && (antOptedIntoToolClearing || thinkingPreservation) {
		betas = append(betas, ContextManagementBeta)
	}

	// 8. Structured outputs (feature flag + model support).
	if includeFirstPartyOnly &&
		ModelSupportsStructuredOutputs(model, provider) &&
		opts.FeatureFlags["strict_tools"] {
		betas = append(betas, StructuredOutputsBeta)
	}

	// 9. Token efficient tools (ant-only, feature flag).
	if opts.UserType == "ant" &&
		includeFirstPartyOnly &&
		opts.FeatureFlags["token_efficient_tools"] {
		betas = append(betas, TokenEfficientToolsBeta)
	}

	// 10. Web search (Vertex Claude 4.0+ or Foundry).
	if ModelSupportsWebSearch(model, provider) {
		betas = append(betas, WebSearchBeta)
	}

	// 11. Prompt caching scope (first-party-only).
	if includeFirstPartyOnly {
		betas = append(betas, PromptCachingScopeBeta)
	}

	// 12. ANTHROPIC_BETAS environment override.
	if opts.AnthropicBetas != "" {
		for _, b := range strings.Split(opts.AnthropicBetas, ",") {
			b = strings.TrimSpace(b)
			if b != "" {
				betas = append(betas, b)
			}
		}
	}

	// 13. Agentic query override for Haiku models.
	if opts.IsAgenticQuery {
		if !contains(betas, ClaudeCodeBeta) {
			betas = append(betas, ClaudeCodeBeta)
		}
		if opts.UserType == "ant" && opts.Entrypoint == "cli" && !contains(betas, CLIInternalBeta) {
			betas = append(betas, CLIInternalBeta)
		}
	}

	return betas
}

// GetModelBetas returns the beta headers that should be sent via HTTP headers.
// For Bedrock, it filters out betas that must go through extraBodyParams.
func GetModelBetas(model string, opts BetaOptions) []string {
	all := Compose(model, opts)
	if opts.Provider == ProviderBedrock {
		var filtered []string
		for _, b := range all {
			if !BedrockExtraParamsHeaders[b] {
				filtered = append(filtered, b)
			}
		}
		return filtered
	}
	return all
}

// GetExtraBodyParamsBetas returns the beta headers that must be sent through
// extraBodyParams instead of HTTP headers. Only relevant for Bedrock.
func GetExtraBodyParamsBetas(model string, opts BetaOptions) []string {
	if opts.Provider != ProviderBedrock {
		return nil
	}
	all := Compose(model, opts)
	var extra []string
	for _, b := range all {
		if BedrockExtraParamsHeaders[b] {
			extra = append(extra, b)
		}
	}
	return extra
}

// MergeSDKBetas merges SDK-provided betas with the auto-detected model betas.
// SDK betas are pre-filtered by the caller; this function only deduplicates.
func MergeSDKBetas(modelBetas, sdkBetas []string) []string {
	if len(sdkBetas) == 0 {
		return modelBetas
	}
	seen := make(map[string]bool, len(modelBetas))
	for _, b := range modelBetas {
		seen[b] = true
	}
	result := append([]string(nil), modelBetas...)
	for _, b := range sdkBetas {
		if !seen[b] {
			seen[b] = true
			result = append(result, b)
		}
	}
	return result
}

// shouldIncludeFirstPartyOnlyBetas reports whether first-party-only experimental
// betas should be included.
func shouldIncludeFirstPartyOnlyBetas(provider ProviderType, disabled bool) bool {
	return (provider == ProviderFirstParty || provider == ProviderFoundry) && !disabled
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

