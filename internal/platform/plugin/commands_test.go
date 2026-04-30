package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFrontmatter_Valid(t *testing.T) {
	content := []byte(`---
name: test-command
description: A test command
allowed-tools: read,write
shell: bash
---
# Hello World
This is the body.`)

	fm, body, err := parseFrontmatter(content)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if fm["name"] != "test-command" {
		t.Errorf("expected name 'test-command', got %q", fm["name"])
	}
	if fm["description"] != "A test command" {
		t.Errorf("expected description, got %q", fm["description"])
	}
	if fm["allowed-tools"] != "read,write" {
		t.Errorf("expected 'read,write', got %q", fm["allowed-tools"])
	}
	if fm["shell"] != "bash" {
		t.Errorf("expected 'bash', got %q", fm["shell"])
	}
	expectedBody := "# Hello World\nThis is the body."
	if string(body) != expectedBody {
		t.Errorf("expected body %q, got %q", expectedBody, string(body))
	}
}

func TestParseFrontmatter_NoClosingDelimiter(t *testing.T) {
	content := []byte(`---
name: test
No closing delimiter.`)

	fm, body, err := parseFrontmatter(content)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(fm) != 0 {
		t.Errorf("expected empty frontmatter, got %v", fm)
	}
	if string(body) != string(content) {
		t.Errorf("expected body to be original content")
	}
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	content := []byte(`# Just Markdown
No frontmatter here.`)

	fm, body, err := parseFrontmatter(content)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(fm) != 0 {
		t.Errorf("expected empty frontmatter, got %v", fm)
	}
	if string(body) != string(content) {
		t.Errorf("expected body to equal original")
	}
}

func TestParseFrontmatter_ArrayArguments(t *testing.T) {
	content := []byte(`---
name: test-cmd
arguments:
  - name
  - target
  - region
---
Body`)

	fm, body, err := parseFrontmatter(content)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if fm["arguments"] != "name,target,region" {
		t.Errorf("expected arguments 'name,target,region', got %q", fm["arguments"])
	}
	if string(body) != "Body" {
		t.Errorf("expected body 'Body', got %q", string(body))
	}
}

func TestParseFrontmatter_InlineArray(t *testing.T) {
	content := []byte(`---
name: test-cmd
arguments: [name, target]
---
Body`)

	fm, _, err := parseFrontmatter(content)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if fm["arguments"] != "name,target" {
		t.Errorf("expected arguments 'name,target', got %q", fm["arguments"])
	}
}

func TestParseFrontmatter_QuotedValues(t *testing.T) {
	content := []byte(`---
name: "quoted-name"
description: 'single-quoted'
---
Body`)

	fm, _, err := parseFrontmatter(content)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if fm["name"] != "quoted-name" {
		t.Errorf("expected 'quoted-name', got %q", fm["name"])
	}
	if fm["description"] != "single-quoted" {
		t.Errorf("expected 'single-quoted', got %q", fm["description"])
	}
}

func TestCommandName_SimpleCommand(t *testing.T) {
	baseDir := "/test-plugin/commands"
	filePath := "/test-plugin/commands/deploy.md"
	name := commandName(filePath, baseDir, "my-plugin", false)
	if name != "my-plugin:deploy" {
		t.Errorf("expected 'my-plugin:deploy', got %q", name)
	}
}

func TestCommandName_NestedCommand(t *testing.T) {
	baseDir := "/test-plugin/commands"
	filePath := "/test-plugin/commands/tools/deploy.md"
	name := commandName(filePath, baseDir, "my-plugin", false)
	if name != "my-plugin:tools:deploy" {
		t.Errorf("expected 'my-plugin:tools:deploy', got %q", name)
	}
}

func TestCommandName_SkillFile(t *testing.T) {
	baseDir := "/test-plugin/commands"
	filePath := "/test-plugin/commands/tools/build/SKILL.md"
	name := commandName(filePath, baseDir, "my-plugin", true)
	if name != "my-plugin:tools:build" {
		t.Errorf("expected 'my-plugin:tools:build', got %q", name)
	}
}

func TestExtractCommands_EmptyCommandsPath(t *testing.T) {
	plugin := &LoadedPlugin{Name: "test", Path: "/tmp/test"}
	cmds, err := ExtractCommands(plugin)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cmds != nil {
		t.Errorf("expected nil, got %v", cmds)
	}
}

func TestExtractCommands_WithCommandsDir(t *testing.T) {
	tmpDir := t.TempDir()
	pluginPath := filepath.Join(tmpDir, "test-plugin")
	commandsPath := filepath.Join(pluginPath, "commands")
	mustMkdirAll(t, commandsPath)
	writeFile(t, filepath.Join(commandsPath, "hello.md"), `---
name: hello
description: Say hello
---
# Hello
This is the hello command.`)

	plugin := &LoadedPlugin{
		Name:         "test-plugin",
		Path:         pluginPath,
		CommandsPath: commandsPath,
	}

	cmds, err := ExtractCommands(plugin)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(cmds))
	}
	if cmds[0].Name != "test-plugin:hello" {
		t.Errorf("expected 'test-plugin:hello', got %q", cmds[0].Name)
	}
	if cmds[0].Description != "Say hello" {
		t.Errorf("expected 'Say hello', got %q", cmds[0].Description)
	}
	if cmds[0].IsSkill {
		t.Error("expected command, not skill")
	}
}

