package analytics

import (
	"testing"
)

func TestSanitizeToolNameBuiltin(t *testing.T) {
	result := SanitizeToolName("Bash")
	if result != "Bash" {
		t.Errorf("expected Bash, got %s", result)
	}
}

func TestSanitizeToolNameMCP(t *testing.T) {
	result := SanitizeToolName("mcp__slack__read_channel")
	if result != "mcp_tool" {
		t.Errorf("expected mcp_tool, got %s", result)
	}
}

func TestSanitizeToolNameMCPServerOnly(t *testing.T) {
	result := SanitizeToolName("mcp__filesystem")
	if result != "mcp_tool" {
		t.Errorf("expected mcp_tool, got %s", result)
	}
}

func TestToolDetailsLoggingDefaultDisabled(t *testing.T) {
	if ToolDetailsLoggingEnabled() {
		t.Error("expected disabled by default")
	}
}

func TestAnalyticsToolDetailsLoggingClaudeAIProxy(t *testing.T) {
	if !AnalyticsToolDetailsLoggingEnabled("claudeai-proxy", "") {
		t.Error("claudeai-proxy should return true")
	}
}

func TestAnalyticsToolDetailsLoggingOfficialURL(t *testing.T) {
	if !AnalyticsToolDetailsLoggingEnabled("", "https://mcp.googleapis.com/test") {
		t.Error("official MCP URL should return true")
	}
}

func TestAnalyticsToolDetailsLoggingCustom(t *testing.T) {
	if AnalyticsToolDetailsLoggingEnabled("", "https://custom.example.com") {
		t.Error("custom URL should return false")
	}
}

func TestAnalyticsToolDetailsLoggingEmpty(t *testing.T) {
	if AnalyticsToolDetailsLoggingEnabled("", "") {
		t.Error("empty server type and URL should return false")
	}
}

func TestExtractMCPToolDetails(t *testing.T) {
	d := ExtractMCPToolDetails("mcp__slack__read_channel")
	if d == nil {
		t.Fatal("expected non-nil result")
	}
	if d["mcpServerName"] != "slack" {
		t.Errorf("expected slack, got %s", d["mcpServerName"])
	}
	if d["mcpToolName"] != "read_channel" {
		t.Errorf("expected read_channel, got %s", d["mcpToolName"])
	}
}

func TestExtractMCPToolDetailsBuiltin(t *testing.T) {
	d := ExtractMCPToolDetails("Bash")
	if d != nil {
		t.Error("expected nil for non-MCP tool")
	}
}

func TestExtractMCPToolDetailsMultiUnderscore(t *testing.T) {
	d := ExtractMCPToolDetails("mcp__server__deep__nested__tool")
	if d == nil {
		t.Fatal("expected non-nil")
	}
	if d["mcpServerName"] != "server" {
		t.Errorf("expected server, got %s", d["mcpServerName"])
	}
	if d["mcpToolName"] != "deep__nested__tool" {
		t.Errorf("expected deep__nested__tool, got %s", d["mcpToolName"])
	}
}

func TestExtractMCPToolDetailsNoTool(t *testing.T) {
	d := ExtractMCPToolDetails("mcp__server")
	if d != nil {
		t.Error("expected nil for mcp__server (no tool part)")
	}
}

func TestExtractMCPToolDetailsEmptyPrefix(t *testing.T) {
	d := ExtractMCPToolDetails("mcp______")
	if d != nil {
		t.Error("expected nil for malformed name")
	}
}

func TestMCPToolDetailsGatePass(t *testing.T) {
	d := MCPToolDetails("mcp__slack__read", "claudeai-proxy", "")
	if d == nil {
		t.Fatal("expected non-nil for claudeai-proxy")
	}
	if d["mcpServerName"] != "slack" {
		t.Errorf("expected slack, got %s", d["mcpServerName"])
	}
}

func TestMCPToolDetailsGateBlock(t *testing.T) {
	d := MCPToolDetails("mcp__slack__read", "", "https://custom.example.com")
	if d != nil {
		t.Error("expected nil for custom MCP without gate")
	}
}

func TestExtractSkillName(t *testing.T) {
	name := ExtractSkillName("Skill", map[string]any{"skill": "test-skill"})
	if name != "test-skill" {
		t.Errorf("expected test-skill, got %s", name)
	}
}

