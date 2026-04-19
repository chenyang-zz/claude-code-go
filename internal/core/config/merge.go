package config

import "github.com/sheepzhao/claude-code-go/internal/core/hook"

func Merge(base, override Config) Config {
	if override.ProjectPath != "" {
		base.ProjectPath = override.ProjectPath
	}
	base.Env = mergeStringMap(base.Env, override.Env)
	if override.Model != "" {
		base.Model = override.Model
	}
	if override.HasEffortLevelSetting {
		base.EffortLevel = NormalizeEffortLevel(override.EffortLevel)
		base.HasEffortLevelSetting = true
	}
	if override.HasFastModeSetting {
		base.FastMode = override.FastMode
		base.HasFastModeSetting = true
	}
	if override.Theme != "" {
		base.Theme = NormalizeThemeSetting(override.Theme)
	}
	if override.EditorMode != "" {
		base.EditorMode = NormalizeEditorMode(override.EditorMode)
	}
	if override.Provider != "" {
		base.Provider = NormalizeProvider(override.Provider)
	}
	if override.APIKey != "" {
		base.APIKey = override.APIKey
	}
	if override.AuthToken != "" {
		base.AuthToken = override.AuthToken
	}
	if override.APIBaseURL != "" {
		base.APIBaseURL = override.APIBaseURL
	}
	if len(override.LoadedSettingSources) > 0 {
		base.LoadedSettingSources = append([]string(nil), override.LoadedSettingSources...)
	}
	base.PolicySettings = mergePolicySettingsConfig(base.PolicySettings, override.PolicySettings)
	if override.APIKeySource != "" {
		base.APIKeySource = override.APIKeySource
	}
	if override.AuthTokenSource != "" {
		base.AuthTokenSource = override.AuthTokenSource
	}
	base.OAuthAccount = mergeOAuthAccountConfig(base.OAuthAccount, override.OAuthAccount)
	if override.APIBaseURLSource != "" {
		base.APIBaseURLSource = override.APIBaseURLSource
	}
	if override.ProxyURL != "" {
		base.ProxyURL = override.ProxyURL
	}
	if override.ProxySource != "" {
		base.ProxySource = override.ProxySource
	}
	if override.AdditionalCACertsPath != "" {
		base.AdditionalCACertsPath = override.AdditionalCACertsPath
	}
	if override.AdditionalCACertsSource != "" {
		base.AdditionalCACertsSource = override.AdditionalCACertsSource
	}
	if override.MTLSClientCertPath != "" {
		base.MTLSClientCertPath = override.MTLSClientCertPath
	}
	if override.MTLSClientCertSource != "" {
		base.MTLSClientCertSource = override.MTLSClientCertSource
	}
	if override.MTLSClientKeyPath != "" {
		base.MTLSClientKeyPath = override.MTLSClientKeyPath
	}
	if override.MTLSClientKeySource != "" {
		base.MTLSClientKeySource = override.MTLSClientKeySource
	}
	if override.ApprovalMode != "" {
		base.ApprovalMode = override.ApprovalMode
	}
	if override.SessionDBPath != "" {
		base.SessionDBPath = override.SessionDBPath
	}
	base.Permissions = mergePermissionConfig(base.Permissions, override.Permissions)
	base.Hooks = hook.MergeHooksConfig(base.Hooks, override.Hooks)
	if override.DisableAllHooks {
		base.DisableAllHooks = true
	}
	return base
}

// mergeStringMap overlays override keys onto base while preserving untouched entries.
func mergeStringMap(base, override map[string]string) map[string]string {
	if len(base) == 0 && len(override) == 0 {
		return nil
	}

	merged := make(map[string]string, len(base)+len(override))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range override {
		merged[key] = value
	}
	return merged
}

// mergePermissionConfig overlays one minimal permission configuration onto another.
func mergePermissionConfig(base, override PermissionConfig) PermissionConfig {
	if override.DefaultMode != "" {
		base.DefaultMode = override.DefaultMode
	}
	if len(override.Allow) > 0 {
		base.Allow = append([]string(nil), override.Allow...)
	}
	if len(override.Deny) > 0 {
		base.Deny = append([]string(nil), override.Deny...)
	}
	if len(override.Ask) > 0 {
		base.Ask = append([]string(nil), override.Ask...)
	}
	if len(override.AdditionalDirectoryEntries) > 0 {
		base.AdditionalDirectoryEntries = cloneAdditionalDirectoryEntries(override.AdditionalDirectoryEntries)
		base.AdditionalDirectories = AdditionalDirectoryPaths(base.AdditionalDirectoryEntries)
	} else if len(override.AdditionalDirectories) > 0 {
		base.AdditionalDirectories = append([]string(nil), override.AdditionalDirectories...)
	}
	if override.DisableBypassPermissionsMode != "" {
		base.DisableBypassPermissionsMode = override.DisableBypassPermissionsMode
	}
	return base
}

// cloneAdditionalDirectoryEntries copies sourced directory entries so later merges cannot mutate prior snapshots.
func cloneAdditionalDirectoryEntries(entries []AdditionalDirectoryConfig) []AdditionalDirectoryConfig {
	if len(entries) == 0 {
		return nil
	}
	cloned := make([]AdditionalDirectoryConfig, len(entries))
	copy(cloned, entries)
	return cloned
}

// mergePolicySettingsConfig overlays managed-settings source metadata when a higher-priority layer provides it.
func mergePolicySettingsConfig(base, override PolicySettingsConfig) PolicySettingsConfig {
	if override.Origin != "" {
		base.Origin = override.Origin
		base.HasBaseFile = override.HasBaseFile
		base.HasDropIns = override.HasDropIns
	}
	return base
}

// mergeOAuthAccountConfig overlays the non-empty cached account metadata fields.
func mergeOAuthAccountConfig(base, override OAuthAccountConfig) OAuthAccountConfig {
	if override.AccountUUID != "" {
		base.AccountUUID = override.AccountUUID
	}
	if override.EmailAddress != "" {
		base.EmailAddress = override.EmailAddress
	}
	if override.OrganizationUUID != "" {
		base.OrganizationUUID = override.OrganizationUUID
	}
	if override.OrganizationName != "" {
		base.OrganizationName = override.OrganizationName
	}
	return base
}
