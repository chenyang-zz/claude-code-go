package lsp

import (
	"testing"
)

func TestNewLSPDiagnosticRegistry(t *testing.T) {
	r := NewLSPDiagnosticRegistry()
	if r == nil {
		t.Fatal("NewLSPDiagnosticRegistry returned nil")
	}
	if r.PendingCount() != 0 {
		t.Fatalf("expected 0 pending, got %d", r.PendingCount())
	}
}

func TestRegisterPending(t *testing.T) {
	r := NewLSPDiagnosticRegistry()

	files := []DiagnosticFile{
		{
			URI: "file:///test/main.go",
			Diagnostics: []Diagnostic{
				{
					Message:  "unused variable",
					Severity: SeverityWarning,
					Range: Range{
						Start: Position{Line: 10, Character: 5},
						End:   Position{Line: 10, Character: 12},
					},
				},
			},
		},
	}

	r.RegisterPending("gopls", files)
	if r.PendingCount() != 1 {
		t.Fatalf("expected 1 pending, got %d", r.PendingCount())
	}
}

func TestCheckForDiagnostics(t *testing.T) {
	r := NewLSPDiagnosticRegistry()

	files := []DiagnosticFile{
		{
			URI: "file:///test/main.go",
			Diagnostics: []Diagnostic{
				{
					Message:  "unused variable",
					Severity: SeverityWarning,
					Range: Range{
						Start: Position{Line: 10, Character: 5},
						End:   Position{Line: 10, Character: 12},
					},
				},
			},
		},
	}

	r.RegisterPending("gopls", files)

	result := r.CheckForDiagnostics()
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if result[0].ServerName != "gopls" {
		t.Fatalf("expected server name 'gopls', got %q", result[0].ServerName)
	}
	if len(result[0].Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(result[0].Files))
	}
	if result[0].Files[0].Diagnostics[0].Message != "unused variable" {
		t.Fatalf("unexpected diagnostic message: %q", result[0].Files[0].Diagnostics[0].Message)
	}

	// After checking, pending should be empty.
	if r.PendingCount() != 0 {
		t.Fatalf("expected 0 pending after check, got %d", r.PendingCount())
	}

	// Second check should return nil (no new diagnostics).
	result2 := r.CheckForDiagnostics()
	if result2 != nil {
		t.Fatal("expected nil result on second check")
	}
}

func TestDeduplicationWithinBatch(t *testing.T) {
	r := NewLSPDiagnosticRegistry()

	diag := Diagnostic{
		Message:  "duplicate message",
		Severity: SeverityError,
		Range: Range{
			Start: Position{Line: 1, Character: 0},
			End:   Position{Line: 1, Character: 10},
		},
	}

	files := []DiagnosticFile{
		{
			URI:         "file:///test/a.go",
			Diagnostics: []Diagnostic{diag, diag}, // same diagnostic twice
		},
	}

	r.RegisterPending("gopls", files)
	result := r.CheckForDiagnostics()
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if len(result[0].Files[0].Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic after dedup, got %d", len(result[0].Files[0].Diagnostics))
	}
}

func TestCrossTurnDeduplication(t *testing.T) {
	r := NewLSPDiagnosticRegistry()

	diag := Diagnostic{
		Message:  "repeated error",
		Severity: SeverityError,
		Range: Range{
			Start: Position{Line: 5, Character: 0},
			End:   Position{Line: 5, Character: 10},
		},
	}

	// First turn: register and check.
	r.RegisterPending("gopls", []DiagnosticFile{
		{URI: "file:///test/a.go", Diagnostics: []Diagnostic{diag}},
	})
	result1 := r.CheckForDiagnostics()
	if len(result1) != 1 {
		t.Fatal("expected result on first check")
	}

	// Second turn: same diagnostic should be suppressed by cross-turn dedup.
	r.RegisterPending("gopls", []DiagnosticFile{
		{URI: "file:///test/a.go", Diagnostics: []Diagnostic{diag}},
	})
	result2 := r.CheckForDiagnostics()
	if result2 != nil {
		t.Fatalf("expected nil on second check due to cross-turn dedup, got %d results", len(result2))
	}
}

