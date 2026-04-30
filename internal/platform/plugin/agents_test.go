package plugin

import (
	"path/filepath"
	"testing"
)

func TestExtractAgents_EmptyPath(t *testing.T) {
	plugin := &LoadedPlugin{Name: "test", Path: "/tmp/test"}
	agents, err := ExtractAgents(plugin)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if agents != nil {
		t.Errorf("expected nil, got %v", agents)
	}
}

func TestExtractAgents_WithAgentsDir(t *testing.T) {
	tmpDir := t.TempDir()
	pluginPath := filepath.Join(tmpDir, "test-plugin")
	agentsPath := filepath.Join(pluginPath, "agents")
	mustMkdirAll(t, agentsPath)
	writeFile(t, filepath.Join(agentsPath, "reviewer.md"), `---
name: Code Reviewer
description: Reviews code for quality
tools: "*"
model: sonnet
background: true
memory: project
max-turns: 10
---
# Code Reviewer
You are a code reviewer.`)

	plugin := &LoadedPlugin{
		Name:       "test-plugin",
		Path:       pluginPath,
		AgentsPath: agentsPath,
	}

	agents, err := ExtractAgents(plugin)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}

	a := agents[0]
	if a.AgentType != "test-plugin:Code Reviewer" {
		t.Errorf("expected 'test-plugin:Code Reviewer', got %q", a.AgentType)
	}
	if a.DisplayName != "Code Reviewer" {
		t.Errorf("expected 'Code Reviewer', got %q", a.DisplayName)
	}
	if a.Description != "Reviews code for quality" {
		t.Errorf("expected description, got %q", a.Description)
	}
	if a.Tools != "*" {
		t.Errorf("expected '*', got %q", a.Tools)
	}
	if a.Model != "sonnet" {
		t.Errorf("expected 'sonnet', got %q", a.Model)
	}
	if !a.Background {
		t.Error("expected Background to be true")
	}
	if a.Memory != "project" {
		t.Errorf("expected 'project', got %q", a.Memory)
	}
	if a.MaxTurns != 10 {
		t.Errorf("expected 10, got %d", a.MaxTurns)
	}
}

func TestExtractAgents_WithNestedAgents(t *testing.T) {
	tmpDir := t.TempDir()
	pluginPath := filepath.Join(tmpDir, "test-plugin")
	agentsPath := filepath.Join(pluginPath, "agents")
	nestedPath := filepath.Join(agentsPath, "qa")
	mustMkdirAll(t, nestedPath)
	writeFile(t, filepath.Join(agentsPath, "top.md"), "# Top Agent")
	writeFile(t, filepath.Join(nestedPath, "tester.md"), `---
name: QA Tester
---
# Tester`)

	plugin := &LoadedPlugin{
		Name:       "test-plugin",
		Path:       pluginPath,
		AgentsPath: agentsPath,
	}

	agents, err := ExtractAgents(plugin)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(agents))
	}

	names := make(map[string]bool)
	for _, a := range agents {
		names[a.AgentType] = true
	}
	if !names["test-plugin:top"] {
		t.Error("expected 'test-plugin:top'")
	}
	if !names["test-plugin:qa:QA Tester"] {
		t.Error("expected 'test-plugin:qa:QA Tester'")
	}
}

func TestExtractAgents_NameOverride(t *testing.T) {
	tmpDir := t.TempDir()
	pluginPath := filepath.Join(tmpDir, "test-plugin")
	agentsPath := filepath.Join(pluginPath, "agents")
	mustMkdirAll(t, agentsPath)
	writeFile(t, filepath.Join(agentsPath, "generic.md"), `---
name: Custom Agent Name
---
# Content`)

	plugin := &LoadedPlugin{
		Name:       "test-plugin",
		Path:       pluginPath,
		AgentsPath: agentsPath,
	}

	agents, err := ExtractAgents(plugin)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	// Name from frontmatter should be used for AgentType, not the filename.
	if agents[0].AgentType != "test-plugin:Custom Agent Name" {
		t.Errorf("expected 'test-plugin:Custom Agent Name', got %q", agents[0].AgentType)
	}
}

func TestExtractAgents_DescriptionFallback(t *testing.T) {
	tmpDir := t.TempDir()
	pluginPath := filepath.Join(tmpDir, "test-plugin")
	agentsPath := filepath.Join(pluginPath, "agents")
	mustMkdirAll(t, agentsPath)
	writeFile(t, filepath.Join(agentsPath, "no-desc.md"), `---
name: No Desc
---
# Heading

Fallback description paragraph.`)

	plugin := &LoadedPlugin{
		Name:       "test-plugin",
		Path:       pluginPath,
		AgentsPath: agentsPath,
	}

	agents, err := ExtractAgents(plugin)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].Description != "Fallback description paragraph." {
		t.Errorf("expected fallback description, got %q", agents[0].Description)
	}
}

func TestExtractAgents_MaxTurnsParsing(t *testing.T) {
	tmpDir := t.TempDir()
	pluginPath := filepath.Join(tmpDir, "test-plugin")
	agentsPath := filepath.Join(pluginPath, "agents")
	mustMkdirAll(t, agentsPath)
	writeFile(t, filepath.Join(agentsPath, "limited.md"), `---
name: Limited
max-turns: 5
---
# Limited`)

	plugin := &LoadedPlugin{
		Name:       "test-plugin",
		Path:       pluginPath,
		AgentsPath: agentsPath,
	}

	agents, err := ExtractAgents(plugin)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].MaxTurns != 5 {
		t.Errorf("expected MaxTurns 5, got %d", agents[0].MaxTurns)
	}
}

func TestBuildAgentName(t *testing.T) {
	tests := []struct {
		filePath     string
		baseDir      string
		pluginName   string
		overrideName string
		expected     string
	}{
		{"/p/agents/reviewer.md", "/p/agents", "my-plugin", "", "my-plugin:reviewer"},
		{"/p/agents/qa/tester.md", "/p/agents", "my-plugin", "", "my-plugin:qa:tester"},
		{"/p/agents/gen.md", "/p/agents", "my-plugin", "Custom", "my-plugin:Custom"},
	}
	for _, tt := range tests {
		result := buildAgentName(tt.filePath, tt.baseDir, tt.pluginName, tt.overrideName)
		if result != tt.expected {
			t.Errorf("buildAgentName(%q, %q, %q, %q) = %q, want %q",
				tt.filePath, tt.baseDir, tt.pluginName, tt.overrideName, result, tt.expected)
		}
	}
}
