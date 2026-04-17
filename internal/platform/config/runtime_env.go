package config

import "strings"

// runtimeSafeEnvVars enumerates the settings-sourced environment variables that may be applied from untrusted project/local settings.
var runtimeSafeEnvVars = map[string]struct{}{
	"ANTHROPIC_CUSTOM_HEADERS":                              {},
	"ANTHROPIC_CUSTOM_MODEL_OPTION":                         {},
	"ANTHROPIC_CUSTOM_MODEL_OPTION_DESCRIPTION":             {},
	"ANTHROPIC_CUSTOM_MODEL_OPTION_NAME":                    {},
	"ANTHROPIC_DEFAULT_HAIKU_MODEL":                         {},
	"ANTHROPIC_DEFAULT_HAIKU_MODEL_DESCRIPTION":             {},
	"ANTHROPIC_DEFAULT_HAIKU_MODEL_NAME":                    {},
	"ANTHROPIC_DEFAULT_HAIKU_MODEL_SUPPORTED_CAPABILITIES":  {},
	"ANTHROPIC_DEFAULT_OPUS_MODEL":                          {},
	"ANTHROPIC_DEFAULT_OPUS_MODEL_DESCRIPTION":              {},
	"ANTHROPIC_DEFAULT_OPUS_MODEL_NAME":                     {},
	"ANTHROPIC_DEFAULT_OPUS_MODEL_SUPPORTED_CAPABILITIES":   {},
	"ANTHROPIC_DEFAULT_SONNET_MODEL":                        {},
	"ANTHROPIC_DEFAULT_SONNET_MODEL_DESCRIPTION":            {},
	"ANTHROPIC_DEFAULT_SONNET_MODEL_NAME":                   {},
	"ANTHROPIC_DEFAULT_SONNET_MODEL_SUPPORTED_CAPABILITIES": {},
	"ANTHROPIC_FOUNDRY_API_KEY":                             {},
	"ANTHROPIC_MODEL":                                       {},
	"ANTHROPIC_SMALL_FAST_MODEL":                            {},
	"ANTHROPIC_SMALL_FAST_MODEL_AWS_REGION":                 {},
	"AWS_DEFAULT_REGION":                                    {},
	"AWS_PROFILE":                                           {},
	"AWS_REGION":                                            {},
	"BASH_DEFAULT_TIMEOUT_MS":                               {},
	"BASH_MAX_OUTPUT_LENGTH":                                {},
	"BASH_MAX_TIMEOUT_MS":                                   {},
	"CLAUDE_BASH_MAINTAIN_PROJECT_WORKING_DIR":              {},
	"CLAUDE_CODE_API_KEY_HELPER_TTL_MS":                     {},
	"CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS":                {},
	"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC":              {},
	"CLAUDE_CODE_DISABLE_TERMINAL_TITLE":                    {},
	"CLAUDE_CODE_ENABLE_TELEMETRY":                          {},
	"CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS":                  {},
	"CLAUDE_CODE_IDE_SKIP_AUTO_INSTALL":                     {},
	"CLAUDE_CODE_MAX_OUTPUT_TOKENS":                         {},
	"CLAUDE_CODE_SKIP_BEDROCK_AUTH":                         {},
	"CLAUDE_CODE_SKIP_FOUNDRY_AUTH":                         {},
	"CLAUDE_CODE_SKIP_VERTEX_AUTH":                          {},
	"CLAUDE_CODE_SUBAGENT_MODEL":                            {},
	"CLAUDE_CODE_USE_BEDROCK":                               {},
	"CLAUDE_CODE_USE_FOUNDRY":                               {},
	"CLAUDE_CODE_USE_VERTEX":                                {},
	"DISABLE_AUTOUPDATER":                                   {},
	"DISABLE_BUG_COMMAND":                                   {},
	"DISABLE_COST_WARNINGS":                                 {},
	"DISABLE_ERROR_REPORTING":                               {},
	"DISABLE_FEEDBACK_COMMAND":                              {},
	"DISABLE_TELEMETRY":                                     {},
	"ENABLE_TOOL_SEARCH":                                    {},
	"MAX_MCP_OUTPUT_TOKENS":                                 {},
	"MAX_THINKING_TOKENS":                                   {},
	"MCP_TIMEOUT":                                           {},
	"MCP_TOOL_TIMEOUT":                                      {},
	"OTEL_EXPORTER_OTLP_HEADERS":                            {},
	"OTEL_EXPORTER_OTLP_LOGS_HEADERS":                       {},
	"OTEL_EXPORTER_OTLP_LOGS_PROTOCOL":                      {},
	"OTEL_EXPORTER_OTLP_METRICS_CLIENT_CERTIFICATE":         {},
	"OTEL_EXPORTER_OTLP_METRICS_CLIENT_KEY":                 {},
	"OTEL_EXPORTER_OTLP_METRICS_HEADERS":                    {},
	"OTEL_EXPORTER_OTLP_METRICS_PROTOCOL":                   {},
	"OTEL_EXPORTER_OTLP_PROTOCOL":                           {},
	"OTEL_EXPORTER_OTLP_TRACES_HEADERS":                     {},
	"OTEL_LOG_TOOL_DETAILS":                                 {},
	"OTEL_LOG_USER_PROMPTS":                                 {},
	"OTEL_LOGS_EXPORT_INTERVAL":                             {},
	"OTEL_LOGS_EXPORTER":                                    {},
	"OTEL_METRIC_EXPORT_INTERVAL":                           {},
	"OTEL_METRICS_EXPORTER":                                 {},
	"OTEL_METRICS_INCLUDE_ACCOUNT_UUID":                     {},
	"OTEL_METRICS_INCLUDE_SESSION_ID":                       {},
	"OTEL_METRICS_INCLUDE_VERSION":                          {},
	"OTEL_RESOURCE_ATTRIBUTES":                              {},
	"USE_BUILTIN_RIPGREP":                                   {},
	"VERTEX_REGION_CLAUDE_3_5_HAIKU":                        {},
	"VERTEX_REGION_CLAUDE_3_5_SONNET":                       {},
	"VERTEX_REGION_CLAUDE_3_7_SONNET":                       {},
	"VERTEX_REGION_CLAUDE_4_0_OPUS":                         {},
	"VERTEX_REGION_CLAUDE_4_0_SONNET":                       {},
	"VERTEX_REGION_CLAUDE_4_1_OPUS":                         {},
	"VERTEX_REGION_CLAUDE_4_5_SONNET":                       {},
	"VERTEX_REGION_CLAUDE_4_6_SONNET":                       {},
	"VERTEX_REGION_CLAUDE_HAIKU_4_5":                        {},
}

