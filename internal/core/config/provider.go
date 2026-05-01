package config

import "strings"

const (
	// ProviderAnthropic identifies the default Anthropic runtime provider.
	ProviderAnthropic = "anthropic"
	// ProviderOpenAICompatible identifies the generic OpenAI-compatible runtime provider.
	ProviderOpenAICompatible = "openai-compatible"
	// ProviderGLM identifies the GLM-compatible runtime provider alias.
	ProviderGLM = "glm"
	// ProviderVertex identifies the Google Cloud Vertex AI runtime provider.
	ProviderVertex = "vertex"
)

// NormalizeProvider folds provider aliases into the stable runtime provider identifiers.
func NormalizeProvider(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "", ProviderAnthropic:
		return ProviderAnthropic
	case "openai", "openai_compatible", ProviderOpenAICompatible:
		return ProviderOpenAICompatible
	case "zhipu", "zhipuai", ProviderGLM:
		return ProviderGLM
	case ProviderVertex:
		return ProviderVertex
	default:
		return strings.TrimSpace(strings.ToLower(value))
	}
}
