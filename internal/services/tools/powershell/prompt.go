package powershell

import (
	"fmt"
)

// DefaultTimeoutMilliseconds is the default PowerShell command timeout.
const DefaultTimeoutMilliseconds = 120000

// MaxTimeoutMilliseconds is the maximum allowed PowerShell command timeout.
const MaxTimeoutMilliseconds = 600000

// defaultTimeout returns the effective default timeout, overridable via env.
func defaultTimeout() int {
	return DefaultTimeoutMilliseconds
}

// maxTimeout returns the effective maximum timeout.
func maxTimeout() int {
	return MaxTimeoutMilliseconds
}

// Description returns the tool description shown in provider tool schemas.
func Description() string {
	return fmt.Sprintf(`Executes a given PowerShell command and returns its output.

The working directory persists between commands, but shell state (variables, functions) does not. The shell environment is initialized from the user's profile.

IMPORTANT: This tool is for terminal operations via PowerShell: git, npm, docker, and PS cmdlets. DO NOT use it for file operations (reading, writing, editing, searching, finding files) - use the specialized tools for this instead.

PowerShell Syntax Notes:
- Variables use $ prefix: $myVar = "value"
- Escape character is backtick (` + "``" + `), not backslash
- Use Verb-Noun cmdlet naming: Get-ChildItem, Set-Location, New-Item, Remove-Item
- Common aliases: ls (Get-ChildItem), cd (Set-Location), cat (Get-Content), rm (Remove-Item)
- Pipe operator | works similarly to bash but passes objects, not text
- Use Select-Object, Where-Object, ForEach-Object for filtering and transformation
- Registry access uses PSDrive prefixes: HKLM:\SOFTWARE\..., HKCU:\...
- Environment variables: read with $env:NAME, set with $env:NAME = "value"
- Call native exe with spaces in path: & "C:\Program Files\App\app.exe" arg1 arg2

Interactive and blocking commands (will hang):
- NEVER use Read-Host, Get-Credential, Out-GridView, $Host.UI.PromptForChoice, or pause
- Destructive cmdlets (Remove-Item, Stop-Process, Clear-Content, etc.) may prompt for confirmation. Add -Confirm:$false when you intend the action to proceed.
- Never use git rebase -i, git add -i, or other commands that open an interactive editor

Instructions:
- If the command will create new directories or files, first use Get-ChildItem (or ls) to verify the parent directory exists.
- Always quote file paths that contain spaces with double quotes.
- You may specify an optional timeout in milliseconds (up to %dms / %d minutes). By default, your command will timeout after %dms (%d minutes).
- You can use the run_in_background parameter to run the command in the background.
- Avoid using PowerShell to run commands that have dedicated tools, unless explicitly instructed.
- Do NOT prefix commands with cd or Set-Location — the working directory is already set correctly.
- For git commands, prefer to create a new commit rather than amending an existing commit. Never skip hooks (--no-verify) unless explicitly asked.
- When issuing multiple commands:
  - If independent, make multiple PowerShell tool calls in a single message.
  - If dependent on each other, chain them in a single call.
  - DO NOT use newlines to separate commands (newlines are ok in quoted strings and here-strings).

Passing multiline strings (commit messages, file content) to native executables:
- Use a single-quoted here-string so PowerShell does not expand $ or backticks inside:
  git commit -m @'
  Commit message here.
  '@

Avoid unnecessary Start-Sleep commands:
- Do not sleep between commands that can run immediately.
- If your command is long running, use run_in_background. No sleep needed.
- If you must sleep, keep the duration short (1-5 seconds) to avoid blocking the user.`, maxTimeout(), maxTimeout()/60000, defaultTimeout(), defaultTimeout()/60000)
}
