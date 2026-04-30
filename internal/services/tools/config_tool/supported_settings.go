package config_tool

// SettingConfig describes the metadata and constraints for a single supported setting.
type SettingConfig struct {
	// Scope determines which settings file the setting is read from and written to.
	Scope string
	// Type declares the expected value kind: "string", "boolean", or "number".
	Type string
	// Description provides a human-readable explanation shown to the model.
	Description string
	// Options, when non-empty, restricts the setting to a fixed set of string values.
	Options []string
}

// supportedSettings is the static registry of settings that ConfigTool can read and write.
// Only settings with verified write support in the Go runtime are included.
var supportedSettings = map[string]SettingConfig{
	"model": {
		Scope:       "user",
		Type:        "string",
		Description: "Override the default model. Use \"default\" to clear.",
		Options:     nil, // open-ended; dynamic model list not yet available
	},
	"theme": {
		Scope:       "user",
		Type:        "string",
		Description: "Color theme for the UI",
		Options:     []string{"light", "dark"},
	},
	"editorMode": {
		Scope:       "user",
		Type:        "string",
		Description: "Key binding mode for the editor",
		Options:     []string{"normal", "vim"},
	},
	"fastMode": {
		Scope:       "user",
		Type:        "boolean",
		Description: "Enable fast mode for shorter responses",
	},
	"effortLevel": {
		Scope:       "user",
		Type:        "string",
		Description: "Effort level for model reasoning",
		Options:     []string{"low", "medium", "high"},
	},
	"permissions.defaultMode": {
		Scope:       "project",
		Type:        "string",
		Description: "Default permission mode for tool usage",
		Options:     []string{"default", "plan", "acceptEdits", "dontAsk", "bypassPermissions"},
	},
}

// getSupportedSetting returns the SettingConfig for a key if it is in the registry.
func getSupportedSetting(key string) (SettingConfig, bool) {
	cfg, ok := supportedSettings[key]
	return cfg, ok
}
