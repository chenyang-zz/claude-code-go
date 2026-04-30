package lsp

import (
	"encoding/json"
	"testing"
)

func TestFormatDiagnosticsForAttachment(t *testing.T) {
	params := PublishDiagnosticsParams{
		URI: "file:///test/main.go",
		Diagnostics: []Diagnostic{
			{
				Message:  "unused variable",
				Severity: SeverityWarning,
				Range: Range{
					Start: Position{Line: 10, Character: 5},
					End:   Position{Line: 10, Character: 12},
				},
				Source: "go-lint",
				Code:   "unused",
			},
		},
	}

	files := formatDiagnosticsForAttachment(params)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	f := files[0]
	if f.URI != "/test/main.go" {
		t.Fatalf("expected URI '/test/main.go', got %q", f.URI)
	}
	if len(f.Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(f.Diagnostics))
	}

	d := f.Diagnostics[0]
	if d.Message != "unused variable" {
		t.Fatalf("expected message 'unused variable', got %q", d.Message)
	}
	if d.Severity != SeverityWarning {
		t.Fatalf("expected severity Warning, got %d", d.Severity)
	}
	if d.Source != "go-lint" {
		t.Fatalf("expected source 'go-lint', got %q", d.Source)
	}
	if d.Code != "unused" {
		t.Fatalf("expected code 'unused', got %q", d.Code)
	}
	if d.Range.Start.Line != 10 || d.Range.Start.Character != 5 {
		t.Fatalf("unexpected range start: line=%d char=%d", d.Range.Start.Line, d.Range.Start.Character)
	}
	if d.Range.End.Line != 10 || d.Range.End.Character != 12 {
		t.Fatalf("unexpected range end: line=%d char=%d", d.Range.End.Line, d.Range.End.Character)
	}
}

func TestFormatDiagnosticsURIVariants(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		expected string
	}{
		{"file scheme", "file:///home/user/test.go", "/home/user/test.go"},
		{"plain path", "/home/user/test.go", "/home/user/test.go"},
		{"relative path", "src/main.go", "src/main.go"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			params := PublishDiagnosticsParams{
				URI: tc.uri,
				Diagnostics: []Diagnostic{
					{
						Message:  "test",
						Severity: SeverityError,
						Range: Range{
							Start: Position{Line: 1, Character: 0},
							End:   Position{Line: 1, Character: 5},
						},
					},
				},
			}
			files := formatDiagnosticsForAttachment(params)
			if len(files) != 1 {
				t.Fatalf("expected 1 file, got %d", len(files))
			}
			if files[0].URI != tc.expected {
				t.Fatalf("expected URI %q, got %q", tc.expected, files[0].URI)
			}
		})
	}
}

func TestFormatDiagnosticsSeverityMapping(t *testing.T) {
	tests := []struct {
		severity DiagnosticSeverity
		expected string
	}{
		{SeverityError, "Error"},
		{SeverityWarning, "Warning"},
		{SeverityInformation, "Info"},
		{SeverityHint, "Hint"},
		{DiagnosticSeverity(99), "Error"}, // unknown defaults to Error
	}

	for _, tc := range tests {
		s := SeverityString(tc.severity)
		if s != tc.expected {
			t.Errorf("SeverityString(%d) = %q, expected %q", tc.severity, s, tc.expected)
		}
	}
}

func TestFormatDiagnosticsEmptyCode(t *testing.T) {
	params := PublishDiagnosticsParams{
		URI: "file:///test/main.go",
		Diagnostics: []Diagnostic{
			{
				Message:  "test",
				Severity: SeverityError,
				Range: Range{
					Start: Position{Line: 1, Character: 0},
					End:   Position{Line: 1, Character: 5},
				},
			},
		},
	}

	files := formatDiagnosticsForAttachment(params)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Diagnostics[0].Code != "" {
		t.Fatalf("expected empty code, got %q", files[0].Diagnostics[0].Code)
	}
}

