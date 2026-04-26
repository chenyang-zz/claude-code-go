package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadManagedAgents_MissingDir(t *testing.T) {
	managedDir := t.TempDir()
	defs, errs, err := LoadManagedAgents(managedDir)
	require.NoError(t, err)
	assert.Empty(t, defs)
	assert.Empty(t, errs)
}

func TestLoadManagedAgents_EmptyDir(t *testing.T) {
	managedDir := t.TempDir()
	agentsDir := filepath.Join(managedDir, ".claude", "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	defs, errs, err := LoadManagedAgents(managedDir)
	require.NoError(t, err)
	assert.Empty(t, defs)
	assert.Empty(t, errs)
}

func TestLoadManagedAgents_SingleAgent(t *testing.T) {
	managedDir := t.TempDir()
	agentsDir := filepath.Join(managedDir, ".claude", "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	content := "---\nname: compliance-agent\ndescription: A compliance specialist\n---\nYou are a compliance specialist."
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "compliance.md"), []byte(content), 0644))

	defs, errs, err := LoadManagedAgents(managedDir)
	require.NoError(t, err)
	assert.Empty(t, errs)
	require.Len(t, defs, 1)
	assert.Equal(t, "compliance-agent", defs[0].AgentType)
	assert.Equal(t, "A compliance specialist", defs[0].WhenToUse)
	assert.Equal(t, "policySettings", defs[0].Source)
	assert.Equal(t, "You are a compliance specialist.", defs[0].SystemPrompt)
}

func TestLoadManagedAgents_InvalidAgent(t *testing.T) {
	managedDir := t.TempDir()
	agentsDir := filepath.Join(managedDir, ".claude", "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	require.NoError(t, os.WriteFile(
		filepath.Join(agentsDir, "broken.md"),
		[]byte("---\nname: broken-agent\n---\nPrompt"),
		0644,
	))

	defs, errs, err := LoadManagedAgents(managedDir)
	require.NoError(t, err)
	assert.Empty(t, defs)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error, "missing required 'description'")
}

func TestLoadLocalAgents_MissingDir(t *testing.T) {
	projectDir := t.TempDir()
	defs, errs, err := LoadLocalAgents(projectDir)
	require.NoError(t, err)
	assert.Empty(t, defs)
	assert.Empty(t, errs)
}

func TestLoadLocalAgents_SingleAgent(t *testing.T) {
	projectDir := t.TempDir()
	agentsDir := filepath.Join(projectDir, ".claude", "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))

	content := "---\nname: local-agent\ndescription: A local specialist\n---\nYou are a local specialist."
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "local.md"), []byte(content), 0644))

	defs, errs, err := LoadLocalAgents(projectDir)
	require.NoError(t, err)
	assert.Empty(t, errs)
	require.Len(t, defs, 1)
	assert.Equal(t, "local-agent", defs[0].AgentType)
	assert.Equal(t, "A local specialist", defs[0].WhenToUse)
	assert.Equal(t, "localSettings", defs[0].Source)
}
