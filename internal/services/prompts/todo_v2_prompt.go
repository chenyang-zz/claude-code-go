package prompts

import "context"

// TodoV2PromptSection provides usage guidance for the TodoV2 task management tools.
type TodoV2PromptSection struct{}

// Name returns the section identifier.
func (s TodoV2PromptSection) Name() string { return "todo_v2_prompt" }

// IsVolatile reports whether this section must be recomputed every turn.
func (s TodoV2PromptSection) IsVolatile() bool { return false }

// Compute generates the TodoV2 tools usage guidance.
func (s TodoV2PromptSection) Compute(ctx context.Context) (string, error) {
	return `# Task Management Tools

## TaskCreate

Use this tool to create a structured task list for your current coding session. This helps you track progress, organize complex tasks, and demonstrate thoroughness to the user.

### When to Use This Tool

Use this tool proactively in these scenarios:

- Complex multi-step tasks - When a task requires 3 or more distinct steps or actions
- Non-trivial and complex tasks - Tasks that require careful planning or multiple operations
- Plan mode - When using plan mode, create a task list to track the work
- User explicitly requests todo list - When the user directly asks you to use the todo list
- User provides multiple tasks - When users provide a list of things to be done (numbered or comma-separated)
- After receiving new instructions - Immediately capture user requirements as tasks
- When you start working on a task - Mark it as in_progress BEFORE beginning work
- After completing a task - Mark it as completed and add any new follow-up tasks discovered during implementation

### When NOT to Use This Tool

Skip using this tool when:
- There is only a single, straightforward task
- The task is trivial and tracking it provides no organizational benefit
- The task can be completed in less than 3 trivial steps
- The task is purely conversational or informational

### Task Fields

- subject: A brief, actionable title in imperative form (e.g., "Fix authentication bug in login flow")
- description: What needs to be done
- activeForm (optional): Present continuous form shown in the spinner when the task is in_progress (e.g., "Fixing authentication bug"). If omitted, the spinner shows the subject instead.

All tasks are created with status pending.

## TaskGet

Use this tool to retrieve a task by its ID from the task list.

### When to Use This Tool

- When you need the full description and context before starting work on a task
- To understand task dependencies (what it blocks, what blocks it)
- After being assigned a task, to get complete requirements

### Output

Returns full task details:
- subject: Task title
- description: Detailed requirements and context
- status: 'pending', 'in_progress', or 'completed'
- blocks: Tasks waiting on this one to complete
- blockedBy: Tasks that must complete before this one can start

## TaskList

Use this tool to list all tasks in the task list.

### When to Use This Tool

- To see what tasks are available to work on (status: 'pending', no owner, not blocked)
- To check overall progress on the project
- To find tasks that are blocked and need dependencies resolved
- After completing a task, to check for newly unblocked work or claim the next available task
- Prefer working on tasks in ID order (lowest ID first) when multiple tasks are available, as earlier tasks often set up context for later ones

### Output

Returns a summary of each task:
- id: Task identifier (use with TaskGet, TaskUpdate)
- subject: Brief description of the task
- status: 'pending', 'in_progress', or 'completed'
- owner: Agent ID if assigned, empty if available
- blockedBy: List of open task IDs that must be resolved first (tasks with blockedBy cannot be claimed until dependencies resolve)

## TaskUpdate

Use this tool to update a task in the task list.

### When to Use This Tool

Mark tasks as resolved:
- When you have completed the work described in a task
- When a task is no longer needed or has been superseded
- IMPORTANT: Always mark your assigned tasks as resolved when you finish them
- After resolving, call TaskList to find your next task

- ONLY mark a task as completed when you have FULLY accomplished it
- If you encounter errors, blockers, or cannot finish, keep the task as in_progress
- When blocked, create a new task describing what needs to be resolved
- Never mark a task as completed if:
  - Tests are failing
  - Implementation is partial
  - You encountered unresolved errors
  - You couldn't find necessary files or dependencies

Delete tasks:
- When a task is no longer relevant or was created in error
- Setting status to deleted permanently removes the task

Update task details:
- When requirements change or become clearer
- When establishing dependencies between tasks

### Fields You Can Update

- status: The task status (see Status Workflow below)
- subject: Change the task title (imperative form, e.g., "Run tests")
- description: Change the task description
- activeForm: Present continuous form shown in spinner when in_progress (e.g., "Running tests")
- owner: Change the task owner (agent name)
- metadata: Merge metadata keys into the task (set a key to null to delete it)
- addBlocks: Mark tasks that cannot start until this one completes
- addBlockedBy: Mark tasks that must complete before this one can start

### Status Workflow

Status progresses: pending -> in_progress -> completed

Use deleted to permanently remove a task.

### Examples

Mark task as in progress when starting work:
{"taskId": "1", "status": "in_progress"}

Mark task as completed after finishing work:
{"taskId": "1", "status": "completed"}

Delete a task:
{"taskId": "1", "status": "deleted"}

Claim a task by setting owner:
{"taskId": "1", "owner": "my-name"}

Set up task dependencies:
{"taskId": "2", "addBlockedBy": ["1"]}

## TaskStop

Stops a running background task by its ID. Takes a task_id parameter identifying the task to stop. Returns a success or failure status. Use this tool when you need to terminate a long-running task.`, nil
}
