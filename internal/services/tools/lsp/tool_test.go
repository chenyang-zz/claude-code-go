package lsp

import (
	"encoding/json"
	"testing"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	platformlsp "github.com/sheepzhao/claude-code-go/internal/platform/lsp"
)

func TestNewTool(t *testing.T) {
	tool := NewTool()
	if tool == nil {
		t.Fatal("NewTool returned nil")
	}
}

func TestName(t *testing.T) {
	tool := NewTool()
	if tool.Name() != Name {
		t.Errorf("Name() = %q, want %q", tool.Name(), Name)
	}
}

func TestDescription(t *testing.T) {
	tool := NewTool()
	desc := tool.Description()
	if len(desc) == 0 {
		t.Error("Description() returned empty string")
	}
	if desc != toolDescription {
		t.Errorf("Description() returned unexpected value")
	}
}

func TestIsReadOnly(t *testing.T) {
	tool := NewTool()
	if !tool.IsReadOnly() {
		t.Error("LSP tool should be read-only")
	}
}

func TestIsConcurrencySafe(t *testing.T) {
	tool := NewTool()
	if !tool.IsConcurrencySafe() {
		t.Error("LSP tool should be concurrency-safe")
	}
}

func TestInputSchema(t *testing.T) {
	tool := NewTool()
	schema := tool.InputSchema()

	required := []string{"operation", "filePath", "line", "character"}
	for _, field := range required {
		prop, ok := schema.Properties[field]
		if !ok {
			t.Errorf("InputSchema missing required field %q", field)
		}
		if !prop.Required {
			t.Errorf("field %q should be required", field)
		}
	}
}

func TestGetMethodAndParams(t *testing.T) {
	tests := []struct {
		operation    string
		expectedMethod string
	}{
		{"goToDefinition", "textDocument/definition"},
		{"findReferences", "textDocument/references"},
		{"hover", "textDocument/hover"},
		{"documentSymbol", "textDocument/documentSymbol"},
		{"workspaceSymbol", "workspace/symbol"},
		{"goToImplementation", "textDocument/implementation"},
		{"prepareCallHierarchy", "textDocument/prepareCallHierarchy"},
		{"incomingCalls", "textDocument/prepareCallHierarchy"},
		{"outgoingCalls", "textDocument/prepareCallHierarchy"},
	}

	for _, tc := range tests {
		method, params := getMethodAndParams(tc.operation, "/test/file.go", 10, 5)
		if method != tc.expectedMethod {
			t.Errorf("getMethodAndParams(%q) method = %q, want %q", tc.operation, method, tc.expectedMethod)
		}
		// Verify params contain expected fields.
		paramsJSON, _ := json.Marshal(params)
		if len(paramsJSON) == 0 {
			t.Errorf("getMethodAndParams(%q) returned nil params", tc.operation)
		}
	}
}

func TestGetMethodAndParams_PositionConversion(t *testing.T) {
	// 1-based input should be converted to 0-based LSP positions.
	method, params := getMethodAndParams("hover", "/test/file.go", 10, 5)
	if method != "textDocument/hover" {
		t.Fatalf("unexpected method: %s", method)
	}

	// Extract position from params.
	paramsJSON, _ := json.Marshal(params)
	var parsed map[string]any
	json.Unmarshal(paramsJSON, &parsed)

	pos, ok := parsed["position"].(map[string]any)
	if !ok {
		t.Fatal("params missing position field")
	}
	if line, ok := pos["line"].(float64); !ok || line != 9 {
		t.Errorf("expected line=9 (0-based from 10), got %v", pos["line"])
	}
	if char, ok := pos["character"].(float64); !ok || char != 4 {
		t.Errorf("expected character=4 (0-based from 5), got %v", pos["character"])
	}
}

func TestFormatGoToDefinitionResult_Single(t *testing.T) {
	loc := platformlsp.Location{
		URI: "file:///home/user/project/main.go",
		Range: platformlsp.Range{
			Start: platformlsp.Position{Line: 9, Character: 4},
			End:   platformlsp.Position{Line: 9, Character: 10},
		},
	}
	raw, _ := json.Marshal(loc)

	result, count, files := formatGoToDefinitionResult(raw, "/home/user/project")
	if count != 1 {
		t.Errorf("expected count=1, got %d", count)
	}
	if files != 1 {
		t.Errorf("expected files=1, got %d", files)
	}
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
}

func TestFormatGoToDefinitionResult_Array(t *testing.T) {
	locs := []platformlsp.Location{
		{URI: "file:///home/user/project/main.go", Range: platformlsp.Range{
			Start: platformlsp.Position{Line: 9, Character: 4},
			End:   platformlsp.Position{Line: 9, Character: 10},
		}},
		{URI: "file:///home/user/project/lib.go", Range: platformlsp.Range{
			Start: platformlsp.Position{Line: 4, Character: 2},
			End:   platformlsp.Position{Line: 4, Character: 8},
		}},
	}
	raw, _ := json.Marshal(locs)

	result, count, files := formatGoToDefinitionResult(raw, "/home/user/project")
	if count != 2 {
		t.Errorf("expected count=2, got %d", count)
	}
	if files != 2 {
		t.Errorf("expected files=2, got %d", files)
	}
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
}