func TestFormatDiagnosticsMultipleDiagnostics(t *testing.T) {
	params := PublishDiagnosticsParams{
		URI: "file:///test/main.go",
		Diagnostics: []Diagnostic{
			{
				Message:  "error 1",
				Severity: SeverityError,
				Range:    Range{Start: Position{Line: 1, Character: 0}, End: Position{Line: 1, Character: 5}},
			},
			{
				Message:  "warning 1",
				Severity: SeverityWarning,
				Range:    Range{Start: Position{Line: 2, Character: 0}, End: Position{Line: 2, Character: 5}},
			},
			{
				Message:  "info 1",
				Severity: SeverityInformation,
				Range:    Range{Start: Position{Line: 3, Character: 0}, End: Position{Line: 3, Character: 5}},
			},
		},
	}

	files := formatDiagnosticsForAttachment(params)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if len(files[0].Diagnostics) != 3 {
		t.Fatalf("expected 3 diagnostics, got %d", len(files[0].Diagnostics))
	}
}

func TestCreateDiagnosticKey(t *testing.T) {
	d1 := Diagnostic{
		Message:  "test error",
		Severity: SeverityError,
		Range:    Range{Start: Position{Line: 1, Character: 2}, End: Position{Line: 3, Character: 4}},
		Source:   "compiler",
		Code:     "E001",
	}
	d2 := Diagnostic{
		Message:  "test error",
		Severity: SeverityError,
		Range:    Range{Start: Position{Line: 1, Character: 2}, End: Position{Line: 3, Character: 4}},
		Source:   "compiler",
		Code:     "E001",
	}
	d3 := Diagnostic{
		Message:  "different error",
		Severity: SeverityError,
		Range:    Range{Start: Position{Line: 1, Character: 2}, End: Position{Line: 3, Character: 4}},
	}

	if createDiagnosticKey(d1) != createDiagnosticKey(d2) {
		t.Fatal("expected same key for identical diagnostics")
	}
	if createDiagnosticKey(d1) == createDiagnosticKey(d3) {
		t.Fatal("expected different keys for different diagnostics")
	}
}

func TestHandlerNotificationDispatch(t *testing.T) {
	// Create a Client and register a notification handler.
	// Since we can't actually start an LSP server in unit tests,
	// we verify the handler registration mechanism works correctly.
	client := NewClient()

	received := make(chan json.RawMessage, 1)
	client.OnNotification("textDocument/publishDiagnostics", func(rawParams json.RawMessage) {
		received <- rawParams
	})

	// Verify handler was registered by checking notifHandlers map.
	client.mu.Lock()
	handlers := client.notifHandlers["textDocument/publishDiagnostics"]
	client.mu.Unlock()

	if len(handlers) != 1 {
		t.Fatalf("expected 1 handler registered, got %d", len(handlers))
	}
}

func TestHandlerMultipleRegistrations(t *testing.T) {
	client := NewClient()

	count := 0
	client.OnNotification("textDocument/publishDiagnostics", func(_ json.RawMessage) {
		count++
	})
	client.OnNotification("textDocument/publishDiagnostics", func(_ json.RawMessage) {
		count++
	})

	client.mu.Lock()
	handlers := client.notifHandlers["textDocument/publishDiagnostics"]
	client.mu.Unlock()

	if len(handlers) != 2 {
		t.Fatalf("expected 2 handlers, got %d", len(handlers))
	}
}

func TestSeverityString(t *testing.T) {
	tests := []struct {
		severity DiagnosticSeverity
		expected string
	}{
		{SeverityError, "Error"},
		{SeverityWarning, "Warning"},
		{SeverityInformation, "Info"},
		{SeverityHint, "Hint"},
	}

	for _, tc := range tests {
		if s := SeverityString(tc.severity); s != tc.expected {
			t.Errorf("SeverityString(%d) = %q, expected %q", tc.severity, s, tc.expected)
		}
	}
}
