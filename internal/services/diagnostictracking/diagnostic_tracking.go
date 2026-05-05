// Package diagnostictracking tracks IDE diagnostic changes across file edits,
// enabling the system to detect new diagnostics introduced by model-generated
// code changes.
package diagnostictracking

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	lsp "github.com/sheepzhao/claude-code-go/internal/platform/lsp"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// MaxDiagnosticsSummaryChars limits the total length of the formatted diagnostic
// summary to prevent excessively long messages.
const MaxDiagnosticsSummaryChars = 4000

// IDERpcCaller abstracts the IDE RPC calls needed by DiagnosticTracking.
// Implementations wrap an MCP client connection to the IDE.
type IDERpcCaller interface {
	// OpenFile ensures a file is opened in the IDE for diagnostics to work.
	OpenFile(filePath string) error
	// GetDiagnostics retrieves diagnostics for all files (empty params) or a
	// specific file (with uri param).
	GetDiagnostics(uri string) ([]lsp.DiagnosticFile, error)
}

// DiagnosticTrackingService tracks diagnostic baselines for files and reports
// new diagnostics introduced by edits. It follows a singleton pattern and is
// initialized once with an IDE RPC caller.
type DiagnosticTrackingService struct {
	mu       sync.Mutex
	initialized bool
	caller      IDERpcCaller

	// baseline stores the last known diagnostics per normalized path.
	baseline map[string][]lsp.Diagnostic

	// lastProcessedTimestamps tracks when each file was last processed.
	lastProcessedTimestamps map[string]int64

	// rightFileDiagnosticsState tracks _claude_fs_right file diagnostics state.
	rightFileDiagnosticsState map[string][]lsp.Diagnostic
}

var (
	instance     *DiagnosticTrackingService
	once         sync.Once
)

// GetInstance returns the singleton DiagnosticTrackingService instance.
func GetInstance() *DiagnosticTrackingService {
	once.Do(func() {
		instance = &DiagnosticTrackingService{}
	})
	return instance
}

// ResetForTest resets the singleton for testing. This should only be called
// from test code.
func ResetForTest() {
	once = sync.Once{}
	instance = nil
}

// Initialize sets up the service with an IDE RPC caller. No-op if already
// initialized.
func (s *DiagnosticTrackingService) Initialize(caller IDERpcCaller) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.initialized {
		return
	}

	s.caller = caller
	s.initialized = true
}

// Shutdown clears all state and marks the service as uninitialized.
func (s *DiagnosticTrackingService) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.initialized = false
	s.baseline = nil
	s.rightFileDiagnosticsState = nil
	s.lastProcessedTimestamps = nil
	s.caller = nil
}

// Reset clears all tracking state while keeping the service initialized.
func (s *DiagnosticTrackingService) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.baseline = nil
	s.rightFileDiagnosticsState = nil
	s.lastProcessedTimestamps = nil
}

// normalizePath removes protocol prefixes from a file URI and normalizes the
// path for consistent lookups across platforms.
func normalizePath(path string) string {
	protocolPrefixes := []string{
		"file://",
		"_claude_fs_right:",
		"_claude_fs_left:",
	}

	normalized := path
	for _, prefix := range protocolPrefixes {
		if strings.HasPrefix(path, prefix) {
			normalized = path[len(prefix):]
			break
		}
	}

	return filepath.ToSlash(normalized)
}

// EnsureFileOpened asks the IDE to open a file so that LSP diagnostics become
// available.
func (s *DiagnosticTrackingService) EnsureFileOpened(fileURI string) error {
	s.mu.Lock()
	caller := s.caller
	initialized := s.initialized
	s.mu.Unlock()

	if !initialized || caller == nil {
		return nil
	}

	return caller.OpenFile(fileURI)
}

