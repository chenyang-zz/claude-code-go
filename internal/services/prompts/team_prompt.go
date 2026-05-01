package prompts

import "context"

// TeamPromptSection provides usage guidance for the TeamCreate and TeamDelete tools.
type TeamPromptSection struct{}

// Name returns the section identifier.
func (s TeamPromptSection) Name() string { return "team_prompt" }

// IsVolatile reports whether this section must be recomputed every turn.
func (s TeamPromptSection) IsVolatile() bool { return false }

// Compute generates the team tools usage guidance.
func (s TeamPromptSection) Compute(ctx context.Context) (string, error) {
	return "# Team Tools\n\n" +
		"## TeamCreate\n\n" +
		"Use this tool proactively whenever:\n" +
		"- The user explicitly asks to use a team, swarm, or group of agents\n" +
		"- The user mentions wanting agents to work together, coordinate, or collaborate\n" +
		"- A task is complex enough that it would benefit from parallel work by multiple agents (e.g., building a full-stack feature with frontend and backend work, refactoring a codebase while keeping tests passing, implementing a multi-step project with research, planning, and coding phases)\n\n" +
		"When in doubt about whether a task warrants a team, prefer spawning a team.\n\n" +
		"### Choosing Agent Types for Teammates\n\n" +
		"When spawning teammates via the Agent tool, choose the subagent_type based on what tools the agent needs for its task. Each agent type has a different set of available tools — match the agent to the work:\n\n" +
		"- Read-only agents (e.g., Explore, Plan) cannot edit or write files. Only assign them research, search, or planning tasks. Never assign them implementation work.\n" +
		"- Full-capability agents (e.g., general-purpose) have access to all tools including file editing, writing, and bash. Use these for tasks that require making changes.\n" +
		"- Custom agents defined in .claude/agents/ may have their own tool restrictions. Check their descriptions to understand what they can and cannot do.\n\n" +
		"Always review the agent type descriptions and their available tools listed in the Agent tool prompt before selecting a subagent_type for a teammate.\n\n" +
		"Create a new team to coordinate multiple agents working on a project. Teams have a 1:1 correspondence with task lists (Team = TaskList).\n\n" +
		"```json\n" +
		`{"team_name": "my-project", "description": "Working on feature X"}` + "\n" +
		"```\n\n" +
		"This creates:\n" +
		"- A team file at ~/.claude/teams/{team-name}/config.json\n" +
		"- A corresponding task list directory at ~/.claude/tasks/{team-name}/\n\n" +
		"### Team Workflow\n\n" +
		"1. Create a team with TeamCreate - this creates both the team and its task list\n" +
		"2. Create tasks using the Task tools (TaskCreate, TaskList, etc.) - they automatically use the team's task list\n" +
		"3. Spawn teammates using the Agent tool with team_name and name parameters to create teammates that join the team\n" +
		"4. Assign tasks using TaskUpdate with owner to give tasks to idle teammates\n" +
		"5. Teammates work on assigned tasks and mark them completed via TaskUpdate\n" +
		"6. Teammates go idle between turns - after each turn, teammates automatically go idle and send a notification. Be patient with idle teammates! Don't comment on their idleness until it actually impacts your work.\n" +
		"7. Shutdown your team - when the task is completed, gracefully shut down your teammates via SendMessage with message: {type: \"shutdown_request\"}.\n\n" +
		"### Task Ownership\n\n" +
		"Tasks are assigned using TaskUpdate with the owner parameter. Any agent can set or change task ownership via TaskUpdate.\n\n" +
		"### Automatic Message Delivery\n\n" +
		"IMPORTANT: Messages from teammates are automatically delivered to you. You do NOT need to manually check your inbox.\n\n" +
		"When you spawn teammates:\n" +
		"- They will send you messages when they complete tasks or need help\n" +
		"- These messages appear automatically as new conversation turns (like user messages)\n" +
		"- If you're busy (mid-turn), messages are queued and delivered when your turn ends\n" +
		"- The UI shows a brief notification with the sender's name when messages are waiting\n\n" +
		"Messages will be delivered automatically.\n\n" +
		"When reporting on teammate messages, you do NOT need to quote the original message—it's already rendered to the user.\n\n" +
		"### Teammate Idle State\n\n" +
		"Teammates go idle after every turn—this is completely normal and expected. A teammate going idle immediately after sending you a message does NOT mean they are done or unavailable. Idle simply means they are waiting for input.\n\n" +
		"- Idle teammates can receive messages. Sending a message to an idle teammate wakes them up and they will process it normally.\n" +
		"- Idle notifications are automatic. The system sends an idle notification whenever a teammate's turn ends. You do not need to react to idle notifications unless you want to assign new work or send a follow-up message.\n" +
		"- Do not treat idle as an error. A teammate sending a message and then going idle is the normal flow—they sent their message and are now waiting for a response.\n" +
		"- Peer DM visibility. When a teammate sends a DM to another teammate, a brief summary is included in their idle notification. This gives you visibility into peer collaboration without the full message content. You do not need to respond to these summaries — they are informational.\n\n" +
		"### Discovering Team Members\n\n" +
		"Teammates can read the team config file to discover other team members:\n" +
		"- Team config location: ~/.claude/teams/{team-name}/config.json\n\n" +
		"The config file contains a members array with each teammate's:\n" +
		"- name: Human-readable name (always use this for messaging and task assignment)\n" +
		"- agentId: Unique identifier (for reference only - do not use for communication)\n" +
		"- agentType: Role/type of the agent\n\n" +
		"IMPORTANT: Always refer to teammates by their NAME (e.g., \"team-lead\", \"researcher\", \"tester\"). Names are used for:\n" +
		"- to when sending messages\n" +
		"- Identifying task owners\n\n" +
		"### Task List Coordination\n\n" +
		"Teams share a task list that all teammates can access at ~/.claude/tasks/{team-name}/.\n\n" +
		"Teammates should:\n" +
		"1. Check TaskList periodically, especially after completing each task, to find available work or see newly unblocked tasks\n" +
		"2. Claim unassigned, unblocked tasks with TaskUpdate (set owner to your name). Prefer tasks in ID order (lowest ID first) when multiple tasks are available, as earlier tasks often set up context for later ones\n" +
		"3. Create new tasks with TaskCreate when identifying additional work\n" +
		"4. Mark tasks as completed with TaskUpdate when done, then check TaskList for next work\n" +
		"5. Coordinate with other teammates by reading the task list status\n" +
		"6. If all available tasks are blocked, notify the team lead or help resolve blocking tasks\n\n" +
		"IMPORTANT notes for communication with your team:\n" +
		"- Do not use terminal tools to view your team's activity; always send a message to your teammates (and remember, refer to them by name).\n" +
		"- Your team cannot hear you if you do not use the SendMessage tool. Always send a message to your teammates if you are responding to them.\n" +
		"- Do NOT send structured JSON status messages like {\"type\":\"idle\",...} or {\"type\":\"task_completed\",...}. Just communicate in plain text when you need to message teammates.\n" +
		"- Use TaskUpdate to mark tasks completed.\n" +
		"- If you are an agent in the team, the system will automatically send idle notifications to the team lead when you stop.\n\n" +
		"## TeamDelete\n\n" +
		"Remove team and task directories when the swarm work is complete.\n\n" +
		"This operation:\n" +
		"- Removes the team directory (~/.claude/teams/{team-name}/)\n" +
		"- Removes the task directory (~/.claude/tasks/{team-name}/)\n" +
		"- Clears team context from the current session\n\n" +
		"IMPORTANT: TeamDelete will fail if the team still has active members. Gracefully terminate teammates first, then call TeamDelete after all teammates have shut down.\n\n" +
		"Use this when all teammates have finished their work and you want to clean up the team resources. The team name is automatically determined from the current session's team context.", nil
}
