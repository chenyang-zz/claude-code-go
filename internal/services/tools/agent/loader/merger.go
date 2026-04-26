package loader

import (
	"sort"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
)

// MergeAgentDefinitions merges multiple agent definition slices by source
// priority. Each subsequent slice may override agents defined in earlier
// slices when agent types collide.
//
// The typical priority order aligned with TypeScript getActiveAgentsFromList is:
//   built-in → plugin → user → project → flag → managed
//
// Callers should pass sources in ascending priority order (lowest first).
// For batch-139 the relevant chain is: built-in → user → project.
func MergeAgentDefinitions(sources ...[]agent.Definition) []agent.Definition {
	merged := make(map[string]agent.Definition)
	for _, source := range sources {
		for _, def := range source {
			merged[def.AgentType] = def
		}
	}

	// Produce a stable, sorted result for test repeatability.
	keys := make([]string, 0, len(merged))
	for k := range merged {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	result := make([]agent.Definition, 0, len(merged))
	for _, k := range keys {
		result = append(result, merged[k])
	}
	return result
}