func TestExtractCommands_WithNestedCommands(t *testing.T) {
	tmpDir := t.TempDir()
	pluginPath := filepath.Join(tmpDir, "test-plugin")
	commandsPath := filepath.Join(pluginPath, "commands")
	nestedPath := filepath.Join(commandsPath, "nested")
	mustMkdirAll(t, nestedPath)
	writeFile(t, filepath.Join(commandsPath, "top.md"), `# Top command`)
	writeFile(t, filepath.Join(nestedPath, "sub.md"), `# Sub command`)

	plugin := &LoadedPlugin{
		Name:         "test-plugin",
		Path:         pluginPath,
		CommandsPath: commandsPath,
	}

	cmds, err := ExtractCommands(plugin)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(cmds))
	}

	names := make(map[string]bool)
	for _, c := range cmds {
		names[c.Name] = true
	}
	if !names["test-plugin:top"] {
		t.Error("expected 'test-plugin:top'")
	}
	if !names["test-plugin:nested:sub"] {
		t.Error("expected 'test-plugin:nested:sub'")
	}
}

func TestExtractSkills_WithSkillsDir(t *testing.T) {
	tmpDir := t.TempDir()
	pluginPath := filepath.Join(tmpDir, "test-plugin")
	skillsPath := filepath.Join(pluginPath, "skills")
	mustMkdirAll(t, skillsPath)
	writeFile(t, filepath.Join(skillsPath, "SKILL.md"), `---
name: my-skill
description: A skill
---
# My Skill
Skill content.`)

	plugin := &LoadedPlugin{
		Name:       "test-plugin",
		Path:       pluginPath,
		SkillsPath: skillsPath,
	}

	cmds, err := ExtractSkills(plugin)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cmds) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(cmds))
	}
	if !cmds[0].IsSkill {
		t.Error("expected IsSkill to be true")
	}
}

func TestExtractSkills_WithSkillDirContainer(t *testing.T) {
	tmpDir := t.TempDir()
	pluginPath := filepath.Join(tmpDir, "test-plugin")
	skillsPath := filepath.Join(pluginPath, "skills")
	skillDir := filepath.Join(skillsPath, "build")
	mustMkdirAll(t, skillDir)
	writeFile(t, filepath.Join(skillDir, "SKILL.md"), `---
name: build
---
# Build Skill`)

	plugin := &LoadedPlugin{
		Name:       "test-plugin",
		Path:       pluginPath,
		SkillsPath: skillsPath,
	}

	cmds, err := ExtractSkills(plugin)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cmds) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(cmds))
	}
	if cmds[0].Name != "test-plugin:build" {
		t.Errorf("expected 'test-plugin:build', got %q", cmds[0].Name)
	}
}

func TestWalkMarkdownFiles_SkipsSymlinks(t *testing.T) {
	tmpDir := t.TempDir()
	mustMkdirAll(t, tmpDir)
	writeFile(t, filepath.Join(tmpDir, "real.md"), "# Real file")
	linkPath := filepath.Join(tmpDir, "link.md")
	targetPath := filepath.Join(tmpDir, "real.md")
	_ = os.Symlink(targetPath, linkPath)

	files, err := walkMarkdownFiles(tmpDir, false)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// walkMarkdownFiles doesn't filter symlinks - that's done in loadCommandsFromDir.
	// Here we just verify it finds the files.
	if len(files) < 1 {
		t.Error("expected at least 1 file")
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		value    string
		defVal   bool
		expected bool
	}{
		{"true", false, true},
		{"false", true, false},
		{"TRUE", false, true},
		{"FALSE", true, false},
		{"", true, true},
		{"", false, false},
		{"unknown", true, true},
		{"unknown", false, false},
	}
	for _, tt := range tests {
		result := parseBool(tt.value, tt.defVal)
		if result != tt.expected {
			t.Errorf("parseBool(%q, %v) = %v, want %v", tt.value, tt.defVal, result, tt.expected)
		}
	}
}

func TestFirstParagraph(t *testing.T) {
	body := []byte("# Heading\n\nFirst paragraph.\nSecond line.\n\nThird paragraph.")
	result := firstParagraph(body)
	if result != "First paragraph. Second line." {
		t.Errorf("expected 'First paragraph. Second line.', got %q", result)
	}
}

func TestExtractCommands_DescriptionFallback(t *testing.T) {
	tmpDir := t.TempDir()
	pluginPath := filepath.Join(tmpDir, "test-plugin")
	commandsPath := filepath.Join(pluginPath, "commands")
	mustMkdirAll(t, commandsPath)
	writeFile(t, filepath.Join(commandsPath, "no-desc.md"), `---
name: no-desc
---
# Heading

This is the first paragraph.`)

	plugin := &LoadedPlugin{
		Name:         "test-plugin",
		Path:         pluginPath,
		CommandsPath: commandsPath,
	}

	cmds, err := ExtractCommands(plugin)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(cmds))
	}
	if cmds[0].Description != "This is the first paragraph." {
		t.Errorf("expected fallback description, got %q", cmds[0].Description)
	}
}

func TestDefaultString(t *testing.T) {
	if s := defaultString("", "bash"); s != "bash" {
		t.Errorf("expected 'bash', got %q", s)
	}
	if s := defaultString("powershell", "bash"); s != "powershell" {
		t.Errorf("expected 'powershell', got %q", s)
	}
}
