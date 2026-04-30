package tool_search

import (
	"context"
	"strings"
	"testing"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
)

// stubTool implements coretool.Tool for test setups.
type stubTool struct {
	name        string
	description string
}

func (s stubTool) Name() string                         { return s.name }
func (s stubTool) Description() string                   { return s.description }
func (s stubTool) InputSchema() coretool.InputSchema     { return coretool.InputSchema{} }
func (s stubTool) IsReadOnly() bool                      { return true }
func (s stubTool) IsConcurrencySafe() bool               { return true }
func (s stubTool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	return coretool.Result{}, nil
}

// setupTestRegistry creates a tool.Registry with known test tools.
func setupTestRegistry() coretool.Registry {
	r := coretool.NewMemoryRegistry()
	r.Register(stubTool{name: "FileRead", description: "Read files from the filesystem"})
	r.Register(stubTool{name: "FileWrite", description: "Write content to files"})
	r.Register(stubTool{name: "Bash", description: "Execute shell commands"})
	r.Register(stubTool{name: "Glob", description: "Find files matching a pattern"})
	r.Register(stubTool{name: "Grep", description: "Search file contents using regular expressions"})
	r.Register(stubTool{name: "WebFetch", description: "Fetch content from a URL"})
	r.Register(stubTool{name: "MCP__github__list_issues", description: "List GitHub issues"})
	r.Register(stubTool{name: "MCP__slack__send_message", description: "Send a Slack message"})
	return r
}

func TestTool_Name(t *testing.T) {
	tool := NewTool()
	if tool.Name() != Name {
		t.Errorf("Name() = %q, want %q", tool.Name(), Name)
	}
}

func TestTool_Description(t *testing.T) {
	tool := NewTool()
	if tool.Description() != toolDescription {
		t.Errorf("Description() = %q, want %q", tool.Description(), toolDescription)
	}
}

func TestTool_InputSchema(t *testing.T) {
	tool := NewTool()
	schema := tool.InputSchema()
	if _, ok := schema.Properties["query"]; !ok {
		t.Error("InputSchema should have 'query' property")
	}
	if _, ok := schema.Properties["max_results"]; !ok {
		t.Error("InputSchema should have 'max_results' property")
	}
}

func TestTool_IsReadOnly(t *testing.T) {
	tool := NewTool()
	if !tool.IsReadOnly() {
		t.Error("IsReadOnly() should be true")
	}
}

func TestTool_IsConcurrencySafe(t *testing.T) {
	tool := NewTool()
	if !tool.IsConcurrencySafe() {
		t.Error("IsConcurrencySafe() should be true")
	}
}

func TestTool_Invoke_NilReceiver(t *testing.T) {
	var tool *Tool
	_, err := tool.Invoke(context.Background(), coretool.Call{})
	if err == nil {
		t.Error("expected error for nil receiver")
	}
}

func TestTool_Invoke_NilRegistry(t *testing.T) {
	SharedRegistry = nil
	tool := NewTool()
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"query": "test"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error for nil registry")
	}
	if !strings.Contains(result.Error, "registry not initialized") {
		t.Errorf("unexpected error message: %s", result.Error)
	}
}

func TestTool_Invoke_EmptyQuery(t *testing.T) {
	SharedRegistry = setupTestRegistry()
	tool := NewTool()
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"query": ""},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error for empty query")
	}
}

func TestTool_Invoke_WhitespaceQuery(t *testing.T) {
	SharedRegistry = setupTestRegistry()
	tool := NewTool()
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"query": "   "},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected error for whitespace-only query")
	}
}

func TestTool_Invoke_SelectDirectMatch(t *testing.T) {
	SharedRegistry = setupTestRegistry()
	tool := NewTool()
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"query": "select:Bash"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
	data := result.Meta["data"].(Output)
	if len(data.Matches) != 1 {
		t.Errorf("expected 1 match, got %d: %v", len(data.Matches), data.Matches)
	}
	if len(data.Matches) > 0 && data.Matches[0] != "Bash" {
		t.Errorf("expected Bash, got %s", data.Matches[0])
	}
}

func TestTool_Invoke_SelectCaseInsensitive(t *testing.T) {
	SharedRegistry = setupTestRegistry()
	tool := NewTool()
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"query": "select:bash"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
	data := result.Meta["data"].(Output)
	if len(data.Matches) != 1 {
		t.Errorf("expected 1 match, got %d", len(data.Matches))
	}
}

