package prompts

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/services/extractmemories"
	"github.com/sheepzhao/claude-code-go/internal/services/teammemsync"
)

// TeamMemoryPromptSection provides the file-based memory system instructions
// (MEMORY.md) for the system prompt. It covers the full four-type memory taxonomy
// with dual-scope guidance (private auto-memory + shared team memory).
//
// When team memory is disabled (FlagTeamMemorySync off), it renders an
// individual-only prompt; when enabled, it renders the combined auto+team prompt
// including scope tags and the team-memory-sensitive-data warning.
type TeamMemoryPromptSection struct{}

// Name returns the unique section identifier.
func (s TeamMemoryPromptSection) Name() string { return "auto_team_memory" }

// IsVolatile reports that this section is stable across turns.
func (s TeamMemoryPromptSection) IsVolatile() bool { return false }

// Compute builds the file-based memory system prompt section derived from
// TS teamMemPrompts.ts buildCombinedMemoryPrompt and memoryTypes.ts constants.
func (s TeamMemoryPromptSection) Compute(ctx context.Context) (string, error) {
	if !extractmemories.IsAutoMemoryEnabled() {
		return "", nil
	}

	data, _ := RuntimeContextFromContext(ctx)
	projectRoot := strings.TrimSpace(data.WorkingDir)
	if projectRoot == "" {
		// Without a project root we cannot construct correct paths.
		return "", nil
	}

	autoMemPath := extractmemories.GetAutoMemPath(projectRoot)
	teamEnabled := teammemsync.IsTeamMemoryEnabled()

	var b strings.Builder

	// ── HEADER ──────────────────────────────────────────────────
	if teamEnabled {
		b.WriteString("# Memory\n\n")
		b.WriteString("You have a persistent, file-based memory system with two directories: ")
		b.WriteString("a private directory at `" + autoMemPath + "` and ")
		b.WriteString("a shared team directory at `" + teammemsync.GetTeamMemPath(projectRoot) + "`. ")
		b.WriteString("Both directories already exist.\n\n")

		b.WriteString("You should build up this memory system over time so that future conversations ")
		b.WriteString("can have a complete picture of who the user is, how they'd like to collaborate with you, ")
		b.WriteString("what behaviors to avoid or repeat, and the context behind the work the user gives you.\n\n")

		b.WriteString("If the user explicitly asks you to remember something, save it immediately as whichever type fits best. ")
		b.WriteString("If they ask you to forget something, find and remove the relevant entry.\n\n")
	} else {
		b.WriteString("# Memory\n\n")
		b.WriteString("You have a persistent, file-based memory system at `" + autoMemPath + "`. ")
		b.WriteString("This directory already exists.\n\n")

		b.WriteString("You should build up this memory system over time so that future conversations ")
		b.WriteString("can have a complete picture of who the user is and how they'd like to collaborate with you.\n\n")

		b.WriteString("If the user explicitly asks you to remember something, save it immediately as whichever type fits best. ")
		b.WriteString("If they ask you to forget something, find and remove the relevant entry.\n\n")
	}

	// ── MEMORY SCOPE ────────────────────────────────────────────
	if teamEnabled {
		b.WriteString("## Memory scope\n\n")
		b.WriteString("There are two scope levels:\n\n")
		b.WriteString("- private: memories that are private between you and the current user. ")
		b.WriteString("They persist across conversations with only this specific user and are stored at the root `" + autoMemPath + "`.\n")
		b.WriteString("- team: memories that are shared with and contributed by all of the users who work within this project directory. ")
		b.WriteString("Team memories are synced at the beginning of every session and they are stored at `" + teammemsync.GetTeamMemPath(projectRoot) + "`.\n\n")
	}

	// ── TYPES SECTION ───────────────────────────────────────────
	b.WriteString(renderTypesSection(teamEnabled))

	// ── WHAT NOT TO SAVE ────────────────────────────────────────
	b.WriteString(whatNotToSaveSection)

	// ── TEAM MEMORY SECRETS WARNING ─────────────────────────────
	if teamEnabled {
		b.WriteString("- You MUST avoid saving sensitive data within shared team memories. ")
		b.WriteString("For example, never save API keys or user credentials.\n\n")
	}

	// ── HOW TO SAVE ─────────────────────────────────────────────
	b.WriteString(renderHowToSave(teamEnabled, autoMemPath, projectRoot))

	// ── WHEN TO ACCESS ──────────────────────────────────────────
	b.WriteString(whenToAccessSection)

	// ── MEMORY DRIFT ────────────────────────────────────────────
	b.WriteString(memoryDriftCaveat + "\n\n")

	// ── TRUSTING RECALL ─────────────────────────────────────────
	b.WriteString(trustingRecallSection)

	// ── MEMORY VS OTHER PERSISTENCE ─────────────────────────────
	b.WriteString(renderMemoryVsPersistence())

	return b.String(), nil
}