// BeforeFileEdited captures a baseline snapshot of diagnostics for a file
// before it is edited. This baseline is later used by GetNewDiagnostics to
// detect newly introduced diagnostics.
func (s *DiagnosticTrackingService) BeforeFileEdited(filePath string) {
	s.mu.Lock()
	caller := s.caller
	initialized := s.initialized
	s.mu.Unlock()

	if !initialized || caller == nil {
		return
	}

	result, err := caller.GetDiagnostics(fmt.Sprintf("file://%s", filePath))
	if err != nil {
		// Fail silently if IDE does not support diagnostics.
		return
	}

	normalized := normalizePath(filePath)
	var baselineDiags []lsp.Diagnostic
	if len(result) > 0 && normalizePath(result[0].URI) == normalized {
		baselineDiags = result[0].Diagnostics
	}

	s.mu.Lock()
	if s.baseline == nil {
		s.baseline = make(map[string][]lsp.Diagnostic)
	}
	if s.lastProcessedTimestamps == nil {
		s.lastProcessedTimestamps = make(map[string]int64)
	}

	s.baseline[normalized] = baselineDiags
	s.lastProcessedTimestamps[normalized] = 0 // Will be set on successful processing
	s.mu.Unlock()
}

// GetNewDiagnostics retrieves diagnostics that are not present in the baselines
// of files that have been tracked via BeforeFileEdited.
func (s *DiagnosticTrackingService) GetNewDiagnostics() ([]lsp.DiagnosticFile, error) {
	s.mu.Lock()
	caller := s.caller
	initialized := s.initialized
	s.mu.Unlock()

	if !initialized || caller == nil {
		return nil, nil
	}

	allFiles, err := caller.GetDiagnostics("")
	if err != nil {
		return nil, err
	}
	if len(allFiles) == 0 {
		return nil, nil
	}

	s.mu.Lock()
	if s.baseline == nil {
		s.baseline = make(map[string][]lsp.Diagnostic)
	}
	if s.rightFileDiagnosticsState == nil {
		s.rightFileDiagnosticsState = make(map[string][]lsp.Diagnostic)
	}
	baseline := s.baseline
	rightState := s.rightFileDiagnosticsState
	s.mu.Unlock()

	// Separate file:// and _claude_fs_right: URIs.
	type diagFile struct {
		file lsp.DiagnosticFile
		normalized string
	}
	var fileURIs []diagFile
	rightURIs := make(map[string]lsp.DiagnosticFile)

	for _, f := range allFiles {
		norm := normalizePath(f.URI)
		if _, ok := baseline[norm]; !ok {
			continue
		}
		if strings.HasPrefix(f.URI, "file://") {
			fileURIs = append(fileURIs, diagFile{file: f, normalized: norm})
		} else if strings.HasPrefix(f.URI, "_claude_fs_right:") {
			rightURIs[norm] = f
		}
	}

	var result []lsp.DiagnosticFile

	for _, df := range fileURIs {
		norm := df.normalized
		fileDiagnostics := df.file.Diagnostics

		// Check for _claude_fs_right alternative.
		if rightFile, ok := rightURIs[norm]; ok {
			prevRight := rightState[norm]
			// Use right file diagnostics if:
			// 1. We've never seen right diagnostics for this file, or
			// 2. Right diagnostics have just changed.
			if prevRight == nil || !diagnosticArraysEqual(prevRight, rightFile.Diagnostics) {
				fileDiagnostics = rightFile.Diagnostics
			}
			rightState[norm] = rightFile.Diagnostics
		}

		// Find diagnostics not in baseline.
		baselineDiags := baseline[norm]
		var newDiags []lsp.Diagnostic
		for _, d := range fileDiagnostics {
			if !containsDiagnostic(baselineDiags, d) {
				newDiags = append(newDiags, d)
			}
		}

		if len(newDiags) > 0 {
			result = append(result, lsp.DiagnosticFile{
				URI:         df.file.URI,
				Diagnostics: newDiags,
			})
		}

		baseline[norm] = fileDiagnostics
	}

	return result, nil
}

// containsDiagnostic checks if a diagnostic array contains a specific diagnostic.
func containsDiagnostic(diags []lsp.Diagnostic, target lsp.Diagnostic) bool {
	for _, d := range diags {
		if diagnosticsEqual(d, target) {
			return true
		}
	}
	return false
}

