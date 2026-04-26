package memory

import (
	"fmt"
	"os"
	"strings"
)

const (
	entrypointName       = "MEMORY.md"
	maxEntrypointLines   = 200
	maxEntrypointBytes   = 25000
	dirExistsGuidance    = "This directory already exists — write to it directly with the Write tool (do not run mkdir or check for its existence)."
)

// LoadAgentMemoryPrompt loads the persistent memory prompt for an agent.
// It creates the memory directory if needed and returns a prompt that includes
// the agent's MEMORY.md content (when present) along with scope-specific guidance.
//
// The scope parameter controls where the memory directory is located and what
// guidance note is attached:
//   - user:    memories should be general across all projects
//   - project: memories should be tailored to this project (shared via version control)
//   - local:   memories should be tailored to this project and machine (not version-controlled)
func LoadAgentMemoryPrompt(agentType string, scope AgentMemoryScope, paths *Paths) string {
	var scopeNote string
	switch scope {
	case ScopeUser:
		scopeNote = "- Since this memory is user-scope, keep learnings general since they apply across all projects"
	case ScopeProject:
		scopeNote = "- Since this memory is project-scope and shared with your team via version control, tailor your memories to this project"
	case ScopeLocal:
		scopeNote = "- Since this memory is local-scope (not checked into version control), tailor your memories to this project and machine"
	}

	memoryDir := paths.GetAgentMemoryDir(agentType, scope)

	// Fire-and-forget directory creation. Even if this races with the agent's
	// first Write tool call, FileWriteTool creates parent directories itself.
	_ = EnsureAgentMemoryDir(memoryDir)

	entrypoint := paths.GetAgentMemoryEntrypoint(agentType, scope)
	entrypointContent := ""
	if data, err := os.ReadFile(entrypoint); err == nil {
		entrypointContent = string(data)
	}

	return buildMemoryPrompt(memoryDir, entrypointContent, scopeNote)
}

// buildMemoryPrompt constructs the full memory prompt string.
func buildMemoryPrompt(memoryDir, entrypointContent, scopeNote string) string {
	var lines []string

	lines = append(lines, "# Persistent Agent Memory")
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("You have a persistent, file-based memory system at `%s`. %s", memoryDir, dirExistsGuidance))
	lines = append(lines, "")
	lines = append(lines, "You should build up this memory system over time so that future conversations can have a complete picture of who the user is, how they'd like to collaborate with you, what behaviors to avoid or repeat, and the context behind the work the user gives you.")
	lines = append(lines, "")
	lines = append(lines, "If the user explicitly asks you to remember something, save it immediately as whichever type fits best. If they ask you to forget something, find and remove the relevant entry.")
	lines = append(lines, "")
	lines = append(lines, scopeNote)
	lines = append(lines, "")
	lines = append(lines, "## How to save memories")
	lines = append(lines, "")
	lines = append(lines, "Saving a memory is a two-step process:")
	lines = append(lines, "")
	lines = append(lines, "**Step 1** — write the memory to its own file (e.g., `user_role.md`, `feedback_testing.md`) using frontmatter format.")
	lines = append(lines, "")
	lines = append(lines, "**Step 2** — add a pointer to that file in `MEMORY.md`. `MEMORY.md` is an index, not a memory — each entry should be one line, under ~150 characters. It has no frontmatter. Never write memory content directly into `MEMORY.md`.")
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("- `MEMORY.md` is always loaded into your conversation context — lines after %d will be truncated, so keep the index concise", maxEntrypointLines))
	lines = append(lines, "- Keep the name, description, and type fields in memory files up-to-date with the content")
	lines = append(lines, "- Organize memory semantically by topic, not chronologically")
	lines = append(lines, "- Update or remove memories that turn out to be wrong or outdated")
	lines = append(lines, "- Do not write duplicate memories. First check if there is an existing memory you can update before writing a new one.")
	lines = append(lines, "")

	if trimmed := strings.TrimSpace(entrypointContent); trimmed != "" {
		t := truncateEntrypointContent(trimmed)
		lines = append(lines, fmt.Sprintf("## %s", entrypointName))
		lines = append(lines, "")
		lines = append(lines, t.content)
	} else {
		lines = append(lines, fmt.Sprintf("## %s", entrypointName))
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("Your %s is currently empty. When you save new memories, they will appear here.", entrypointName))
	}

	return strings.Join(lines, "\n")
}

// truncatedEntrypoint holds the result of truncating MEMORY.md content.
type truncatedEntrypoint struct {
	content          string
	lineCount        int
	byteCount        int
	wasLineTruncated bool
	wasByteTruncated bool
}

// truncateEntrypointContent truncates MEMORY.md content to line and byte caps.
// Line truncation happens first; if the result still exceeds the byte cap,
// it is cut at the last newline before the cap.
func truncateEntrypointContent(raw string) truncatedEntrypoint {
	trimmed := strings.TrimSpace(raw)
	contentLines := strings.Split(trimmed, "\n")
	lineCount := len(contentLines)
	byteCount := len(trimmed)

	wasLineTruncated := lineCount > maxEntrypointLines
	wasByteTruncated := byteCount > maxEntrypointBytes

	if !wasLineTruncated && !wasByteTruncated {
		return truncatedEntrypoint{
			content:          trimmed,
			lineCount:        lineCount,
			byteCount:        byteCount,
			wasLineTruncated: false,
			wasByteTruncated: false,
		}
	}

	truncated := trimmed
	if wasLineTruncated {
		truncated = strings.Join(contentLines[:maxEntrypointLines], "\n")
	}

	if len(truncated) > maxEntrypointBytes {
		cutAt := strings.LastIndex(truncated[:maxEntrypointBytes], "\n")
		if cutAt > 0 {
			truncated = truncated[:cutAt]
		} else {
			truncated = truncated[:maxEntrypointBytes]
		}
	}

	var reason string
	if wasByteTruncated && !wasLineTruncated {
		reason = fmt.Sprintf("%d bytes (limit: %d) — index entries are too long", byteCount, maxEntrypointBytes)
	} else if wasLineTruncated && !wasByteTruncated {
		reason = fmt.Sprintf("%d lines (limit: %d)", lineCount, maxEntrypointLines)
	} else {
		reason = fmt.Sprintf("%d lines and %d bytes", lineCount, byteCount)
	}

	return truncatedEntrypoint{
		content: truncated + fmt.Sprintf("\n\n> WARNING: %s is %s. Only part of it was loaded. Keep index entries to one line under ~200 chars; move detail into topic files.", entrypointName, reason),
		lineCount:        lineCount,
		byteCount:        byteCount,
		wasLineTruncated: wasLineTruncated,
		wasByteTruncated: wasByteTruncated,
	}
}
