package exit

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/sheepzhao/claude-code-go/internal/core/hook"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	worktreeshared "github.com/sheepzhao/claude-code-go/internal/services/tools/worktree/shared"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// Name is the stable registry identifier for the ExitWorktreeTool.
	Name = "ExitWorktree"
)

// HookDispatcher dispatches hook events for worktree lifecycle hooks.
type HookDispatcher interface {
	// RunHooks executes hooks for the given event and returns results.
	// Returns nil when no hooks are configured.
	RunHooks(ctx context.Context, event hook.HookEvent, input any, cwd string) []hook.HookResult
}

// Tool implements the ExitWorktreeTool for exiting and optionally removing git worktrees.
type Tool struct {
	manager         *worktreeshared.Manager
	hooks           HookDispatcher
	hookCfg         hook.HooksConfig
	disableAllHooks bool
}

// Input is the typed request payload for ExitWorktreeTool.
type Input struct {
	Action         string `json:"action"`
	DiscardChanges *bool  `json:"discard_changes,omitempty"`
}

// Output is the structured result returned after exiting a worktree.
type Output struct {
	Action         string `json:"action"`
	OriginalCwd    string `json:"originalCwd"`
	WorktreePath   string `json:"worktreePath"`
	WorktreeBranch string `json:"worktreeBranch,omitempty"`
	Message        string `json:"message"`
}

// NewTool constructs an ExitWorktreeTool with a shared worktree manager.
func NewTool(manager *worktreeshared.Manager) *Tool {
	return &Tool{manager: manager}
}

// NewToolWithHooks constructs an ExitWorktreeTool with hook dispatch capability.
func NewToolWithHooks(manager *worktreeshared.Manager, dispatcher HookDispatcher, hookCfg hook.HooksConfig, disableAllHooks bool) *Tool {
	return &Tool{manager: manager, hooks: dispatcher, hookCfg: hookCfg, disableAllHooks: disableAllHooks}
}

// Name returns the stable tool identifier.
func (t *Tool) Name() string {
	return Name
}

// Description returns a short human-readable summary for the tool.
func (t *Tool) Description() string {
	return "Exits a worktree session created by EnterWorktree and restores the original working directory. Use this tool ONLY when the user explicitly asks to exit a worktree. Supports \"keep\" (leave worktree on disk) and \"remove\" (delete worktree and branch) actions."
}

// InputSchema returns the declared input contract.
func (t *Tool) InputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"action": {
				Type:        coretool.ValueKindString,
				Description: "\"keep\" leaves the worktree and branch on disk; \"remove\" deletes both.",
				Required:    true,
			},
			"discard_changes": {
				Type:        coretool.ValueKindBoolean,
				Description: "Required true when action is \"remove\" and the worktree has uncommitted files or unmerged commits. The tool will refuse and list them otherwise.",
			},
		},
	}
}

// IsReadOnly reports that ExitWorktreeTool may remove a worktree and is not read-only.
func (t *Tool) IsReadOnly() bool {
	return false
}

// IsConcurrencySafe reports that worktree removal cannot run concurrently.
func (t *Tool) IsConcurrencySafe() bool {
	return false
}

// RequiresUserInteraction indicates the tool requires user approval before execution.
func (t *Tool) RequiresUserInteraction() bool {
	return true
}

// Invoke exits a worktree session by either keeping or removing the worktree.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{}, fmt.Errorf("exit worktree tool: nil receiver")
	}
	if t.manager == nil {
		return coretool.Result{}, fmt.Errorf("exit worktree tool: worktree manager is not configured")
	}

	input, err := coretool.DecodeInput[Input](t.InputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	if input.Action != "keep" && input.Action != "remove" {
		return coretool.Result{
			Error: fmt.Sprintf("action must be \"keep\" or \"remove\", got %q", input.Action),
		}, nil
	}

	// For the minimal implementation without session state, we operate on the
	// working directory as the potential worktree path. The session CWD switching
	// is deferred to a later batch.
	worktreePath := call.Context.WorkingDir

	if input.Action == "remove" {
		force := input.DiscardChanges != nil && *input.DiscardChanges

		if !force {
			changes := t.manager.CountWorktreeChanges(worktreePath)
			if changes != nil && changes.ChangedFiles > 0 {
				return coretool.Result{
					Error: fmt.Sprintf(
						"Worktree has %d uncommitted files. Removing will discard this work permanently. Confirm with the user, then re-invoke with discard_changes: true — or use action: \"keep\" to preserve the worktree.",
						changes.ChangedFiles,
					),
				}, nil
			}
		}

		if err := t.manager.RemoveWorktree(worktreePath, force); err != nil {
			return coretool.Result{Error: fmt.Sprintf("exit worktree tool: %v", err)}, nil
		}

		// Fire WorktreeRemove hook after successful removal.
		// This is a non-blocking notification hook (matching TS behaviour where
		// failed hooks only log errors and do not prevent cleanup).
		t.fireWorktreeRemoveHook(ctx, worktreePath)

		logger.DebugCF("exit_worktree_tool", "worktree removed", map[string]any{
			"worktree_path": worktreePath,
			"force":         force,
		})

		return coretool.Result{
			Output: fmt.Sprintf("Exited and removed worktree at %s", worktreePath),
			Meta: map[string]any{
				"data": Output{
					Action:       "remove",
					OriginalCwd:  call.Context.WorkingDir,
					WorktreePath: worktreePath,
					Message:      fmt.Sprintf("Exited and removed worktree at %s.", worktreePath),
				},
			},
		}, nil
	}

	// Action is "keep" — leave the worktree on disk, no filesystem changes.
	logger.DebugCF("exit_worktree_tool", "worktree kept", map[string]any{
		"worktree_path": worktreePath,
	})

	return coretool.Result{
		Output: fmt.Sprintf("Exited worktree. Your work is preserved at %s", worktreePath),
		Meta: map[string]any{
			"data": Output{
				Action:       "keep",
				OriginalCwd:  call.Context.WorkingDir,
				WorktreePath: worktreePath,
				Message:      fmt.Sprintf("Exited worktree. Your work is preserved at %s.", worktreePath),
			},
		},
	}, nil
}

// fireWorktreeRemoveHook dispatches the WorktreeRemove hook as a non-blocking
// notification after a worktree is successfully removed. It only fires when
// hooks are configured and not globally disabled.
func (t *Tool) fireWorktreeRemoveHook(ctx context.Context, worktreePath string) {
	if t.hooks == nil || t.hookCfg == nil {
		return
	}
	if t.disableAllHooks {
		return
	}
	if !t.hookCfg.HasEvent(hook.EventWorktreeRemove) {
		return
	}
	// Use parent directory as CWD since worktreePath was just removed.
	parentDir := filepath.Dir(worktreePath)
	hookInput := hook.WorktreeRemoveHookInput{
		BaseHookInput: hook.BaseHookInput{
			CWD: parentDir,
		},
		HookEventName: string(hook.EventWorktreeRemove),
		WorktreePath:  worktreePath,
	}
	_ = t.hooks.RunHooks(ctx, hook.EventWorktreeRemove, hookInput, parentDir)
}
