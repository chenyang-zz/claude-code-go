// Package betas provides the Anthropic beta header composition engine.
package betas

import (
	"os"
	"strings"
)

// ProviderType identifies which Anthropic API provider is in use.
type ProviderType string

const (
	// ProviderFirstParty is the official Anthropic API.
	ProviderFirstParty ProviderType = "firstParty"
	// ProviderVertex is Google Cloud Vertex AI.
	ProviderVertex ProviderType = "vertex"
	// ProviderBedrock is AWS Bedrock.
	ProviderBedrock ProviderType = "bedrock"
	// ProviderFoundry is Azure AI Foundry.
	ProviderFoundry ProviderType = "foundry"
)

// BetaOptions carries the inputs needed to compute beta headers.
type BetaOptions struct {
	// Provider is the current API provider.
	Provider ProviderType

	// UserType is "ant" for internal Anthropic users, empty otherwise.
	UserType string

	// Entrypoint is "cli" or "desktop". Only affects CLI-internal beta.
	Entrypoint string

	// DisableExperimentalBetas mirrors CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS.
	DisableExperimentalBetas bool

	// DisableInterleavedThinking mirrors DISABLE_INTERLEAVED_THINKING.
	DisableInterleavedThinking bool

	// UseAPIContextManagement mirrors USE_API_CONTEXT_MANAGEMENT.
	UseAPIContextManagement bool

	// UseConnectorTextSummarization mirrors USE_CONNECTOR_TEXT_SUMMARIZATION.
	// Empty means unset (defer to feature flag); truthy forces on; falsy forces off.
	UseConnectorTextSummarization string

	// AnthropicBetas mirrors ANTHROPIC_BETAS (comma-separated).
	AnthropicBetas string

	// SDKBetas carries betas provided by the SDK caller.
	// Only AllowedSDKBetas entries are permitted; the rest are filtered.
	SDKBetas []string

	// FeatureFlags holds placeholder feature flag values.
	// GrowthBook integration is not implemented; callers must populate
	// the flags they need before invoking Compose.
	FeatureFlags map[string]bool

	// IsSubscriber indicates whether the user is a Claude.ai subscriber.
	IsSubscriber bool

	// IsAgenticQuery, when true, forces claude-code and cli-internal
	// betas even for Haiku models.
	IsAgenticQuery bool
}

// ParseEnvOptions builds BetaOptions from environment variables.
func ParseEnvOptions() BetaOptions {
	return BetaOptions{
		UserType:                   os.Getenv("USER_TYPE"),
		Entrypoint:                 os.Getenv("CLAUDE_CODE_ENTRYPOINT"),
		DisableExperimentalBetas:   isEnvTruthy(os.Getenv("CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS")),
		DisableInterleavedThinking:    isEnvTruthy(os.Getenv("DISABLE_INTERLEAVED_THINKING")),
		UseAPIContextManagement:       isEnvTruthy(os.Getenv("USE_API_CONTEXT_MANAGEMENT")),
		UseConnectorTextSummarization: os.Getenv("USE_CONNECTOR_TEXT_SUMMARIZATION"),
		AnthropicBetas:                os.Getenv("ANTHROPIC_BETAS"),
	}
}

// isEnvTruthy returns true when the value is a truthy string
// such as "1", "true", or "yes".
func isEnvTruthy(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

// isEnvDefinedFalsy returns true only when the variable is explicitly
// set to a falsy value ("0", "false", "no", "off").
func isEnvDefinedFalsy(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return v == "0" || v == "false" || v == "no" || v == "off"
}
