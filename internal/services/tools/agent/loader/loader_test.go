package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadCustomAgents_MissingDir(t *testing.T) {
	defs, errs, err := LoadCustomAgents(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, defs)
	assert.Empty(t, errs)
}

func TestLoadCustomAgents_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, ".claude", "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	defs, errs, err := LoadCustomAgents(dir)
	require.NoError(t, err)
	assert.Empty(t, defs)
	assert.Empty(t, errs)
}

func TestLoadCustomAgents_SingleAgent(t *testing.T) {
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, ".claude", "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	content := "---\nname: search-agent\ndescription: A search specialist\n---\nYou are a search specialist."
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "search.md"), []byte(content), 0644))

	defs, errs, err := LoadCustomAgents(dir)
	require.NoError(t, err)
	assert.Empty(t, errs)
	require.Len(t, defs, 1)
	assert.Equal(t, "search-agent", defs[0].AgentType)
	assert.Equal(t, "A search specialist", defs[0].WhenToUse)
	assert.Equal(t, "projectSettings", defs[0].Source)
	assert.Equal(t, "You are a search specialist.", defs[0].SystemPrompt)
}

func TestLoadCustomAgents_MultipleAgents(t *testing.T) {
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, ".claude", "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	require.NoError(t, os.WriteFile(
		filepath.Join(agentsDir, "agent1.md"),
		[]byte("---\nname: agent-1\ndescription: First agent\n---\nPrompt 1"),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(agentsDir, "agent2.md"),
		[]byte("---\nname: agent-2\ndescription: Second agent\n---\nPrompt 2"),
		0644,
	))

	defs, errs, err := LoadCustomAgents(dir)
	require.NoError(t, err)
	assert.Empty(t, errs)
	require.Len(t, defs, 2)

	types := make([]string, len(defs))
	for i, d := range defs {
		types[i] = d.AgentType
	}
	assert.Contains(t, types, "agent-1")
	assert.Contains(t, types, "agent-2")
}

func TestLoadCustomAgents_NonAgentMarkdown(t *testing.T) {
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, ".claude", "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	// File without 'name' field — should be silently skipped
	require.NoError(t, os.WriteFile(
		filepath.Join(agentsDir, "readme.md"),
		[]byte("# Readme\n\nThis is not an agent."),
		0644,
	))

	defs, errs, err := LoadCustomAgents(dir)
	require.NoError(t, err)
	assert.Empty(t, defs)
	assert.Empty(t, errs)
}

func TestLoadCustomAgents_InvalidAgent(t *testing.T) {
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, ".claude", "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	// Agent with name but no description — should be recorded as error
	require.NoError(t, os.WriteFile(
		filepath.Join(agentsDir, "broken.md"),
		[]byte("---\nname: broken-agent\n---\nPrompt"),
		0644,
	))

	defs, errs, err := LoadCustomAgents(dir)
	require.NoError(t, err)
	assert.Empty(t, defs)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error, "missing required 'description'")
}

func TestLoadCustomAgents_SingleFailureDoesNotBlockOthers(t *testing.T) {
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, ".claude", "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	require.NoError(t, os.WriteFile(
		filepath.Join(agentsDir, "good.md"),
		[]byte("---\nname: good-agent\ndescription: Works fine\n---\nPrompt"),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(agentsDir, "broken.md"),
		[]byte("---\nname: broken-agent\n---\nPrompt"),
		0644,
	))

	defs, errs, err := LoadCustomAgents(dir)
	require.NoError(t, err)
	require.Len(t, defs, 1)
	assert.Equal(t, "good-agent", defs[0].AgentType)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error, "missing required 'description'")
}

func TestLoadCustomAgents_RecursiveSubdir(t *testing.T) {
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, ".claude", "agents", "sub")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	require.NoError(t, os.WriteFile(
		filepath.Join(agentsDir, "nested.md"),
		[]byte("---\nname: nested-agent\ndescription: Nested agent\n---\nPrompt"),
		0644,
	))

	defs, errs, err := LoadCustomAgents(dir)
	require.NoError(t, err)
	assert.Empty(t, errs)
	require.Len(t, defs, 1)
	assert.Equal(t, "nested-agent", defs[0].AgentType)
}

func TestLoadCustomAgents_NonMdFilesIgnored(t *testing.T) {
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, ".claude", "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	require.NoError(t, os.WriteFile(
		filepath.Join(agentsDir, "notes.txt"),
		[]byte("---\nname: txt-agent\ndescription: Text file\n---\nPrompt"),
		0644,
	))

	defs, errs, err := LoadCustomAgents(dir)
	require.NoError(t, err)
	assert.Empty(t, defs)
	assert.Empty(t, errs)
}