// renderTypesSection returns the combined or individual types taxonomy
// derived from TS memoryTypes.ts TYPES_SECTION_COMBINED / TYPES_SECTION_INDIVIDUAL.
func renderTypesSection(teamEnabled bool) string {
	var b strings.Builder

	if teamEnabled {
		b.WriteString("## Types of memory\n\n")
		b.WriteString("There are several discrete types of memory that you can store in your memory system. ")
		b.WriteString("Each type below declares a <scope> of `private`, `team`, or guidance for choosing between the two.\n\n")
	} else {
		b.WriteString("## Types of memory\n\n")
		b.WriteString("There are several discrete types of memory that you can store in your memory system:\n\n")
	}

	b.WriteString("<types>\n")

	// ── USER ──
	b.WriteString(renderUserType(teamEnabled))

	// ── FEEDBACK ──
	b.WriteString(renderFeedbackType(teamEnabled))

	// ── PROJECT ──
	b.WriteString(renderProjectType(teamEnabled))

	// ── REFERENCE ──
	b.WriteString(renderReferenceType(teamEnabled))

	b.WriteString("</types>\n\n")
	return b.String()
}

func renderUserType(teamEnabled bool) string {
	var b strings.Builder
	b.WriteString("<type>\n")
	b.WriteString("    <name>user</name>\n")
	if teamEnabled {
		b.WriteString("    <scope>always private</scope>\n")
	}
	b.WriteString("    <description>Contain information about the user's role, goals, responsibilities, and knowledge. ")
	b.WriteString("Great user memories help you tailor your future behavior to the user's preferences and perspective. ")
	b.WriteString("Your goal in reading and writing these memories is to build up an understanding of who the user is and how you can be most helpful to them specifically. ")
	b.WriteString("For example, you should collaborate with a senior software engineer differently than a student who is coding for the very first time. ")
	b.WriteString("Keep in mind, that the aim here is to be helpful to the user. ")
	b.WriteString("Avoid writing memories about the user that could be viewed as a negative judgement or that are not relevant to the work you're trying to accomplish together.</description>\n")
	b.WriteString("    <when_to_save>When you learn any details about the user's role, preferences, responsibilities, or knowledge</when_to_save>\n")
	b.WriteString("    <how_to_use>When your work should be informed by the user's profile or perspective. For example, if the user is asking you to explain a part of the code, you should answer that question in a way that is tailored to the specific details that they will find most valuable or that helps them build their mental model in relation to domain knowledge they already have.</how_to_use>\n")
	b.WriteString("    <examples>\n")
	if teamEnabled {
		b.WriteString("    user: I'm a data scientist investigating what logging we have in place\n")
		b.WriteString("    assistant: [saves private user memory: user is a data scientist, currently focused on observability/logging]\n\n")
		b.WriteString("    user: I've been writing Go for ten years but this is my first time touching the React side of this repo\n")
		b.WriteString("    assistant: [saves private user memory: deep Go expertise, new to React and this project's frontend — frame frontend explanations in terms of backend analogues]\n")
	} else {
		b.WriteString("    user: I'm a data scientist investigating what logging we have in place\n")
		b.WriteString("    assistant: [saves user memory: user is a data scientist, currently focused on observability/logging]\n\n")
		b.WriteString("    user: I've been writing Go for ten years but this is my first time touching the React side of this repo\n")
		b.WriteString("    assistant: [saves user memory: deep Go expertise, new to React and this project's frontend — frame frontend explanations in terms of backend analogues]\n")
	}
	b.WriteString("    </examples>\n")
	b.WriteString("</type>\n")
	return b.String()
}