// runtimeProviderManagedEnvVars enumerates provider-routing environment variables that host-managed provider mode must shield from settings overrides.
var runtimeProviderManagedEnvVars = map[string]struct{}{
	"ANTHROPIC_API_KEY":                     {},
	"ANTHROPIC_AUTH_TOKEN":                  {},
	"ANTHROPIC_BASE_URL":                    {},
	"ANTHROPIC_BEDROCK_BASE_URL":            {},
	"ANTHROPIC_DEFAULT_HAIKU_MODEL":         {},
	"ANTHROPIC_DEFAULT_OPUS_MODEL":          {},
	"ANTHROPIC_DEFAULT_SONNET_MODEL":        {},
	"ANTHROPIC_FOUNDRY_API_KEY":             {},
	"ANTHROPIC_FOUNDRY_BASE_URL":            {},
	"ANTHROPIC_FOUNDRY_RESOURCE":            {},
	"ANTHROPIC_MODEL":                       {},
	"ANTHROPIC_SMALL_FAST_MODEL":            {},
	"ANTHROPIC_SMALL_FAST_MODEL_AWS_REGION": {},
	"ANTHROPIC_VERTEX_BASE_URL":             {},
	"ANTHROPIC_VERTEX_PROJECT_ID":           {},
	"AWS_BEARER_TOKEN_BEDROCK":              {},
	"CLAUDE_CODE_API_BASE_URL":              {},
	"CLAUDE_CODE_API_KEY":                   {},
	"CLAUDE_CODE_AUTH_TOKEN":                {},
	"CLAUDE_CODE_MODEL":                     {},
	"CLAUDE_CODE_OAUTH_TOKEN":               {},
	"CLAUDE_CODE_PROVIDER":                  {},
	"CLAUDE_CODE_PROVIDER_MANAGED_BY_HOST":  {},
	"CLAUDE_CODE_SKIP_BEDROCK_AUTH":         {},
	"CLAUDE_CODE_SKIP_FOUNDRY_AUTH":         {},
	"CLAUDE_CODE_SKIP_VERTEX_AUTH":          {},
	"CLAUDE_CODE_SUBAGENT_MODEL":            {},
	"CLAUDE_CODE_USE_BEDROCK":               {},
	"CLAUDE_CODE_USE_FOUNDRY":               {},
	"CLAUDE_CODE_USE_VERTEX":                {},
	"CLOUD_ML_REGION":                       {},
	"GLM_API_KEY":                           {},
	"GLM_BASE_URL":                          {},
	"OPENAI_API_KEY":                        {},
	"OPENAI_BASE_URL":                       {},
	"ZHIPUAI_API_KEY":                       {},
	"ZHIPUAI_BASE_URL":                      {},
}

