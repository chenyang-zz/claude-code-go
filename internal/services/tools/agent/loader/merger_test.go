package loader

import (
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/agent"
	"github.com/stretchr/testify/assert"
)

func TestMergeAgentDefinitions_Empty(t *testing.T) {
	result := MergeAgentDefinitions()
	assert.Empty(t, result)
}

func TestMergeAgentDefinitions_SingleSource(t *testing.T) {
	src := []agent.Definition{
		{AgentType: "explore", Source: "built-in"},
		{AgentType: "verify", Source: "built-in"},
	}
	result := MergeAgentDefinitions(src)
	requireLen(t, result, 2)
}

func TestMergeAgentDefinitions_NoCollision(t *testing.T) {
	builtIn := []agent.Definition{
		{AgentType: "explore", Source: "built-in"},
	}
	user := []agent.Definition{
		{AgentType: "my-agent", Source: "userSettings"},
	}
	result := MergeAgentDefinitions(builtIn, user)
	requireLen(t, result, 2)

	byType := make(map[string]string)
	for _, d := range result {
		byType[d.AgentType] = d.Source
	}
	assert.Equal(t, "built-in", byType["explore"])
	assert.Equal(t, "userSettings", byType["my-agent"])
}

func TestMergeAgentDefinitions_UserOverridesBuiltIn(t *testing.T) {
	builtIn := []agent.Definition{
		{AgentType: "explore", Source: "built-in", WhenToUse: "Built-in explore"},
	}
	user := []agent.Definition{
		{AgentType: "explore", Source: "userSettings", WhenToUse: "User explore"},
	}
	result := MergeAgentDefinitions(builtIn, user)
	requireLen(t, result, 1)
	assert.Equal(t, "userSettings", result[0].Source)
	assert.Equal(t, "User explore", result[0].WhenToUse)
}

func TestMergeAgentDefinitions_ProjectOverridesUser(t *testing.T) {
	user := []agent.Definition{
		{AgentType: "search", Source: "userSettings", WhenToUse: "User search"},
	}
	project := []agent.Definition{
		{AgentType: "search", Source: "projectSettings", WhenToUse: "Project search"},
	}
	result := MergeAgentDefinitions(user, project)
	requireLen(t, result, 1)
	assert.Equal(t, "projectSettings", result[0].Source)
	assert.Equal(t, "Project search", result[0].WhenToUse)
}

func TestMergeAgentDefinitions_ProjectOverridesBuiltInAndUser(t *testing.T) {
	builtIn := []agent.Definition{
		{AgentType: "explore", Source: "built-in"},
		{AgentType: "verify", Source: "built-in"},
	}
	user := []agent.Definition{
		{AgentType: "explore", Source: "userSettings"},
	}
	project := []agent.Definition{
		{AgentType: "explore", Source: "projectSettings"},
		{AgentType: "search", Source: "projectSettings"},
	}
	result := MergeAgentDefinitions(builtIn, user, project)
	requireLen(t, result, 3)

	byType := make(map[string]string)
	for _, d := range result {
		byType[d.AgentType] = d.Source
	}
	assert.Equal(t, "projectSettings", byType["explore"])
	assert.Equal(t, "built-in", byType["verify"])
	assert.Equal(t, "projectSettings", byType["search"])
}

func TestMergeAgentDefinitions_LaterSourceWins(t *testing.T) {
	// Explicit ordering: first → second → third
	first := []agent.Definition{
		{AgentType: "a", Source: "first"},
	}
	second := []agent.Definition{
		{AgentType: "a", Source: "second"},
	}
	third := []agent.Definition{
		{AgentType: "a", Source: "third"},
	}
	result := MergeAgentDefinitions(first, second, third)
	requireLen(t, result, 1)
	assert.Equal(t, "third", result[0].Source)
}

func TestMergeAgentDefinitions_NilAndEmptySlices(t *testing.T) {
	builtIn := []agent.Definition{
		{AgentType: "explore", Source: "built-in"},
	}
	result := MergeAgentDefinitions(nil, builtIn, nil, []agent.Definition{})
	requireLen(t, result, 1)
	assert.Equal(t, "explore", result[0].AgentType)
}

func requireLen(t *testing.T, result []agent.Definition, n int) {
	t.Helper()
	if len(result) != n {
		t.Fatalf("expected %d definitions, got %d", n, len(result))
	}
}
