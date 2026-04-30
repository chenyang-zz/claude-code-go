package skill

import (
	"path/filepath"
	"strings"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// conditionalSkills stores skills with paths frontmatter that haven't been
// activated yet. Skills remain here until a matching file path causes activation.
var conditionalSkills = make(map[string]*Skill)

// activatedConditionalSkillNames tracks skill names that have been permanently
// activated (survives conditional skill cache clears within a session).
var activatedConditionalSkillNames = make(map[string]struct{})

// SeparateConditionalSkills splits skills into unconditional and conditional
// groups. Skills with non-empty paths are placed into the conditional store
// and NOT returned. Skills already permanently activated skip the conditional
// store and are returned as unconditional. The returned slice contains only
// skills that should be registered immediately.
func SeparateConditionalSkills(skills []*Skill) []*Skill {
	var unconditional []*Skill
	var newConditional []*Skill

	for _, s := range skills {
		if len(s.paths) > 0 {
			if _, activated := activatedConditionalSkillNames[s.name]; activated {
				unconditional = append(unconditional, s)
			} else {
				newConditional = append(newConditional, s)
			}
		} else {
			unconditional = append(unconditional, s)
		}
	}

	for _, s := range newConditional {
		conditionalSkills[s.name] = s
	}

	if len(newConditional) > 0 {
		names := make([]string, len(newConditional))
		for i, s := range newConditional {
			names[i] = s.name
		}
		logger.DebugCF("skill", "conditional skills stored", map[string]any{
			"count":  len(newConditional),
			"skills": names,
		})
	}

	return unconditional
}

// ActivateConditionalSkillsForPaths checks each conditional skill's path
// patterns against the given filePaths using gitignore-style glob matching.
// Skills whose patterns match are moved from conditional storage to dynamic
// storage and their names are permanently recorded so they remain activated
// even after cache clears. Returns the names of newly activated skills.
//
// Path patterns support *, ?, and ** (cross-directory) wildcards and are
// matched against file paths relative to cwd.
func ActivateConditionalSkillsForPaths(filePaths []string, cwd string) []string {
	if len(conditionalSkills) == 0 {
		return nil
	}

	var activated []string

	for name, s := range conditionalSkills {
		if len(s.paths) == 0 {
			continue
		}

		for _, filePath := range filePaths {
			relativePath := filePath
			if filepath.IsAbs(filePath) {
				var err error
				relativePath, err = filepath.Rel(cwd, filePath)
				if err != nil || strings.HasPrefix(relativePath, "..") {
					continue
				}
			}
			// Guard: empty paths and paths escaping the base are not
			// matchable against cwd-relative patterns.
			if relativePath == "" || strings.HasPrefix(relativePath, "..") {
				continue
			}

			if matchSkillPath(relativePath, s.paths) {
				dynamicSkills[name] = s
				delete(conditionalSkills, name)
				activatedConditionalSkillNames[name] = struct{}{}
				activated = append(activated, name)
				logger.DebugCF("skill", "activated conditional skill", map[string]any{
					"name":         name,
					"matched_path": relativePath,
				})
				break
			}
		}
	}

	if len(activated) > 0 {
		emitSkillsLoaded()
	}

	return activated
}

// GetConditionalSkillCount returns the number of pending conditional skills.
func GetConditionalSkillCount() int {
	return len(conditionalSkills)
}

// ClearConditionalSkills resets all conditional skill state. For testing only.
func ClearConditionalSkills() {
	conditionalSkills = make(map[string]*Skill)
	activatedConditionalSkillNames = make(map[string]struct{})
}

// --- gitignore-style path matching ---

// matchSkillPath tests whether filePath matches any of the given path patterns
// using gitignore-style glob semantics. Patterns may contain * (single-segment
// wildcard), ? (single char), and ** (cross-directory wildcard).
func matchSkillPath(filePath string, patterns []string) bool {
	segments := splitPathSegments(filePath)
	for _, pattern := range patterns {
		patternSegs := splitPathSegments(pattern)
		if matchGlobSegments(segments, patternSegs) {
			return true
		}
	}
	return false
}

// splitPathSegments converts a slash-separated path into segments.
func splitPathSegments(path string) []string {
	if path == "" {
		return nil
	}
	return strings.Split(filepath.ToSlash(path), "/")
}

// matchGlobSegments applies * / ? / ** matching across path segments following
// the same semantics as the ignore library's gitignore-style matching.
func matchGlobSegments(pathSegs, patternSegs []string) bool {
	if len(patternSegs) == 0 {
		return len(pathSegs) == 0
	}

	if patternSegs[0] == "**" {
		// ** matches zero or more path segments
		if matchGlobSegments(pathSegs, patternSegs[1:]) {
			return true
		}
		if len(pathSegs) == 0 {
			return false
		}
		return matchGlobSegments(pathSegs[1:], patternSegs)
	}

	if len(pathSegs) == 0 {
		return false
	}

	if !matchSegment(pathSegs[0], patternSegs[0]) {
		return false
	}

	return matchGlobSegments(pathSegs[1:], patternSegs[1:])
}

// matchSegment matches a single path segment against a pattern segment using
// the standard * (any sequence) and ? (single char) wildcards.
func matchSegment(segment, pattern string) bool {
	// Fast path: exact match
	if segment == pattern {
		return true
	}

	si, pi := 0, 0
	for pi < len(pattern) {
		if pattern[pi] == '*' {
			// * matches any sequence (including empty) within a single segment
			if pi == len(pattern)-1 {
				return true // trailing * matches rest
			}
			// Advance past * and try to match the remaining pattern at each
			// position in the segment.
			pi++
			for si < len(segment) {
				if matchSegment(segment[si:], pattern[pi:]) {
					return true
				}
				si++
			}
			return matchSegment(segment[si:], pattern[pi:])
		}
		if si >= len(segment) {
			return false
		}
		if pattern[pi] != '?' && pattern[pi] != segment[si] {
			return false
		}
		si++
		pi++
	}

	return si == len(segment)
}