// diagnosticsEqual compares two diagnostics for structural equality.
func diagnosticsEqual(a, b lsp.Diagnostic) bool {
	return a.Message == b.Message &&
		a.Severity == b.Severity &&
		a.Source == b.Source &&
		a.Code == b.Code &&
		a.Range.Start.Line == b.Range.Start.Line &&
		a.Range.Start.Character == b.Range.Start.Character &&
		a.Range.End.Line == b.Range.End.Line &&
		a.Range.End.Character == b.Range.End.Character
}

// diagnosticArraysEqual checks if two diagnostic arrays contain the same set
// of diagnostics, ignoring order.
func diagnosticArraysEqual(a, b []lsp.Diagnostic) bool {
	if len(a) != len(b) {
		return false
	}
	for _, da := range a {
		if !containsDiagnostic(b, da) {
			return false
		}
	}
	return true
}

// HandleQueryStart is called at the start of each new query to initialize or
// reset the diagnostic tracker. If not yet initialized, it attempts to find a
// connected IDE client from the provided callers. If already initialized, it
// resets tracking state for the new query loop.
func (s *DiagnosticTrackingService) HandleQueryStart(callers []IDERpcCaller) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		// Find the first available caller.
		for _, c := range callers {
			if c != nil {
				s.caller = c
				s.initialized = true
				break
			}
		}
	} else {
		s.baseline = nil
		s.rightFileDiagnosticsState = nil
		s.lastProcessedTimestamps = nil
	}
}

// severityName returns a string label for a diagnostic severity level.
func severityName(severity lsp.DiagnosticSeverity) string {
	switch severity {
	case lsp.SeverityError:
		return "ERROR"
	case lsp.SeverityWarning:
		return "WARN"
	case lsp.SeverityInformation:
		return "INFO"
	case lsp.SeverityHint:
		return "HINT"
	default:
		return "UNKN"
	}
}

// FormatDiagnosticsSummary formats a set of diagnostic files into a
// human-readable summary string suitable for inclusion in messages or logs.
func FormatDiagnosticsSummary(files []lsp.DiagnosticFile) string {
	if len(files) == 0 {
		return ""
	}

	// Sort files by URI for deterministic output.
	sort.Slice(files, func(i, j int) bool {
		return files[i].URI < files[j].URI
	})

	var b strings.Builder
	for i, file := range files {
		if i > 0 {
			b.WriteString("\n\n")
		}

		filename := filepath.Base(file.URI)
		b.WriteString(filename)
		b.WriteString(":\n")

		// Sort diagnostics by severity (error first) then by line.
		sorted := make([]lsp.Diagnostic, len(file.Diagnostics))
		copy(sorted, file.Diagnostics)
		sort.Slice(sorted, func(i, j int) bool {
			if sorted[i].Severity != sorted[j].Severity {
				return sorted[i].Severity < sorted[j].Severity
			}
			return sorted[i].Range.Start.Line < sorted[j].Range.Start.Line
		})

		for _, d := range sorted {
			sev := severityName(d.Severity)
			line := d.Range.Start.Line + 1
			col := d.Range.Start.Character + 1
			b.WriteString(fmt.Sprintf("  [%s] Line %d:%d %s", sev, line, col, d.Message))
			if d.Code != "" {
				b.WriteString(fmt.Sprintf(" [%s]", d.Code))
			}
			if d.Source != "" {
				b.WriteString(fmt.Sprintf(" (%s)", d.Source))
			}
			b.WriteString("\n")
		}
	}

	result := b.String()
	if len(result) > MaxDiagnosticsSummaryChars {
		trunc := "[truncated]"
		result = result[:MaxDiagnosticsSummaryChars-len(trunc)] + trunc
	}

	return result
}

// ensureInitialized acquires the lock and returns the caller if initialized.
func (s *DiagnosticTrackingService) ensureInitialized() (IDERpcCaller, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.caller, s.initialized
}

func init() {
	_ = logger.DebugCF
}