func TestClearDeliveredForFile(t *testing.T) {
	r := NewLSPDiagnosticRegistry()

	diag := Diagnostic{
		Message:  "test error",
		Severity: SeverityError,
		Range: Range{
			Start: Position{Line: 1, Character: 0},
			End:   Position{Line: 1, Character: 5},
		},
	}

	// Deliver once.
	r.RegisterPending("gopls", []DiagnosticFile{
		{URI: "file:///test/a.go", Diagnostics: []Diagnostic{diag}},
	})
	r.CheckForDiagnostics()

	// Clear the file's delivered cache.
	r.ClearDeliveredForFile("file:///test/a.go")

	// Same diagnostic should now be delivered again.
	r.RegisterPending("gopls", []DiagnosticFile{
		{URI: "file:///test/a.go", Diagnostics: []Diagnostic{diag}},
	})
	result := r.CheckForDiagnostics()
	if len(result) != 1 {
		t.Fatalf("expected 1 result after clearing delivered cache, got %d", len(result))
	}
}

func TestVolumeLimitingPerFile(t *testing.T) {
	r := NewLSPDiagnosticRegistry()

	var diags []Diagnostic
	for i := 0; i < 20; i++ {
		diags = append(diags, Diagnostic{
			Message:  "error",
			Severity: SeverityError,
			Range: Range{
				Start: Position{Line: i, Character: 0},
				End:   Position{Line: i, Character: 5},
			},
		})
	}

	r.RegisterPending("gopls", []DiagnosticFile{
		{URI: "file:///test/a.go", Diagnostics: diags},
	})

	result := r.CheckForDiagnostics()
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if len(result[0].Files[0].Diagnostics) > maxDiagnosticsPerFile {
		t.Fatalf("expected at most %d diagnostics per file, got %d",
			maxDiagnosticsPerFile, len(result[0].Files[0].Diagnostics))
	}
}

func TestVolumeLimitingTotal(t *testing.T) {
	r := NewLSPDiagnosticRegistry()

	// Create multiple files each with many diagnostics.
	for fi := 0; fi < 5; fi++ {
		var diags []Diagnostic
		for i := 0; i < 15; i++ {
			diags = append(diags, Diagnostic{
				Message:  "error",
				Severity: SeverityError,
				Range: Range{
					Start: Position{Line: i, Character: 0},
					End:   Position{Line: i, Character: 5},
				},
			})
		}
		r.RegisterPending("gopls", []DiagnosticFile{
			{URI: "file:///test/file.go", Diagnostics: diags},
		})
	}

	result := r.CheckForDiagnostics()
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	total := 0
	for _, f := range result[0].Files {
		total += len(f.Diagnostics)
	}
	if total > maxTotalDiagnostics {
		t.Fatalf("expected at most %d total diagnostics, got %d",
			maxTotalDiagnostics, total)
	}
}

func TestSeverityPriorityInVolumeLimiting(t *testing.T) {
	r := NewLSPDiagnosticRegistry()

	// Mix of errors and hints — errors should be kept, hints dropped.
	var diags []Diagnostic
	for i := 0; i < 8; i++ {
		diags = append(diags, Diagnostic{
			Message:  "hint",
			Severity: SeverityHint,
			Range: Range{
				Start: Position{Line: i, Character: 0},
				End:   Position{Line: i, Character: 5},
			},
		})
	}
	for i := 0; i < 8; i++ {
		diags = append(diags, Diagnostic{
			Message:  "error",
			Severity: SeverityError,
			Range: Range{
				Start: Position{Line: 100 + i, Character: 0},
				End:   Position{Line: 100 + i, Character: 5},
			},
		})
	}

	r.RegisterPending("gopls", []DiagnosticFile{
		{URI: "file:///test/a.go", Diagnostics: diags},
	})

	result := r.CheckForDiagnostics()
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}

	// First diagnostics should be errors (severity 1), not hints (severity 4).
	kept := result[0].Files[0].Diagnostics
	if len(kept) == 0 {
		t.Fatal("expected some diagnostics to be kept")
	}
	if kept[0].Severity != SeverityError {
		t.Fatalf("expected first diagnostic to be Error (severity=1), got severity=%d with message %q",
			kept[0].Severity, kept[0].Message)
	}
}

