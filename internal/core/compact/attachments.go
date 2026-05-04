package compact

import (
	"sort"
)

// Post-compact attachment constants, matching the TS source values in
// src/services/compact/compact.ts (lines 122-130).
const (
	PostCompactMaxFilesToRestore = 5
	PostCompactTokenBudget       = 50000
	PostCompactMaxTokensPerFile  = 5000
	PostCompactMaxTokensPerSkill = 5000
	PostCompactSkillsTokenBudget = 25000
)

// PostCompactAttachment represents a context attachment that is injected after
// compaction so the model retains access to important information (recently
// read files, plan content, invoked skills).
type PostCompactAttachment struct {
	Type    string // "file", "plan", "skill"
	Name    string // file name or skill name
	Content string // attachment content
}

// FileReadState tracks the last-read content and timestamp for a file.
// Corresponds to the TS readFileState record value.
type FileReadState struct {
	Content   string
	Timestamp int64
}

// SkillInfo holds metadata and content for a previously invoked skill.
type SkillInfo struct {
	Name      string
	Path      string
	Content   string
	InvokedAt int64
}

// CreatePostCompactFileAttachments builds attachment messages for recently
// accessed files so the model can reference them after compaction without
// re-reading.
//
// Files are sorted by recency (most recent first), limited to maxFiles, and
// each file's content is truncated to maxTokensPerFile tokens. The total
// token budget is capped at tokenBudget.
//
// TS equivalent: createPostCompactFileAttachments (compact.ts ~line 1415).
func CreatePostCompactFileAttachments(
	readFileState map[string]FileReadState,
	maxFiles int,
	tokenBudget int,
) []PostCompactAttachment {
	type entry struct {
		filename string
		FileReadState
	}

	entries := make([]entry, 0, len(readFileState))
	for filename, state := range readFileState {
		entries = append(entries, entry{filename: filename, FileReadState: state})
	}

	// Sort by timestamp descending (most recent first).
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp > entries[j].Timestamp
	})

	if len(entries) > maxFiles {
		entries = entries[:maxFiles]
	}

	var results []PostCompactAttachment
	usedTokens := 0

	for _, e := range entries {
		content := e.Content
		contentTokens := EstimateTokensForText(content)
		if contentTokens > PostCompactMaxTokensPerFile {
			content = truncateString(content, PostCompactMaxTokensPerFile)
			contentTokens = PostCompactMaxTokensPerFile
		}

		if usedTokens+contentTokens > tokenBudget {
			continue
		}
		usedTokens += contentTokens

		results = append(results, PostCompactAttachment{
			Type:    "file",
			Name:    e.filename,
			Content: content,
		})
	}

	return results
}

// CreatePlanAttachment creates a plan-type attachment if the provided plan
// content is non-empty. Returns nil otherwise.
//
// TS equivalent: createPlanAttachmentIfNeeded (compact.ts ~line 1470).
func CreatePlanAttachment(planContent string, planFilePath string) *PostCompactAttachment {
	if planContent == "" {
		return nil
	}
	return &PostCompactAttachment{
		Type:    "plan",
		Name:    planFilePath,
		Content: planContent,
	}
}

// CreateSkillAttachment builds a single attachment that aggregates all invoked
// skills. Skills are sorted by InvokedAt descending (most recent first), each
// truncated to maxTokensPerSkill, and the total is capped at totalTokenBudget.
//
// Returns nil if no skills survive the budget filter.
//
// TS equivalent: createSkillAttachmentIfNeeded (compact.ts ~line 1494).
func CreateSkillAttachment(
	skills []SkillInfo,
	maxTokensPerSkill int,
	totalTokenBudget int,
) *PostCompactAttachment {
	if len(skills) == 0 {
		return nil
	}

	// Sort by InvokedAt descending (most recent first).
	sort.Slice(skills, func(i, j int) bool {
		return skills[i].InvokedAt > skills[j].InvokedAt
	})

	usedTokens := 0
	type trimmedSkill struct {
		Name    string
		Path    string
		Content string
	}
	var trimmed []trimmedSkill

	for _, s := range skills {
		content := s.Content
		tokens := EstimateTokensForText(content)
		if tokens > maxTokensPerSkill {
			content = truncateString(content, maxTokensPerSkill)
			tokens = maxTokensPerSkill
		}
		if usedTokens+tokens > totalTokenBudget {
			continue
		}
		usedTokens += tokens
		trimmed = append(trimmed, trimmedSkill{
			Name:    s.Name,
			Path:    s.Path,
			Content: content,
		})
	}

	if len(trimmed) == 0 {
		return nil
	}

	// Concatenate all skill contents into a single attachment body.
	combined := ""
	for i, s := range trimmed {
		if i > 0 {
			combined += "\n\n"
		}
		combined += "--- " + s.Name + " (" + s.Path + ") ---\n" + s.Content
	}

	return &PostCompactAttachment{
		Type:    "skill",
		Name:    "invoked_skills",
		Content: combined,
	}
}

// truncateString cuts a string to approximately the given token budget.
// Since EstimateTokensForText uses len(text)/4, we cap at maxTokens*4 runes.
func truncateString(s string, maxTokens int) string {
	maxChars := maxTokens * defaultBytesPerToken
	if len(s) <= maxChars {
		return s
	}
	return s[:maxChars]
}
