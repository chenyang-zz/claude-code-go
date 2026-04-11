package config

func Merge(base, override Config) Config {
	if override.ProjectPath != "" {
		base.ProjectPath = override.ProjectPath
	}
	if override.Model != "" {
		base.Model = override.Model
	}
	if override.EditorMode != "" {
		base.EditorMode = NormalizeEditorMode(override.EditorMode)
	}
	if override.Provider != "" {
		base.Provider = override.Provider
	}
	if override.APIKey != "" {
		base.APIKey = override.APIKey
	}
	if override.APIBaseURL != "" {
		base.APIBaseURL = override.APIBaseURL
	}
	if override.ApprovalMode != "" {
		base.ApprovalMode = override.ApprovalMode
	}
	if override.SessionDBPath != "" {
		base.SessionDBPath = override.SessionDBPath
	}
	base.Permissions = mergePermissionConfig(base.Permissions, override.Permissions)
	return base
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
	if len(override.AdditionalDirectories) > 0 {
		base.AdditionalDirectories = append([]string(nil), override.AdditionalDirectories...)
	}
	if override.DisableBypassPermissionsMode != "" {
		base.DisableBypassPermissionsMode = override.DisableBypassPermissionsMode
	}
	return base
}
