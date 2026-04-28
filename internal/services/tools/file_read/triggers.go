package file_read

import (
	"os"
	"path/filepath"
	"strings"
)

// ReadPathTriggerMeta stores the minimal trigger metadata emitted after a successful text read.
type ReadPathTriggerMeta struct {
	// NestedMemoryAttachmentTriggers mirrors the read path list used by memory attachment follow-up logic.
	NestedMemoryAttachmentTriggers []string
	// DynamicSkillDirTriggers stores discovered skill directories associated with the read path.
	DynamicSkillDirTriggers []string
}

// buildReadPathTriggerMeta computes lightweight trigger metadata for one successful text read.
func buildReadPathTriggerMeta(filePath string, workingDir string) ReadPathTriggerMeta {
	meta := ReadPathTriggerMeta{
		NestedMemoryAttachmentTriggers: []string{filePath},
	}
	if isEnvTruthy(os.Getenv("CLAUDE_CODE_SIMPLE")) {
		return meta
	}
	meta.DynamicSkillDirTriggers = discoverSkillDirsForPath(filePath, workingDir)
	return meta
}

// discoverSkillDirsForPath walks up from the file directory and returns ancestor directories that contain SKILL.md.
func discoverSkillDirsForPath(filePath string, workingDir string) []string {
	start := filepath.Clean(filepath.Dir(filePath))
	if start == "." || start == string(filepath.Separator) && filePath == "" {
		return nil
	}

	stop := filepath.Clean(workingDir)
	if stop == "." || stop == "" {
		stop = string(filepath.Separator)
	}

	seen := make(map[string]struct{})
	dirs := make([]string, 0)
	current := start
	for {
		skillFile := filepath.Join(current, "SKILL.md")
		if info, err := os.Stat(skillFile); err == nil && !info.IsDir() {
			if _, ok := seen[current]; !ok {
				seen[current] = struct{}{}
				dirs = append(dirs, current)
			}
		}

		if current == stop || current == string(filepath.Separator) {
			break
		}
		if !isSubpathOrSame(current, stop) && current != start {
			break
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	return dirs
}

// isSubpathOrSame reports whether pathValue is the same as base or nested below it.
func isSubpathOrSame(pathValue string, base string) bool {
	pathClean := filepath.Clean(pathValue)
	baseClean := filepath.Clean(base)
	if pathClean == baseClean {
		return true
	}
	prefix := baseClean + string(filepath.Separator)
	return strings.HasPrefix(pathClean, prefix)
}

// isEnvTruthy normalizes common truthy environment flag values.
func isEnvTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
