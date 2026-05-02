// Package betas provides the Anthropic beta header composition engine.
package betas

import (
	"strings"
)

// CanonicalName resolves a model identifier to its canonical short name.
// It mirrors the logic in src/utils/model/model.ts::firstPartyNameToCanonical.
func CanonicalName(model string) string {
	name := strings.ToLower(model)

	// Claude 4+ models — check more specific versions first.
	if strings.Contains(name, "claude-opus-4-6") {
		return "claude-opus-4-6"
	}
	if strings.Contains(name, "claude-opus-4-5") {
		return "claude-opus-4-5"
	}
	if strings.Contains(name, "claude-opus-4-1") {
		return "claude-opus-4-1"
	}
	if strings.Contains(name, "claude-opus-4") {
		return "claude-opus-4"
	}
	if strings.Contains(name, "claude-sonnet-4-6") {
		return "claude-sonnet-4-6"
	}
	if strings.Contains(name, "claude-sonnet-4-5") {
		return "claude-sonnet-4-5"
	}
	if strings.Contains(name, "claude-sonnet-4") {
		return "claude-sonnet-4"
	}
	if strings.Contains(name, "claude-haiku-4-5") {
		return "claude-haiku-4-5"
	}

	// Claude 3.x models.
	if strings.Contains(name, "claude-3-7-sonnet") {
		return "claude-3-7-sonnet"
	}
	if strings.Contains(name, "claude-3-5-sonnet") {
		return "claude-3-5-sonnet"
	}
	if strings.Contains(name, "claude-3-5-haiku") {
		return "claude-3-5-haiku"
	}
	if strings.Contains(name, "claude-3-opus") {
		return "claude-3-opus"
	}
	if strings.Contains(name, "claude-3-sonnet") {
		return "claude-3-sonnet"
	}
	if strings.Contains(name, "claude-3-haiku") {
		return "claude-3-haiku"
	}

	// Fallback: extract the first claude- pattern.
	idx := strings.Index(name, "claude-")
	if idx >= 0 {
		end := idx + 7
		for end < len(name) {
			c := name[end]
			if c == '-' || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
				end++
				continue
			}
			break
		}
		if end > idx+7 {
			return name[idx:end]
		}
	}

	return model
}

// IsHaiku returns true when the model is a Haiku variant.
func IsHaiku(model string) bool {
	return strings.Contains(strings.ToLower(model), "haiku")
}

// ModelSupports1M reports whether the model supports the 1M context window.
func ModelSupports1M(model string) bool {
	c := CanonicalName(model)
	return strings.Contains(c, "claude-sonnet-4") || strings.Contains(c, "opus-4-6")
}

// ModelSupportsISP reports whether the model supports interleaved thinking.
func ModelSupportsISP(model string, provider ProviderType) bool {
	if provider == ProviderFoundry {
		return true
	}
	c := CanonicalName(model)
	if provider == ProviderFirstParty {
		return !strings.Contains(c, "claude-3-")
	}
	return strings.Contains(c, "claude-opus-4") || strings.Contains(c, "claude-sonnet-4")
}

// ModelSupportsContextManagement reports whether the model supports
// context-management beta (tool clearing / thinking preservation).
func ModelSupportsContextManagement(model string, provider ProviderType) bool {
	if provider == ProviderFoundry {
		return true
	}
	c := CanonicalName(model)
	if provider == ProviderFirstParty {
		return !strings.Contains(c, "claude-3-")
	}
	return strings.Contains(c, "claude-opus-4") ||
		strings.Contains(c, "claude-sonnet-4") ||
		strings.Contains(c, "claude-haiku-4")
}

// ModelSupportsStructuredOutputs reports whether the model supports
// structured outputs (strict tools). Only first-party and Foundry.
func ModelSupportsStructuredOutputs(model string, provider ProviderType) bool {
	if provider != ProviderFirstParty && provider != ProviderFoundry {
		return false
	}
	c := CanonicalName(model)
	return strings.Contains(c, "claude-sonnet-4-6") ||
		strings.Contains(c, "claude-sonnet-4-5") ||
		strings.Contains(c, "claude-opus-4-1") ||
		strings.Contains(c, "claude-opus-4-5") ||
		strings.Contains(c, "claude-opus-4-6") ||
		strings.Contains(c, "claude-haiku-4-5")
}

// ModelSupportsWebSearch reports whether the model supports web search.
// Vertex: Claude 4.0+ only. Foundry: always enabled.
func ModelSupportsWebSearch(model string, provider ProviderType) bool {
	if provider == ProviderFoundry {
		return true
	}
	if provider != ProviderVertex {
		return false
	}
	c := CanonicalName(model)
	return strings.Contains(c, "claude-opus-4") ||
		strings.Contains(c, "claude-sonnet-4") ||
		strings.Contains(c, "claude-haiku-4")
}

// GetToolSearchBetaHeader returns the correct tool search beta header
// for the given provider.
func GetToolSearchBetaHeader(provider ProviderType) string {
	if provider == ProviderVertex || provider == ProviderBedrock {
		return ToolSearch3PBeta
	}
	return ToolSearch1PBeta
}