func renderFeedbackType(teamEnabled bool) string {
	var b strings.Builder
	b.WriteString("<type>\n")
	b.WriteString("    <name>feedback</name>\n")
	if teamEnabled {
		b.WriteString("    <scope>default to private. Save as team only when the guidance is clearly a project-wide convention that every contributor should follow (e.g., a testing policy, a build invariant), not a personal style preference.</scope>\n")
	}
	b.WriteString("    <description>Guidance the user has given you about how to approach work — both what to avoid and what to keep doing. These are a very important type of memory to read and write as they allow you to remain coherent and responsive to the way you should approach work in the project. Record from failure AND success: if you only save corrections, you will avoid past mistakes but drift away from approaches the user has already validated, and may grow overly cautious.")
	if teamEnabled {
		b.WriteString(" Before saving a private feedback memory, check that it doesn't contradict a team feedback memory — if it does, either don't save it or note the override explicitly.")
	}
	b.WriteString("</description>\n")
	b.WriteString("    <when_to_save>Any time the user corrects your approach (\"no not that\", \"don't\", \"stop doing X\") OR confirms a non-obvious approach worked (\"yes exactly\", \"perfect, keep doing that\", accepting an unusual choice without pushback). Corrections are easy to notice; confirmations are quieter — watch for them. In both cases, save what is applicable to future conversations, especially if surprising or not obvious from the code. Include *why* so you can judge edge cases later.</when_to_save>\n")
	b.WriteString("    <how_to_use>Let these memories guide your behavior so that the user")
	if teamEnabled {
		b.WriteString(" and other users in the project")
	}
	b.WriteString(" do not need to offer the same guidance twice.</how_to_use>\n")
	b.WriteString("    <body_structure>Lead with the rule itself, then a **Why:** line (the reason the user gave — often a past incident or strong preference) and a **How to apply:** line (when/where this guidance kicks in). Knowing *why* lets you judge edge cases instead of blindly following the rule.</body_structure>\n")
	b.WriteString("    <examples>\n")
	if teamEnabled {
		b.WriteString("    user: don't mock the database in these tests — we got burned last quarter when mocked tests passed but the prod migration failed\n")
		b.WriteString("    assistant: [saves team feedback memory: integration tests must hit a real database, not mocks. Reason: prior incident where mock/prod divergence masked a broken migration. Team scope: this is a project testing policy, not a personal preference]\n\n")
		b.WriteString("    user: stop summarizing what you just did at the end of every response, I can read the diff\n")
		b.WriteString("    assistant: [saves private feedback memory: this user wants terse responses with no trailing summaries. Private because it's a communication preference, not a project convention]\n\n")
		b.WriteString("    user: yeah the single bundled PR was the right call here, splitting this one would've just been churn\n")
		b.WriteString("    assistant: [saves private feedback memory: for refactors in this area, user prefers one bundled PR over many small ones. Confirmed after I chose this approach — a validated judgment call, not a correction]\n")
	} else {
		b.WriteString("    user: don't mock the database in these tests — we got burned last quarter when mocked tests passed but the prod migration failed\n")
		b.WriteString("    assistant: [saves feedback memory: integration tests must hit a real database, not mocks. Reason: prior incident where mock/prod divergence masked a broken migration]\n\n")
		b.WriteString("    user: stop summarizing what you just did at the end of every response, I can read the diff\n")
		b.WriteString("    assistant: [saves feedback memory: this user wants terse responses with no trailing summaries]\n\n")
		b.WriteString("    user: yeah the single bundled PR was the right call here, splitting this one would've just been churn\n")
		b.WriteString("    assistant: [saves feedback memory: for refactors in this area, user prefers one bundled PR over many small ones. Confirmed after I chose this approach — a validated judgment call, not a correction]\n")
	}
	b.WriteString("    </examples>\n")
	b.WriteString("</type>\n")
	return b.String()
}

