package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadUserAgents_MissingDir(t *testing.T) {
	home := t.TempDir()
	defs, errs, err := LoadUserAgents(home)
	require.NoError(t, err)
	assert.Empty(t, defs)
	assert.Empty(t, errs)
}

func TestLoadUserAgents_EmptyDir(t *testing.T) {
	home := t.TempDir()
	agentsDir := filepath.Join(home, ".claude", "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	defs, errs, err := LoadUserAgents(home)
	require.NoError(t, err)
	assert.Empty(t, defs)
	assert.Empty(t, errs)
}

func TestLoadUserAgents_SingleAgent(t *testing.T) {
	home := t.TempDir()
	agentsDir := filepath.Join(home, ".claude", "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	content := "---\nname: global-search\ndescription: Global search agent\n---\nYou are a global search agent."
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "search.md"), []byte(content), 0644))

	defs, errs, err := LoadUserAgents(home)
	require.NoError(t, err)
	assert.Empty(t, errs)
	require.Len(t, defs, 1)
	assert.Equal(t, "global-search", defs[0].AgentType)
	assert.Equal(t, "Global search agent", defs[0].WhenToUse)
	assert.Equal(t, "userSettings", defs[0].Source)
	assert.Equal(t, "You are a global search agent.", defs[0].SystemPrompt)
}

func TestLoadUserAgents_MultipleAgents(t *testing.T) {
	home := t.TempDir()
	agentsDir := filepath.Join(home, ".claude", "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	require.NoError(t, os.WriteFile(
		filepath.Join(agentsDir, "agent1.md"),
		[]byte("---\nname: user-agent-1\ndescription: First user agent\n---\nPrompt 1"),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(agentsDir, "agent2.md"),
		[]byte("---\nname: user-agent-2\ndescription: Second user agent\n---\nPrompt 2"),
		0644,
	))

	defs, errs, err := LoadUserAgents(home)
	require.NoError(t, err)
	assert.Empty(t, errs)
	require.Len(t, defs, 2)

	types := make([]string, len(defs))
	for i, d := range defs {
		types[i] = d.AgentType
	}
	assert.Contains(t, types, "user-agent-1")
	assert.Contains(t, types, "user-agent-2")
}

func TestLoadUserAgents_NonAgentMarkdown(t *testing.T) {
	home := t.TempDir()
	agentsDir := filepath.Join(home, ".claude", "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	require.NoError(t, os.WriteFile(
		filepath.Join(agentsDir, "readme.md"),
		[]byte("# Readme\n\nThis is not an agent."),
		0644,
	))

	defs, errs, err := LoadUserAgents(home)
	require.NoError(t, err)
	assert.Empty(t, defs)
	assert.Empty(t, errs)
}

func TestLoadUserAgents_InvalidAgent(t *testing.T) {
	home := t.TempDir()
	agentsDir := filepath.Join(home, ".claude", "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	require.NoError(t, os.WriteFile(
		filepath.Join(agentsDir, "broken.md"),
		[]byte("---\nname: broken-agent\n---\nPrompt"),
		0644,
	))

	defs, errs, err := LoadUserAgents(home)
	require.NoError(t, err)
	assert.Empty(t, defs)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error, "missing required 'description'")
}

func TestLoadUserAgents_RecursiveSubdir(t *testing.T) {
	home := t.TempDir()
	agentsDir := filepath.Join(home, ".claude", "agents", "sub")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	require.NoError(t, os.WriteFile(
		filepath.Join(agentsDir, "nested.md"),
		[]byte("---\nname: nested-user-agent\ndescription: Nested user agent\n---\nPrompt"),
		0644,
	))

	defs, errs, err := LoadUserAgents(home)
	require.NoError(t, err)
	assert.Empty(t, errs)
	require.Len(t, defs, 1)
	assert.Equal(t, "nested-user-agent", defs[0].AgentType)
}
