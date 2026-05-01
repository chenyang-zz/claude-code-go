package prompts

import "context"

// ConfigPromptSection provides usage guidance for the Config tool.
type ConfigPromptSection struct{}

// Name returns the section identifier.
func (s ConfigPromptSection) Name() string { return "config_prompt" }

// IsVolatile reports whether this section must be recomputed every turn.
func (s ConfigPromptSection) IsVolatile() bool { return false }

// Compute generates the Config tool usage guidance.
func (s ConfigPromptSection) Compute(ctx context.Context) (string, error) {
	return `# ConfigTool

Get or set Claude Code configuration settings.

View or change Claude Code settings. Use when the user requests configuration changes, asks about current settings, or when adjusting a setting would benefit them.

## Usage
- Get current value: Omit the "value" parameter
- Set new value: Include the "value" parameter

## Configurable settings list

The following settings are available for you to change:

### Global Settings (stored in ~/.claude.json)
- theme: "dark", "light" - The color theme for the UI
- editorMode: "default", "vim" - Keybindings mode for the editor
- verbose: true/false - Enable verbose output
- permissions.defaultMode: "plan", "automatic" - Default approval mode for tool calls
- model: Override the default model (sonnet, opus, haiku, best, or full model ID)

### Project Settings (stored in settings.json)
- additionalDirectories: List of extra directories to include in the project context
- hooks: Hook rules for automation
- mcpServers: MCP server configurations

## Examples
- Get theme: { "setting": "theme" }
- Set dark theme: { "setting": "theme", "value": "dark" }
- Enable vim mode: { "setting": "editorMode", "value": "vim" }
- Enable verbose: { "setting": "verbose", "value": true }
- Change model: { "setting": "model", "value": "opus" }
- Change permission mode: { "setting": "permissions.defaultMode", "value": "plan" }`, nil
}