func TestFormatGoToDefinitionResult_Empty(t *testing.T) {
	raw, _ := json.Marshal([]platformlsp.Location{})
	result, count, files := formatGoToDefinitionResult(raw, "")
	if count != 0 || files != 0 {
		t.Errorf("expected 0/0, got %d/%d", count, files)
	}
	_ = result
}

func TestFormatFindReferencesResult(t *testing.T) {
	locs := []platformlsp.Location{
		{URI: "file:///home/user/project/main.go", Range: platformlsp.Range{
			Start: platformlsp.Position{Line: 4, Character: 2},
			End:   platformlsp.Position{Line: 4, Character: 8},
		}},
		{URI: "file:///home/user/project/main.go", Range: platformlsp.Range{
			Start: platformlsp.Position{Line: 10, Character: 5},
			End:   platformlsp.Position{Line: 10, Character: 11},
		}},
		{URI: "file:///home/user/project/lib.go", Range: platformlsp.Range{
			Start: platformlsp.Position{Line: 2, Character: 3},
			End:   platformlsp.Position{Line: 2, Character: 9},
		}},
	}
	raw, _ := json.Marshal(locs)

	result, count, files := formatFindReferencesResult(raw, "/home/user/project")
	if count != 3 {
		t.Errorf("expected count=3, got %d", count)
	}
	if files != 2 {
		t.Errorf("expected files=2, got %d", files)
	}
	_ = result
}

func TestFormatHoverResult(t *testing.T) {
	hover := platformlsp.Hover{
		Contents: platformlsp.MarkupContent{
			Kind:  "markdown",
			Value: "func main() — entry point",
		},
	}
	raw, _ := json.Marshal(hover)

	result, count, files := formatHoverResult(raw)
	if count != 1 || files != 1 {
		t.Errorf("expected 1/1, got %d/%d", count, files)
	}
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
}

func TestFormatDocumentSymbolResult_DocumentSymbol(t *testing.T) {
	syms := []platformlsp.DocumentSymbol{
		{
			Name: "main", Kind: platformlsp.SymbolKindFunction,
			Range:          platformlsp.Range{Start: platformlsp.Position{Line: 0, Character: 0}, End: platformlsp.Position{Line: 5, Character: 1}},
			SelectionRange: platformlsp.Range{Start: platformlsp.Position{Line: 0, Character: 0}, End: platformlsp.Position{Line: 5, Character: 1}},
		},
	}
	raw, _ := json.Marshal(syms)

	result, count, files := formatDocumentSymbolResult(raw, "")
	if count != 1 {
		t.Errorf("expected count=1, got %d", count)
	}
	if files != 1 {
		t.Errorf("expected files=1, got %d", files)
	}
	_ = result
}

func TestFormatDocumentSymbolResult_Hierarchical(t *testing.T) {
	syms := []platformlsp.DocumentSymbol{
		{
			Name: "MyStruct", Kind: platformlsp.SymbolKindStruct,
			Range:          platformlsp.Range{Start: platformlsp.Position{Line: 0, Character: 0}, End: platformlsp.Position{Line: 10, Character: 1}},
			SelectionRange: platformlsp.Range{Start: platformlsp.Position{Line: 0, Character: 0}, End: platformlsp.Position{Line: 0, Character: 8}},
			Children: []platformlsp.DocumentSymbol{
				{
					Name: "Method1", Kind: platformlsp.SymbolKindMethod,
					Range:          platformlsp.Range{Start: platformlsp.Position{Line: 1, Character: 2}, End: platformlsp.Position{Line: 3, Character: 3}},
					SelectionRange: platformlsp.Range{Start: platformlsp.Position{Line: 1, Character: 2}, End: platformlsp.Position{Line: 1, Character: 9}},
				},
				{
					Name: "Method2", Kind: platformlsp.SymbolKindMethod,
					Range:          platformlsp.Range{Start: platformlsp.Position{Line: 4, Character: 2}, End: platformlsp.Position{Line: 6, Character: 3}},
					SelectionRange: platformlsp.Range{Start: platformlsp.Position{Line: 4, Character: 2}, End: platformlsp.Position{Line: 4, Character: 9}},
				},
			},
		},
	}
	raw, _ := json.Marshal(syms)

	result, count, files := formatDocumentSymbolResult(raw, "")
	if count != 3 {
		t.Errorf("expected count=3 (1 parent + 2 children), got %d", count)
	}
	if files != 1 {
		t.Errorf("expected files=1, got %d", files)
	}
	_ = result
}

func TestFormatWorkspaceSymbolResult(t *testing.T) {
	syms := []platformlsp.SymbolInformation{
		{
			Name: "main", Kind: platformlsp.SymbolKindFunction,
			Location: platformlsp.Location{
				URI:   "file:///home/user/project/main.go",
				Range: platformlsp.Range{Start: platformlsp.Position{Line: 0, Character: 0}, End: platformlsp.Position{Line: 0, Character: 4}},
			},
			ContainerName: "package",
		},
	}
	raw, _ := json.Marshal(syms)

	result, count, files := formatWorkspaceSymbolResult(raw, "/home/user/project")
	if count != 1 {
		t.Errorf("expected count=1, got %d", count)
	}
	if files != 1 {
		t.Errorf("expected files=1, got %d", files)
	}
	_ = result
}

