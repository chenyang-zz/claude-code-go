package prompts

import "context"

// WorktreePromptSection provides usage guidance for the EnterWorktree and ExitWorktree tools.
type WorktreePromptSection struct{}

// Name returns the section identifier.
func (s WorktreePromptSection) Name() string { return "worktree_prompt" }

// IsVolatile reports whether this section must be recomputed every turn.
func (s WorktreePromptSection) IsVolatile() bool { return false }

// Compute generates the worktree tool usage guidance.
func (s WorktreePromptSection) Compute(ctx context.Context) (string, error) {
	return `# Worktree Tools

Use EnterWorktree ONLY when the user explicitly asks to work in a worktree. This tool creates an isolated git worktree and switches the current session into it.

## When to Use EnterWorktree

- The user explicitly says "worktree" (e.g., "start a worktree", "work in a worktree", "create a worktree", "use a worktree")

## When NOT to Use EnterWorktree

- The user asks to create a branch, switch branches, or work on a different branch — use git commands instead
- The user asks to fix a bug or work on a feature — use normal git workflow unless they specifically mention worktrees
- Never use this tool unless the user explicitly mentions "worktree"

## Requirements

- Must be in a git repository, OR have WorktreeCreate/WorktreeRemove hooks configured in settings.json
- Must not already be in a worktree

## EnterWorktree Behavior

- In a git repository: creates a new git worktree inside .claude/worktrees/ with a new branch based on HEAD
- Outside a git repository: delegates to WorktreeCreate/WorktreeRemove hooks for VCS-agnostic isolation
- Switches the session's working directory to the new worktree
- Use ExitWorktree to leave the worktree mid-session (keep or remove). On session exit, if still in the worktree, the user will be prompted to keep or remove it

## ExitWorktree Behavior

This tool ONLY operates on worktrees created by EnterWorktree in this session. It will NOT touch:
- Worktrees you created manually with git worktree add
- Worktrees from a previous session (even if created by EnterWorktree then)
- The directory you're in if EnterWorktree was never called

If called outside an EnterWorktree session, the tool is a no-op: it reports that no worktree session is active and takes no action. Filesystem state is unchanged.

## ExitWorktree Parameters

- action (required): "keep" or "remove"
  - "keep" — leave the worktree directory and branch intact on disk. Use this if the user wants to come back to the work later, or if there are changes to preserve.
  - "remove" — delete the worktree directory and its branch. Use this for a clean exit when the work is done or abandoned.
- discard_changes (optional, default false): only meaningful with action: "remove". If the worktree has uncommitted files or commits not on the original branch, the tool will REFUSE to remove it unless this is set to true. If the tool returns an error listing changes, confirm with the user before re-invoking with discard_changes: true.`, nil
}
