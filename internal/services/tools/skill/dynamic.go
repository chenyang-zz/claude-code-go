package skill

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// dynamicSkillDirs records every .claude/skills/ path that has already been
// checked (hit or miss). This avoids repeating the same stat on every file
// operation when the directory doesn't exist (the common case).
var dynamicSkillDirs = make(map[string]struct{})

// dynamicSkills stores skills discovered during the session (via file-path
// walking). Keyed by skill name; deeper paths override shallower ones during
// addSkillDirectories.
var dynamicSkills = make(map[string]*Skill)

// skillsLoadedCallbacks stores callbacks registered via OnDynamicSkillsLoaded.
// Keyed by unique ID so callbacks can be cleanly unsubscribed.
var skillsLoadedCallbacks = make(map[int]func())
var nextCallbackID int

// OnDynamicSkillsLoaded registers a callback that is called whenever dynamic
// skills are loaded or conditional skills are activated. Used by other modules
// to clear caches when the skill set changes. Returns an unsubscribe function.
func OnDynamicSkillsLoaded(callback func()) func() {
	id := nextCallbackID
	nextCallbackID++
	skillsLoadedCallbacks[id] = callback
	return func() {
		delete(skillsLoadedCallbacks, id)
	}
}

// emitSkillsLoaded notifies all registered listeners that the dynamic skill
// set has changed.
func emitSkillsLoaded() {
	for _, cb := range skillsLoadedCallbacks {
		cb()
	}
}

// DiscoverSkillDirsForPaths walks up from each filePath toward cwd, looking
// for .claude/skills/ directories. Directories already checked are skipped.
// Returns discovered directories sorted deepest-first (skills closer to the
// file take precedence).
func DiscoverSkillDirsForPaths(filePaths []string, cwd string) []string {
	resolvedCwd := strings.TrimRight(cwd, string(filepath.Separator))
	var newDirs []string

	for _, filePath := range filePaths {
		// Start from the file's parent directory.
		currentDir := filepath.Dir(filePath)

		// Walk up to (but not including) cwd. Use prefix+separator to avoid
		// matching /project-backup when cwd is /project.
		cwdPrefix := resolvedCwd + string(filepath.Separator)
		for strings.HasPrefix(currentDir, cwdPrefix) {
			skillDir := filepath.Join(currentDir, ".claude", "skills")

			if _, checked := dynamicSkillDirs[skillDir]; !checked {
				dynamicSkillDirs[skillDir] = struct{}{}
				if info, err := os.Stat(skillDir); err == nil && info.IsDir() {
					newDirs = append(newDirs, skillDir)
				}
			}

			parent := filepath.Dir(currentDir)
			if parent == currentDir {
				break
			}
			currentDir = parent
		}
	}

	// Sort deepest first so skills closer to the file take precedence.
	sort.Slice(newDirs, func(i, j int) bool {
		return strings.Count(newDirs[i], string(filepath.Separator)) >
			strings.Count(newDirs[j], string(filepath.Separator))
	})

	return newDirs
}

// AddSkillDirectories loads skills from the given directories and merges them
// into the dynamic skills map. Directories are processed in reverse order
// (shallowest first) so deeper paths override shallower ones.
func AddSkillDirectories(dirs []string) ([]*Skill, error) {
	if len(dirs) == 0 {
		return nil, nil
	}

	previousNames := make(map[string]struct{})
	for name := range dynamicSkills {
		previousNames[name] = struct{}{}
	}

	var allLoaded []*Skill
	for _, dir := range dirs {
		skills, loadErrors, _ := loadSkillsFromDir(dir, "projectSettings")
		for _, le := range loadErrors {
			logger.DebugCF("skill", "dynamic skill load error", map[string]any{
				"skill": le.Name,
				"error": le.Error,
			})
		}
		allLoaded = append(allLoaded, skills...)
	}

	// Process in reverse (shallower first) so deeper paths override.
	for i := len(allLoaded) - 1; i >= 0; i-- {
		s := allLoaded[i]
		dynamicSkills[s.name] = s
	}

	var addedNames []string
	for name := range dynamicSkills {
		if _, existed := previousNames[name]; !existed {
			addedNames = append(addedNames, name)
		}
	}

	if len(addedNames) > 0 {
		logger.DebugCF("skill", "dynamically discovered skills", map[string]any{
			"count":        len(addedNames),
			"directories":  len(dirs),
			"added_skills": addedNames,
		})
		emitSkillsLoaded()
	}

	return allLoaded, nil
}

// GetDynamicSkills returns all dynamically discovered skills (including those
// activated from conditional storage).
func GetDynamicSkills() []*Skill {
	result := make([]*Skill, 0, len(dynamicSkills))
	for _, s := range dynamicSkills {
		result = append(result, s)
	}
	return result
}

// ClearDynamicSkills resets all dynamic discovery and conditional skill state.
// For testing only.
func ClearDynamicSkills() {
	dynamicSkillDirs = make(map[string]struct{})
	dynamicSkills = make(map[string]*Skill)
	skillsLoadedCallbacks = make(map[int]func())
	nextCallbackID = 0
	ClearConditionalSkills()
}