func renderProjectType(teamEnabled bool) string {
	var b strings.Builder
	b.WriteString("<type>\n")
	b.WriteString("    <name>project</name>\n")
	if teamEnabled {
		b.WriteString("    <scope>private or team, but strongly bias toward team</scope>\n")
	}
	b.WriteString("    <description>Information that you learn about ongoing work, goals, initiatives, bugs, or incidents within the project that is not otherwise derivable from the code or git history. Project memories help you understand the broader context and motivation behind the work users are working on within this working directory.</description>\n")
	b.WriteString("    <when_to_save>When you learn who is doing what, why, or by when. These states change relatively quickly so try to keep your understanding of this up to date. Always convert relative dates in user messages to absolute dates when saving (e.g., \"Thursday\" → \"2026-03-05\"), so the memory remains interpretable after time passes.</when_to_save>\n")
	b.WriteString("    <how_to_use>Use these memories to more fully understand the details and nuance behind the user's request")
	if teamEnabled {
		b.WriteString(", anticipate coordination issues across users,")
	}
	b.WriteString(" make better informed suggestions.</how_to_use>\n")
	b.WriteString("    <body_structure>Lead with the fact or decision, then a **Why:** line (the motivation — often a constraint, deadline, or stakeholder ask) and a **How to apply:** line (how this should shape your suggestions). Project memories decay fast, so the why helps future-you judge whether the memory is still load-bearing.</body_structure>\n")
	b.WriteString("    <examples>\n")
	if teamEnabled {
		b.WriteString("    user: we're freezing all non-critical merges after Thursday — mobile team is cutting a release branch\n")
		b.WriteString("    assistant: [saves team project memory: merge freeze begins 2026-03-05 for mobile release cut. Flag any non-critical PR work scheduled after that date]\n\n")
		b.WriteString("    user: the reason we're ripping out the old auth middleware is that legal flagged it for storing session tokens in a way that doesn't meet the new compliance requirements\n")
		b.WriteString("    assistant: [saves team project memory: auth middleware rewrite is driven by legal/compliance requirements around session token storage, not tech-debt cleanup — scope decisions should favor compliance over ergonomics]\n")
	} else {
		b.WriteString("    user: we're freezing all non-critical merges after Thursday — mobile team is cutting a release branch\n")
		b.WriteString("    assistant: [saves project memory: merge freeze begins 2026-03-05 for mobile release cut. Flag any non-critical PR work scheduled after that date]\n\n")
		b.WriteString("    user: the reason we're ripping out the old auth middleware is that legal flagged it for storing session tokens in a way that doesn't meet the new compliance requirements\n")
		b.WriteString("    assistant: [saves project memory: auth middleware rewrite is driven by legal/compliance requirements around session token storage, not tech-debt cleanup — scope decisions should favor compliance over ergonomics]\n")
	}
	b.WriteString("    </examples>\n")
	b.WriteString("</type>\n")
	return b.String()
}