func TestFormatPrepareCallHierarchyResult(t *testing.T) {
	items := []platformlsp.CallHierarchyItem{
		{
			Name: "main", Kind: platformlsp.SymbolKindFunction,
			URI:   "file:///home/user/project/main.go",
			Range: platformlsp.Range{Start: platformlsp.Position{Line: 0, Character: 0}, End: platformlsp.Position{Line: 5, Character: 1}},
			SelectionRange: platformlsp.Range{Start: platformlsp.Position{Line: 0, Character: 0}, End: platformlsp.Position{Line: 0, Character: 4}},
		},
	}
	raw, _ := json.Marshal(items)

	result, count, files := formatPrepareCallHierarchyResult(raw, "/home/user/project")
	if count != 1 {
		t.Errorf("expected count=1, got %d", count)
	}
	if files != 1 {
		t.Errorf("expected files=1, got %d", files)
	}
	_ = result
}

func TestFormatIncomingCallsResult(t *testing.T) {
	calls := []platformlsp.CallHierarchyIncomingCall{
		{
			From: platformlsp.CallHierarchyItem{
				Name: "caller", Kind: platformlsp.SymbolKindFunction,
				URI:   "file:///home/user/project/main.go",
				Range: platformlsp.Range{Start: platformlsp.Position{Line: 10, Character: 0}, End: platformlsp.Position{Line: 15, Character: 1}},
				SelectionRange: platformlsp.Range{Start: platformlsp.Position{Line: 10, Character: 0}, End: platformlsp.Position{Line: 10, Character: 6}},
			},
			FromRanges: []platformlsp.Range{
				{Start: platformlsp.Position{Line: 12, Character: 4}, End: platformlsp.Position{Line: 12, Character: 8}},
			},
		},
	}
	raw, _ := json.Marshal(calls)

	result, count, files := formatIncomingCallsResult(raw, "/home/user/project")
	if count != 1 || files != 1 {
		t.Errorf("expected 1/1, got %d/%d", count, files)
	}
	_ = result
}

func TestFormatOutgoingCallsResult(t *testing.T) {
	calls := []platformlsp.CallHierarchyOutgoingCall{
		{
			To: platformlsp.CallHierarchyItem{
				Name: "callee", Kind: platformlsp.SymbolKindFunction,
				URI:   "file:///home/user/project/lib.go",
				Range: platformlsp.Range{Start: platformlsp.Position{Line: 5, Character: 0}, End: platformlsp.Position{Line: 10, Character: 1}},
				SelectionRange: platformlsp.Range{Start: platformlsp.Position{Line: 5, Character: 0}, End: platformlsp.Position{Line: 5, Character: 6}},
			},
			FromRanges: []platformlsp.Range{
				{Start: platformlsp.Position{Line: 3, Character: 8}, End: platformlsp.Position{Line: 3, Character: 14}},
			},
		},
	}
	raw, _ := json.Marshal(calls)

	result, count, files := formatOutgoingCallsResult(raw, "/home/user/project")
	if count != 1 || files != 1 {
		t.Errorf("expected 1/1, got %d/%d", count, files)
	}
	_ = result
}

func TestFormatURI(t *testing.T) {
	result := formatURI("file:///home/user/project/main.go", "/home/user/project")
	if result == "" {
		t.Error("formatURI returned empty string")
	}
}

func TestFormatLocation(t *testing.T) {
	loc := platformlsp.Location{
		URI:   "file:///home/user/project/main.go",
		Range: platformlsp.Range{Start: platformlsp.Position{Line: 9, Character: 4}},
	}
	result := formatLocation(loc, "/home/user/project")
	if len(result) == 0 {
		t.Error("formatLocation returned empty")
	}
}

func TestParseJSON(t *testing.T) {
	type testStruct struct {
		Name string `json:"name"`
	}
	raw := []byte(`{"name":"test"}`)
	var dst testStruct
	if err := parseJSON(raw, &dst); err != nil {
		t.Errorf("parseJSON error: %v", err)
	}
	if dst.Name != "test" {
		t.Errorf("expected 'test', got %q", dst.Name)
	}
}

func TestDecodeInput_Valid(t *testing.T) {
	input, err := coretool.DecodeInput[Input](inputSchema(), map[string]any{
		"operation": "hover",
		"filePath":  "/path/to/file.go",
		"line":      float64(10),
		"character": float64(5),
	})
	if err != nil {
		t.Fatalf("DecodeInput error: %v", err)
	}
	if input.Operation != "hover" {
		t.Errorf("expected hover, got %q", input.Operation)
	}
	if input.FilePath != "/path/to/file.go" {
		t.Errorf("unexpected filePath: %q", input.FilePath)
	}
}
