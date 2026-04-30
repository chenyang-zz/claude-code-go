package bundled

import "github.com/sheepzhao/claude-code-go/internal/services/tools/skill"

const keybindingsPrompt = `# Keybindings Skill

Create or modify ` + "`~/.claude/keybindings.json`" + ` to customize keyboard shortcuts.

## CRITICAL: Read Before Write

**Always read ` + "`~/.claude/keybindings.json`" + ` first** (it may not exist yet). Merge changes with existing bindings — never replace the entire file.

- Use **Edit** tool for modifications to existing files
- Use **Write** tool only if the file does not exist yet

## File Format
` + "```json" + `
{
  "$schema": "https://www.schemastore.org/claude-code-keybindings.json",
  "$docs": "https://code.claude.com/docs/en/keybindings",
  "bindings": [
    {
      "context": "Chat",
      "bindings": {
        "ctrl+e": "chat:externalEditor"
      }
    }
  ]
}
` + "```" + `

Always include the ` + "`$schema`" + ` and ` + "`$docs`" + ` fields.

## Keystroke Syntax

**Modifiers** (combine with ` + "`+`" + `):
- ` + "`ctrl`" + ` (alias: ` + "`control`" + `)
- ` + "`alt`" + ` (aliases: ` + "`opt`" + `, ` + "`option`" + `)
- ` + "`shift`" + `
- ` + "`meta`" + ` (aliases: ` + "`cmd`" + `, ` + "`command`" + `)

**Special keys**: ` + "`escape`/`esc`" + `, ` + "`enter`/`return`" + `, ` + "`tab`" + `, ` + "`space`" + `, ` + "`backspace`" + `, ` + "`delete`" + `, ` + "`up`" + `, ` + "`down`" + `, ` + "`left`" + `, ` + "`right`" + `

**Chords**: Space-separated keystrokes, e.g. ` + "`ctrl+k ctrl+s`" + ` (1-second timeout between keystrokes)

**Examples**: ` + "`ctrl+shift+p`" + `, ` + "`alt+enter`" + `, ` + "`ctrl+k ctrl+n`" + `

## Unbinding Default Shortcuts

Set a key to ` + "`null`" + ` to remove its default binding:
` + "```json" + `
{ "context": "Chat", "bindings": { "ctrl+s": null } }
` + "```" + `

## How User Bindings Interact with Defaults

- User bindings are **additive** — they are appended after the default bindings
- To **move** a binding to a different key: unbind the old key (null) AND add the new binding
- A context only needs to appear in the user's file if they want to change something in that context

## Common Patterns

### Rebind a key
` + "```json" + `
{ "context": "Chat", "bindings": { "ctrl+g": null, "ctrl+e": "chat:externalEditor" } }
` + "```" + `

### Add a chord binding
` + "```json" + `
{ "context": "Global", "bindings": { "ctrl+k ctrl+t": "app:toggleTodos" } }
` + "```" + `

## Available Contexts

| Context | Description |
|---------|-------------|
| Global | Global shortcuts active everywhere |
| Chat | Chat input and message area |
| Autocomplete | Autocomplete suggestions |
| Confirmation | Permission/confirmation dialogs |
| Tabs | Tab navigation |
| Transcript | Transcript view |
| HistorySearch | History search dialog |
| Task | Task list |
| ThemePicker | Theme picker |
| Help | Help dialog |
| Attachments | File attachments |
| Footer | Footer area |
| MessageSelector | Message selector |
| DiffDialog | Diff dialog |
| ModelPicker | Model picker |
| Select | Selection mode |

## Available Actions

| Action | Default Key(s) | Context |
|--------|---------------|---------|
| app:help | ctrl+o | Global |
| app:toggleTodos | ctrl+k ctrl+t | Global |
| app:exit | ctrl+d | Global |
| chat:submit | ctrl+enter | Chat |
| chat:externalEditor | ctrl+g | Chat |
| chat:newline | shift+enter | Chat |
| autocomplete:accept | tab | Autocomplete |
| confirm:accept | enter | Confirmation |
| confirm:cancel | escape | Confirmation |

## Behavioral Rules

1. Only include contexts the user wants to change (minimal overrides)
2. Validate that actions and contexts are from the known lists above
3. Warn the user if they choose a key that conflicts with reserved shortcuts or common tools like tmux (ctrl+b) and screen (ctrl+a)
4. When adding a new binding for an existing action, the new binding is additive
5. To fully replace a default binding, unbind the old key AND add the new one

## Validation with /doctor

The ` + "`/doctor`" + ` command includes a "Keybinding Configuration Issues" section.

### Common Issues

| Issue | Cause | Fix |
|-------|-------|-----|
| keybindings.json must have a "bindings" array | Missing wrapper | Wrap bindings in { "bindings": [...] } |
| "bindings" must be an array | bindings is not an array | Set "bindings" to an array |
| Unknown context "X" | Typo in context name | Use exact context names |
| Duplicate key "X" | Same key defined twice | Remove duplicate |
| Could not parse keystroke "X" | Invalid key syntax | Check syntax |

## Reserved Shortcuts

### Non-rebindable
- ctrl+c — Terminal interrupt (SIGINT)
- ctrl+z — Terminal suspend (SIGTSTP)
- ctrl+\\ — Terminal quit (SIGQUIT)

### Terminal reserved (may conflict)
- ctrl+a / ctrl+e — Line navigation (readline)
- ctrl+k / ctrl+u / ctrl+w — Line editing (readline)
- ctrl+l — Clear screen

### macOS reserved
- cmd+h — Hide app
- cmd+q — Quit app
- cmd+m — Minimize
- cmd+w — Close window
- cmd+tab — App switcher`

func registerKeybindingsSkill() {
	skill.RegisterBundledSkill(skill.BundledSkillDefinition{
		Name:         "keybindings-help",
		Description:  "Use when the user wants to customize keyboard shortcuts, rebind keys, add chord bindings, or modify ~/.claude/keybindings.json. Examples: \"rebind ctrl+s\", \"add a chord shortcut\", \"change the submit key\", \"customize keybindings\".",
		AllowedTools: []string{"Read"},
		UserInvocable: false,
		GetPromptForCommand: func(args string) (string, error) {
			prompt := keybindingsPrompt
			if args != "" {
				prompt += "\n\n## User Request\n\n" + args
			}
			return prompt, nil
		},
	})
}