func renderReferenceType(teamEnabled bool) string {
	var b strings.Builder
	b.WriteString("<type>\n")
	b.WriteString("    <name>reference</name>\n")
	if teamEnabled {
		b.WriteString("    <scope>usually team</scope>\n")
	}
	b.WriteString("    <description>Stores pointers to where information can be found in external systems. These memories allow you to remember where to look to find up-to-date information outside of the project directory.</description>\n")
	b.WriteString("    <when_to_save>When you learn about resources in external systems and their purpose. For example, that bugs are tracked in a specific project in Linear or that feedback can be found in a specific Slack channel.</when_to_save>\n")
	b.WriteString("    <how_to_use>When the user references an external system or information that may be in an external system.</how_to_use>\n")
	b.WriteString("    <examples>\n")
	if teamEnabled {
		b.WriteString("    user: check the Linear project \"INGEST\" if you want context on these tickets, that's where we track all pipeline bugs\n")
		b.WriteString("    assistant: [saves team reference memory: pipeline bugs are tracked in Linear project \"INGEST\"]\n\n")
		b.WriteString("    user: the Grafana board at grafana.internal/d/api-latency is what oncall watches — if you're touching request handling, that's the thing that'll page someone\n")
		b.WriteString("    assistant: [saves team reference memory: grafana.internal/d/api-latency is the oncall latency dashboard — check it when editing request-path code]\n")
	} else {
		b.WriteString("    user: check the Linear project \"INGEST\" if you want context on these tickets, that's where we track all pipeline bugs\n")
		b.WriteString("    assistant: [saves reference memory: pipeline bugs are tracked in Linear project \"INGEST\"]\n\n")
		b.WriteString("    user: the Grafana board at grafana.internal/d/api-latency is what oncall watches — if you're touching request handling, that's the thing that'll page someone\n")
		b.WriteString("    assistant: [saves reference memory: grafana.internal/d/api-latency is the oncall latency dashboard — check it when editing request-path code]\n")
	}
	b.WriteString("    </examples>\n")
	b.WriteString("</type>\n")
	return b.String()
}

// whatNotToSaveSection mirrors TS memoryTypes.ts WHAT_NOT_TO_SAVE_SECTION.
const whatNotToSaveSection = `## What NOT to save in memory

- Code patterns, conventions, architecture, file paths, or project structure — these can be derived by reading the current project state.
- Git history, recent changes, or who-changed-what — ` + "`git log` / `git blame`" + ` are authoritative.
- Debugging solutions or fix recipes — the fix is in the code; the commit message has the context.
- Anything already documented in CLAUDE.md files.
- Ephemeral task details: in-progress work, temporary state, current conversation context.

These exclusions apply even when the user explicitly asks you to save. If they ask you to save a PR list or activity summary, ask what was *surprising* or *non-obvious* about it — that is the part worth keeping.

`

// memoryDriftCaveat mirrors TS memoryTypes.ts MEMORY_DRIFT_CAVEAT.
const memoryDriftCaveat = "- Memory records can become stale over time. Use memory as context for what was true at a given point in time. Before answering the user or building assumptions based solely on information in memory records, verify that the memory is still correct and up-to-date by reading the current state of the files or resources. If a recalled memory conflicts with current information, trust what you observe now — and update or remove the stale memory rather than acting on it."

// whenToAccessSection mirrors TS memoryTypes.ts WHEN_TO_ACCESS_SECTION.
const whenToAccessSection = `## When to access memories
- When memories seem relevant, or the user references prior-conversation work.
- You MUST access memory when the user explicitly asks you to check, recall, or remember.
- If the user says to *ignore* or *not use* memory: proceed as if MEMORY.md were empty. Do not apply remembered facts, cite, compare against, or mention memory content.
` + memoryDriftCaveat + `

`

// trustingRecallSection mirrors TS memoryTypes.ts TRUSTING_RECALL_SECTION.
const trustingRecallSection = `## Before recommending from memory

A memory that names a specific function, file, or flag is a claim that it existed *when the memory was written*. It may have been renamed, removed, or never merged. Before recommending it:

- If the memory names a file path: check the file exists.
- If the memory names a function or flag: grep for it.
- If the user is about to act on your recommendation (not just asking about history), verify first.

"The memory says X exists" is not the same as "X exists now."

A memory that summarizes repo state (activity logs, architecture snapshots) is frozen in time. If the user asks about *recent* or *current* state, prefer ` + "`git log`" + ` or reading the code over recalling the snapshot.

`