func TestExtractSkillNameWrongTool(t *testing.T) {
	name := ExtractSkillName("Bash", map[string]any{"skill": "test"})
	if name != "" {
		t.Errorf("expected empty, got %s", name)
	}
}

func TestExtractSkillNameMissingInput(t *testing.T) {
	name := ExtractSkillName("Skill", "not a map")
	if name != "" {
		t.Errorf("expected empty, got %s", name)
	}
}

func TestExtractSkillNameMissingSkillKey(t *testing.T) {
	name := ExtractSkillName("Skill", map[string]any{"other": "value"})
	if name != "" {
		t.Errorf("expected empty, got %s", name)
	}
}

func TestFileExtensionForAnalytics(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"file.go", "go"},
		{"file.ts", "ts"},
		{"file.test.tsx", "tsx"},
		{"Makefile", ""},
		{"archive.tar.gz", "gz"},
		{"path/to/file.py", "py"},
		{"file.UPPER", "upper"},
		{".hidden", ""},
	}
	for _, tt := range tests {
		result := FileExtensionForAnalytics(tt.path)
		if result != tt.expected {
			t.Errorf("FileExtensionForAnalytics(%q) = %q, want %q", tt.path, result, tt.expected)
		}
	}
}

func TestFileExtensionForAnalyticsLongExt(t *testing.T) {
	result := FileExtensionForAnalytics("file.hash-abcd1234567890")
	if result != "other" {
		t.Errorf("expected other for long extension, got %s", result)
	}
}

func TestFileExtensionForAnalyticsNoExt(t *testing.T) {
	result := FileExtensionForAnalytics("README")
	if result != "" {
		t.Errorf("expected empty for README, got %s", result)
	}
}

func TestFileExtensionsFromBashCommand(t *testing.T) {
	result := FileExtensionsFromBashCommand("cat file.go", "")
	if result != "go" {
		t.Errorf("expected go, got %s", result)
	}
}

func TestFileExtensionsFromBashCommandCompound(t *testing.T) {
	result := FileExtensionsFromBashCommand("cat a.go && mv b.ts /tmp/", "")
	if result != "go,ts" {
		t.Errorf("expected go,ts, got %s", result)
	}
}

func TestFileExtensionsFromBashCommandNoMatch(t *testing.T) {
	result := FileExtensionsFromBashCommand("echo hello", "")
	if result != "" {
		t.Errorf("expected empty, got %s", result)
	}
}

func TestFileExtensionsFromBashCommandFlagArgs(t *testing.T) {
	result := FileExtensionsFromBashCommand("grep -r 'pattern' dir/", "")
	if result == "" {
		return // no file extensions expected
	}
	// Just make sure it doesn't crash on flag args
}

func TestFileExtensionsFromBashCommandSimulatedEdit(t *testing.T) {
	result := FileExtensionsFromBashCommand("", "/path/file.ts")
	if result != "ts" {
		t.Errorf("expected ts, got %s", result)
	}
}

func TestFileExtensionsFromBashCommandWithSed(t *testing.T) {
	result := FileExtensionsFromBashCommand("sed -i 's/foo/bar/g' file.txt", "file.txt")
	if result != "txt" {
		t.Errorf("expected txt, got %s", result)
	}
}

func TestTruncateToolInputStringShort(t *testing.T) {
	result := truncateToolInputValue("short", 0)
	if result != "short" {
		t.Errorf("expected short, got %v", result)
	}
}

func TestTruncateToolInputNestedLimit(t *testing.T) {
	result := truncateToolInputValue(map[string]any{"deep": map[string]any{"deeper": map[string]any{"deepest": "value"}}}, 0)
	r, ok := result.(map[string]any)
	if !ok {
		t.Fatal("expected map")
	}
	_, ok = r["deep"]
	if !ok {
		t.Error("expected deep key")
	}
}

func TestExtractToolInputForTelemetryDisabled(t *testing.T) {
	result := ExtractToolInputForTelemetry(map[string]any{"key": "value"})
	if result != "" {
		t.Errorf("expected empty when disabled, got %s", result)
	}
}

func TestSanitizeToolNameEmpty(t *testing.T) {
	result := SanitizeToolName("")
	if result != "" {
		t.Errorf("expected empty, got %s", result)
	}
}

func TestSanitizeToolNameBareMCP(t *testing.T) {
	result := SanitizeToolName("mcp__")
	if result != "mcp_tool" {
		t.Errorf("expected mcp_tool, got %s", result)
	}
}