// runtimeProviderManagedPrefixes enumerates prefix-based routing variables that should also be stripped in host-managed provider mode.
var runtimeProviderManagedPrefixes = []string{
	"VERTEX_REGION_CLAUDE_",
}

// buildRuntimeSettingsEnv composes the settings-sourced environment visible to the Go runtime.
func buildRuntimeSettingsEnv(sourceEnvs map[SettingSource]map[string]string, hostManagedProvider bool) map[string]string {
	merged := map[string]string{}
	for _, source := range []SettingSource{
		SettingSourceUserSettings,
		SettingSourceFlagSettings,
	} {
		mergeRuntimeEnvLayer(merged, sourceEnvs[source], hostManagedProvider, false)
	}
	for _, source := range []SettingSource{
		SettingSourceProjectSettings,
		SettingSourceLocalSettings,
	} {
		mergeRuntimeEnvLayer(merged, sourceEnvs[source], hostManagedProvider, true)
	}
	if len(merged) == 0 {
		return nil
	}
	return merged
}

// mergeRuntimeEnvLayer applies one source env layer into the merged runtime env, optionally restricting untrusted keys to the safe allowlist.
func mergeRuntimeEnvLayer(dst map[string]string, layer map[string]string, hostManagedProvider bool, safeOnly bool) {
	for key, value := range layer {
		if hostManagedProvider && isProviderManagedRuntimeEnvVar(key) {
			continue
		}
		if safeOnly && !isSafeRuntimeEnvVar(key) {
			continue
		}
		dst[key] = value
	}
}

// isSafeRuntimeEnvVar reports whether one settings-sourced environment variable may be applied from project/local settings.
func isSafeRuntimeEnvVar(key string) bool {
	_, ok := runtimeSafeEnvVars[strings.ToUpper(strings.TrimSpace(key))]
	return ok
}

// isProviderManagedRuntimeEnvVar reports whether host-managed provider mode must reject a settings-sourced environment variable.
func isProviderManagedRuntimeEnvVar(key string) bool {
	upper := strings.ToUpper(strings.TrimSpace(key))
	if _, ok := runtimeProviderManagedEnvVars[upper]; ok {
		return true
	}
	for _, prefix := range runtimeProviderManagedPrefixes {
		if strings.HasPrefix(upper, prefix) {
			return true
		}
	}
	return false
}