func TestClearPending(t *testing.T) {
	r := NewLSPDiagnosticRegistry()

	r.RegisterPending("gopls", []DiagnosticFile{
		{
			URI: "file:///test/a.go",
			Diagnostics: []Diagnostic{
				{Message: "test", Severity: SeverityError, Range: Range{
					Start: Position{Line: 1, Character: 0},
					End:   Position{Line: 1, Character: 5},
				}},
			},
		},
	})
	r.ClearPending()
	if r.PendingCount() != 0 {
		t.Fatalf("expected 0 pending after clear, got %d", r.PendingCount())
	}
}

func TestResetAll(t *testing.T) {
	r := NewLSPDiagnosticRegistry()

	diag := Diagnostic{
		Message:  "test",
		Severity: SeverityError,
		Range: Range{
			Start: Position{Line: 1, Character: 0},
			End:   Position{Line: 1, Character: 5},
		},
	}

	// Deliver a diagnostic to populate cross-turn state.
	r.RegisterPending("gopls", []DiagnosticFile{
		{URI: "file:///test/a.go", Diagnostics: []Diagnostic{diag}},
	})
	r.CheckForDiagnostics()

	// Reset all.
	r.ResetAll()

	// Same diagnostic should now be delivered again (cross-turn state cleared).
	r.RegisterPending("gopls", []DiagnosticFile{
		{URI: "file:///test/a.go", Diagnostics: []Diagnostic{diag}},
	})
	result := r.CheckForDiagnostics()
	if len(result) != 1 {
		t.Fatalf("expected 1 result after reset, got %d", len(result))
	}
}

func TestEmptyDiagnosticsFiltered(t *testing.T) {
	r := NewLSPDiagnosticRegistry()

	// Register files with no diagnostics.
	r.RegisterPending("gopls", []DiagnosticFile{
		{URI: "file:///test/a.go", Diagnostics: []Diagnostic{}},
	})

	result := r.CheckForDiagnostics()
	if result != nil {
		t.Fatal("expected nil result for empty diagnostics")
	}
}

func TestMultipleServers(t *testing.T) {
	r := NewLSPDiagnosticRegistry()

	r.RegisterPending("gopls", []DiagnosticFile{
		{
			URI: "file:///test/a.go",
			Diagnostics: []Diagnostic{
				{
					Message:  "error from gopls",
					Severity: SeverityError,
					Range: Range{
						Start: Position{Line: 1, Character: 0},
						End:   Position{Line: 1, Character: 5},
					},
				},
			},
		},
	})
	r.RegisterPending("tsserver", []DiagnosticFile{
		{
			URI: "file:///test/b.ts",
			Diagnostics: []Diagnostic{
				{
					Message:  "error from tsserver",
					Severity: SeverityWarning,
					Range: Range{
						Start: Position{Line: 2, Character: 0},
						End:   Position{Line: 2, Character: 5},
					},
				},
			},
		},
	})

	result := r.CheckForDiagnostics()
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if result[0].ServerName != "gopls, tsserver" && result[0].ServerName != "tsserver, gopls" {
		t.Fatalf("expected server names to include both gopls and tsserver, got %q", result[0].ServerName)
	}
	if len(result[0].Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(result[0].Files))
	}
}

func TestConcurrentAccess(t *testing.T) {
	r := NewLSPDiagnosticRegistry()
	done := make(chan struct{})

	// Concurrent writers.
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				r.RegisterPending("gopls", []DiagnosticFile{
					{
						URI: "file:///test/a.go",
						Diagnostics: []Diagnostic{
							{
								Message:  "concurrent test",
								Severity: SeverityError,
								Range: Range{
									Start: Position{Line: j, Character: 0},
									End:   Position{Line: j, Character: 5},
								},
							},
						},
					},
				})
			}
			done <- struct{}{}
		}()
	}

	// Concurrent readers.
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 5; j++ {
				r.CheckForDiagnostics()
				r.PendingCount()
			}
			done <- struct{}{}
		}()
	}

	// Wait for all goroutines.
	for i := 0; i < 15; i++ {
		<-done
	}
}