func TestTool_Invoke_SelectNotFound(t *testing.T) {
	SharedRegistry = setupTestRegistry()
	tool := NewTool()
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"query": "select:NonExistent"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
	data := result.Meta["data"].(Output)
	if len(data.Matches) != 0 {
		t.Errorf("expected 0 matches, got %d: %v", len(data.Matches), data.Matches)
	}
}

func TestTool_Invoke_SelectMultiMatch(t *testing.T) {
	SharedRegistry = setupTestRegistry()
	tool := NewTool()
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"query": "select:Bash,Glob,Grep"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
	data := result.Meta["data"].(Output)
	if len(data.Matches) != 3 {
		t.Errorf("expected 3 matches, got %d: %v", len(data.Matches), data.Matches)
	}
}

func TestTool_Invoke_KeywordSearchByName(t *testing.T) {
	SharedRegistry = setupTestRegistry()
	tool := NewTool()
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"query": "file"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
	data := result.Meta["data"].(Output)
	if len(data.Matches) == 0 {
		t.Error("expected at least 1 match for 'file'")
	}
	// FileRead and FileWrite should match by name.
	// Glob and Grep may also match by description.
	hasFileRead := false
	hasFileWrite := false
	for _, m := range data.Matches {
		if m == "FileRead" {
			hasFileRead = true
		}
		if m == "FileWrite" {
			hasFileWrite = true
		}
	}
	if !hasFileRead {
		t.Error("expected FileRead to match 'file'")
	}
	if !hasFileWrite {
		t.Error("expected FileWrite to match 'file'")
	}
}

func TestTool_Invoke_KeywordSearchByDescription(t *testing.T) {
	SharedRegistry = setupTestRegistry()
	tool := NewTool()
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"query": "shell"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
	data := result.Meta["data"].(Output)
	if len(data.Matches) == 0 {
		t.Error("expected at least 1 match for 'shell'")
	}
	hasBash := false
	for _, m := range data.Matches {
		if m == "Bash" {
			hasBash = true
		}
	}
	if !hasBash {
		t.Error("expected Bash to match 'shell' (description contains 'shell commands')")
	}
}

func TestTool_Invoke_KeywordSearchNoMatch(t *testing.T) {
	SharedRegistry = setupTestRegistry()
	tool := NewTool()
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"query": "nonexistent_xyz"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
	data := result.Meta["data"].(Output)
	if len(data.Matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(data.Matches))
	}
}

func TestTool_Invoke_MaxResultsDefault(t *testing.T) {
	SharedRegistry = setupTestRegistry()
	tool := NewTool()
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"query": "e"}, // matches File*, Grep, WebFetch, ...
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
	data := result.Meta["data"].(Output)
	if len(data.Matches) > 5 {
		t.Errorf("default max_results should limit to 5, got %d", len(data.Matches))
	}
}

func TestTool_Invoke_MaxResultsCustom(t *testing.T) {
	SharedRegistry = setupTestRegistry()
	tool := NewTool()
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"query": "e", "max_results": float64(2)},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
	data := result.Meta["data"].(Output)
	if len(data.Matches) > 2 {
		t.Errorf("max_results should limit to 2, got %d", len(data.Matches))
	}
}

func TestTool_Invoke_ExactNameMatchShowsInKeywordSearch(t *testing.T) {
	SharedRegistry = setupTestRegistry()
	tool := NewTool()
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"query": "Bash"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
	data := result.Meta["data"].(Output)
	hasBash := false
	for _, m := range data.Matches {
		if m == "Bash" {
			hasBash = true
		}
	}
	if !hasBash {
		t.Error("keyword search for 'Bash' should match Bash tool")
	}
}

func TestTool_Invoke_TotalDeferredTools(t *testing.T) {
	SharedRegistry = setupTestRegistry()
	tool := NewTool()
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"query": "file"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
	data := result.Meta["data"].(Output)
	if data.TotalDeferredTools == 0 {
		t.Error("total_deferred_tools should not be zero")
	}
}

func TestTool_Invoke_SchemaInvalidType(t *testing.T) {
	SharedRegistry = setupTestRegistry()
	tool := NewTool()
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"query": 123},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == "" {
		t.Error("expected schema validation error for non-string query")
	}
}

func TestTool_Invoke_OutputFormat(t *testing.T) {
	SharedRegistry = setupTestRegistry()
	tool := NewTool()
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"query": "file"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected result error: %s", result.Error)
	}
	// Result.Output should be a JSON string
	if len(result.Output) == 0 || result.Output[0] != '{' {
		t.Errorf("Output should be JSON object string, got: %s", result.Output)
	}
}
