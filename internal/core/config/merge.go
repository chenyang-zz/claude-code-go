package config

func Merge(base, override Config) Config {
	if override.Model != "" {
		base.Model = override.Model
	}
	if override.Provider != "" {
		base.Provider = override.Provider
	}
	if override.ApprovalMode != "" {
		base.ApprovalMode = override.ApprovalMode
	}
	if override.SessionDBPath != "" {
		base.SessionDBPath = override.SessionDBPath
	}
	return base
}
