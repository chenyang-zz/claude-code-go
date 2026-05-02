package autodream

import (
	"fmt"
	"strings"
)

const (
	// entrypointName is the MEMORY.md index filename.
	entrypointName = "MEMORY.md"
	// maxEntrypointLines is the maximum number of lines for the MEMORY.md index.
	maxEntrypointLines = 200
)

// buildConsolidationPrompt builds the four-phase memory consolidation prompt.
//
// memoryRoot is the auto-memory directory path.
// transcriptDir is the directory containing session JSONL transcripts.
// extra is injected after the four phases (e.g. session list and tool constraints).
func buildConsolidationPrompt(memoryRoot, transcriptDir, extra string) string {
	var b strings.Builder

	b.WriteString("# Dream: Memory Consolidation\n\n")
	b.WriteString("You are performing a dream — a reflective pass over your memory files. ")
	b.WriteString("Synthesize what you've learned recently into durable, well-organized ")
	b.WriteString("memories so that future sessions can orient quickly.\n\n")

	b.WriteString(fmt.Sprintf("Memory directory: `%s`\n", memoryRoot))
	b.WriteString("The memory directory already exists — no need to create it.\n\n")
	b.WriteString(fmt.Sprintf("Session transcripts: `%s` (large JSONL files — grep narrowly, don't read whole files)\n\n", transcriptDir))

	b.WriteString("---\n\n")

	// Phase 1: Orient.
	b.WriteString("## Phase 1 — Orient\n\n")
	b.WriteString("- `ls` the memory directory to see what already exists (the directory is already created)\n")
	b.WriteString(fmt.Sprintf("- Read `%s` to understand the current index\n", entrypointName))
	b.WriteString("- Skim existing topic files so you improve them rather than creating duplicates\n")
	b.WriteString("- If `logs/` or `sessions/` subdirectories exist (assistant-mode layout), review recent entries there\n\n")

	// Phase 2: Gather.
	b.WriteString("## Phase 2 — Gather recent signal\n\n")
	b.WriteString("Look for new information worth persisting. Sources in rough priority order:\n\n")
	b.WriteString("1. **Daily logs** (`logs/YYYY/MM/YYYY-MM-DD.md`) if present — these are the append-only stream\n")
	b.WriteString("2. **Existing memories that drifted** — facts that contradict something you see in the codebase now\n")
	b.WriteString("3. **Transcript search** — if you need specific context, grep the JSONL transcripts for narrow terms:\n")
	b.WriteString(fmt.Sprintf("   `grep -rn \"<narrow term>\" %s/ --include=\"*.jsonl\" | tail -50`\n\n", transcriptDir))
	b.WriteString("Don't exhaustively read transcripts. Look only for things you already suspect matter.\n\n")

	// Phase 3: Consolidate.
	b.WriteString("## Phase 3 — Consolidate\n\n")
	b.WriteString("For each thing worth remembering, write or update a memory file at the top level ")
	b.WriteString("of the memory directory. Use the memory file format and type conventions from ")
	b.WriteString("your system prompt's auto-memory section — it's the source of truth for what ")
	b.WriteString("to save, how to structure it, and what NOT to save.\n\n")
	b.WriteString("Focus on:\n")
	b.WriteString("- Merging new signal into existing topic files rather than creating near-duplicates\n")
	b.WriteString("- Converting relative dates to absolute dates so they remain interpretable after time passes\n")
	b.WriteString("- Deleting contradicted facts — if today's investigation disproves an old memory, fix it at the source\n\n")

	// Phase 4: Prune and index.
	b.WriteString("## Phase 4 — Prune and index\n\n")
	b.WriteString(fmt.Sprintf("Update `%s` so it stays under %d lines AND under ~25KB. ", entrypointName, maxEntrypointLines))
	b.WriteString("It's an **index**, not a dump — each entry should be one line under ~150 characters: ")
	b.WriteString("`- [Title](file.md) — one-line hook`. Never write memory content directly into it.\n\n")
	b.WriteString("- Remove pointers to memories that are now stale, wrong, or superseded\n")
	b.WriteString("- Demote verbose entries: if an index line is over ~200 chars, it's carrying content ")
	b.WriteString("that belongs in the topic file — shorten the line, move the detail\n")
	b.WriteString("- Add pointers to newly important memories\n")
	b.WriteString("- Resolve contradictions — if two files disagree, fix the wrong one\n\n")

	b.WriteString("---\n\n")
	b.WriteString("Return a brief summary of what you consolidated, updated, or pruned. ")
	b.WriteString("If nothing changed (memories are already tight), say so.")

	if extra != "" {
		b.WriteString(fmt.Sprintf("\n\n%s", extra))
	}

	return b.String()
}
