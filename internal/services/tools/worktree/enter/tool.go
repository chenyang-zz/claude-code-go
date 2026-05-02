package enter

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/sheepzhao/claude-code-go/internal/core/hook"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	runtimehooks "github.com/sheepzhao/claude-code-go/internal/runtime/hooks"
	worktreeshared "github.com/sheepzhao/claude-code-go/internal/services/tools/worktree/shared"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// Name is the stable registry identifier for the EnterWorktreeTool.
	Name = "EnterWorktree"
)

// HookDispatcher dispatches hook events for worktree lifecycle hooks.
type HookDispatcher interface {
	// RunHooks executes hooks for the given event and returns results.
	// Returns nil when no hooks are configured.
	RunHooks(ctx context.Context, event hook.HookEvent, input any, cwd string) []hook.HookResult
}

// Tool implements the EnterWorktreeTool for creating isolated git worktrees.
type Tool struct {
	manager         *worktreeshared.Manager
	hooks           HookDispatcher
	hookCfg         hook.HooksConfig
	disableAllHooks bool
}

// Input is the typed request payload for EnterWorktreeTool.
type Input struct {
	Name string `json:"name,omitempty"`
}

// Output is the structured result returned after creating a worktree.
type Output struct {
	WorktreePath   string `json:"worktreePath"`
	WorktreeBranch string `json:"worktreeBranch,omitempty"`
	Message        string `json:"message"`
}

// NewTool constructs an EnterWorktreeTool with a shared worktree manager.
func NewTool(manager *worktreeshared.Manager) *Tool {
	return &Tool{manager: manager}
}

// NewToolWithHooks constructs an EnterWorktreeTool with hook dispatch capability.
func NewToolWithHooks(manager *worktreeshared.Manager, dispatcher HookDispatcher, hookCfg hook.HooksConfig, disableAllHooks bool) *Tool {
	return &Tool{manager: manager, hooks: dispatcher, hookCfg: hookCfg, disableAllHooks: disableAllHooks}
}

// Name returns the stable tool identifier.
func (t *Tool) Name() string {
	return Name
}

// Description returns a short human-readable summary for the tool.
func (t *Tool) Description() string {
	return "Creates an isolated git worktree and switches the session into it. Use this tool ONLY when the user explicitly asks to work in a worktree. Must be in a git repository. Creates a new git worktree inside .claude/worktrees/ with a new branch based on HEAD."
}

// InputSchema returns the declared input contract.
func (t *Tool) InputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"name": {
				Type: coretool.ValueKindString,
				Description: "Optional name for the worktree. Each \"/\"-separated segment may contain only letters, digits, dots, underscores, and dashes; max 64 chars total. A random name is generated if not provided.",
			},
		},
	}
}

// IsReadOnly reports that EnterWorktreeTool creates a worktree and is not read-only.
func (t *Tool) IsReadOnly() bool {
	return false
}

// IsConcurrencySafe reports that worktree creation cannot run concurrently.
func (t *Tool) IsConcurrencySafe() bool {
	return false
}

// RequiresUserInteraction indicates the tool requires user approval before execution.
func (t *Tool) RequiresUserInteraction() bool {
	return true
}

// Invoke creates a new git worktree and returns its path.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{}, fmt.Errorf("enter worktree tool: nil receiver")
	}
	if t.manager == nil {
		return coretool.Result{}, fmt.Errorf("enter worktree tool: worktree manager is not configured")
	}

	input, err := coretool.DecodeInput[Input](t.InputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	// Generate slug from input name or a random suffix.
	slug := strings.TrimSpace(input.Name)
	if slug == "" {
		slug = randomSlug()
	}

	// Validate the slug.
	if err := t.manager.ValidateSlug(slug); err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	// Create the worktree.
	result, err := t.manager.CreateWorktree(call.Context.WorkingDir, slug)
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("enter worktree tool: %v", err)}, nil
	}

	// Fire WorktreeCreate hooks after successful git worktree creation.
	// This is a post-creation notification hook. Blocking (exit code 2) returns
	// an error to the caller; the worktree is already created and cannot be
	// rolled back within this scope.
	if t.shouldRunWorktreeHooks(hook.EventWorktreeCreate) {
		hookInput := hook.WorktreeCreateHookInput{
			BaseHookInput: hook.BaseHookInput{
				CWD: call.Context.WorkingDir,
			},
			HookEventName: string(hook.EventWorktreeCreate),
			Name:          slug,
		}
		results := t.hooks.RunHooks(ctx, hook.EventWorktreeCreate, hookInput, call.Context.WorkingDir)
		if runtimehooks.HasBlockingResult(results) {
			errs := runtimehooks.BlockingErrors(results)
			return coretool.Result{Error: fmt.Sprintf("WorktreeCreate blocked by hook:\n%s", strings.Join(errs, "\n"))}, nil
		}
	}

	logger.DebugCF("enter_worktree_tool", "worktree created", map[string]any{
		"slug":        slug,
		"path":        result.Path,
		"branch":      result.Branch,
		"working_dir": call.Context.WorkingDir,
	})

	branchInfo := ""
	if result.Branch != "" {
		branchInfo = fmt.Sprintf(" on branch %s", result.Branch)
	}

	return coretool.Result{
		Output: fmt.Sprintf("Created worktree at %s%s", result.Path, branchInfo),
		Meta: map[string]any{
			"data": Output{
				WorktreePath:   result.Path,
				WorktreeBranch: result.Branch,
				Message: fmt.Sprintf(
					"Created worktree at %s%s. The session is now working in the worktree. Use ExitWorktree to leave mid-session, or exit the session to be prompted.",
					result.Path, branchInfo,
				),
			},
		},
	}, nil
}

// shouldRunWorktreeHooks reports whether worktree lifecycle hooks should execute
// for the given event.
func (t *Tool) shouldRunWorktreeHooks(event hook.HookEvent) bool {
	if t.hooks == nil || t.hookCfg == nil {
		return false
	}
	if t.disableAllHooks {
		return false
	}
	return t.hookCfg.HasEvent(event)
}

// randomSlug generates a short random slug for unnamed worktrees.
func randomSlug() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	const length = 8
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rng.Intn(len(charset))]
	}
	return string(b)
}