// renderHowToSave renders the "How to save memories" section with two-step
// flow when team is enabled and simplified flow otherwise.
func renderHowToSave(teamEnabled bool, autoMemPath string, projectRoot string) string {
	var b strings.Builder
	entrypointName := "MEMORY.md"
	maxLines := "200"

	b.WriteString("## How to save memories\n\n")

	if teamEnabled {
		b.WriteString("Saving a memory is a two-step process:\n\n")
		b.WriteString("**Step 1** — write the memory to its own file in the chosen directory ")
		b.WriteString("(private or team, per the type's scope guidance) using this frontmatter format:\n\n")
	} else {
		b.WriteString("Saving a memory is a two-step process:\n\n")
		b.WriteString("**Step 1** — write the memory to its own file using this frontmatter format:\n\n")
	}

	b.WriteString("```markdown\n")
	b.WriteString("---\n")
	b.WriteString("name: {{memory name}}\n")
	b.WriteString("description: {{one-line description — used to decide relevance in future conversations, so be specific}}\n")
	b.WriteString("type: {{user, feedback, project, reference}}\n")
	b.WriteString("---\n\n")
	b.WriteString("{{memory content — for feedback/project types, structure as: rule/fact, then **Why:** and **How to apply:** lines}}\n")
	b.WriteString("```\n\n")

	if teamEnabled {
		b.WriteString("**Step 2** — add a pointer to that file in the same directory's `" + entrypointName + "`. ")
		b.WriteString("Each directory (private and team) has its own `" + entrypointName + "` index — ")
		b.WriteString("each entry should be one line, under ~" + maxLines + " characters: ")
		b.WriteString("`- [Title](file.md) — one-line hook`. They have no frontmatter. ")
		b.WriteString("Never write memory content directly into a `" + entrypointName + "`.\n\n")
		b.WriteString("- Both `" + entrypointName + "` indexes are loaded into your conversation context — ")
		b.WriteString("lines after " + maxLines + " will be truncated, so keep them concise\n")
	} else {
		b.WriteString("**Step 2** — add a pointer to that file in `" + entrypointName + "`. ")
		b.WriteString("Each entry should be one line, under ~" + maxLines + " characters: ")
		b.WriteString("`- [Title](file.md) — one-line hook`. It has no frontmatter. ")
		b.WriteString("Never write memory content directly into `" + entrypointName + "`.\n\n")
		teamIdxPath := filepath.Join(teamIdxPath(projectRoot))
		_ = teamIdxPath
		b.WriteString("- `" + entrypointName + "` is loaded into your conversation context — ")
		b.WriteString("lines after " + maxLines + " will be truncated, so keep the index concise\n")
	}

	b.WriteString("- Keep the name, description, and type fields in memory files up-to-date with the content\n")
	b.WriteString("- Organize memory semantically by topic, not chronologically\n")
	b.WriteString("- Update or remove memories that turn out to be wrong or outdated\n")
	b.WriteString("- Do not write duplicate memories. First check if there is an existing memory you can update before writing a new one.\n\n")

	return b.String()
}

// teamIdxPath is a helper placeholder used inside renderHowToSave.
func teamIdxPath(projectRoot string) string {
	return extractmemories.GetAutoMemPath(projectRoot) + "/MEMORY.md"
}

// renderMemoryVsPersistence mirrors the "Memory and other forms of persistence"
// section from TS teamMemPrompts.ts.
func renderMemoryVsPersistence() string {
	return `## Memory and other forms of persistence
Memory is one of several persistence mechanisms available to you as you assist the user in a given conversation. The distinction is often that memory can be recalled in future conversations and should not be used for persisting information that is only useful within the scope of the current conversation.
- When to use or update a plan instead of memory: If you are about to start a non-trivial implementation task and would like to reach alignment with the user on your approach you should use a Plan rather than saving this information to memory. Similarly, if you already have a plan within the conversation and you have changed your approach persist that change by updating the plan rather than saving a memory.
- When to use or update tasks instead of memory: When you need to break your work in current conversation into discrete steps or keep track of your progress use tasks instead of saving to memory. Tasks are great for persisting information about the work that needs to be done in the current conversation, but memory should be reserved for information that will be useful in future conversations.
`
}
