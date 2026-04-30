package lsp

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// Volume limiting constants for diagnostic delivery.
const (
	maxDiagnosticsPerFile = 10
	maxTotalDiagnostics   = 30
	maxDeliveredFiles     = 500
)

// pendingDiagnostic holds a batch of diagnostics received from an LSP server
// that have not yet been delivered to the conversation.
type pendingDiagnostic struct {
	serverName     string
	files          []DiagnosticFile
	timestamp      int64
	attachmentSent bool
}

// LSPDiagnosticRegistry stores LSP diagnostics received asynchronously from
// LSP servers via textDocument/publishDiagnostics notifications. It handles
// deduplication, volume limiting, and cross-turn tracking.
//
// Pattern:
//  1. LSP server sends publishDiagnostics notification
//  2. registerPendingLSPDiagnostic() stores diagnostic
//  3. checkForLSPDiagnostics() retrieves pending diagnostics (with dedup + volume limit)
//  4. Consumer delivers diagnostics to the conversation
type LSPDiagnosticRegistry struct {
	mu                  sync.Mutex
	pending             map[string]*pendingDiagnostic // diagnostic ID → pending
	delivered           map[string]map[string]struct{} // file URI → set of diagnostic keys
	nextID              int64
}

// NewLSPDiagnosticRegistry creates a new diagnostic registry.
func NewLSPDiagnosticRegistry() *LSPDiagnosticRegistry {
	return &LSPDiagnosticRegistry{
		pending:   make(map[string]*pendingDiagnostic),
		delivered: make(map[string]map[string]struct{}),
	}
}

// RegisterPending stores diagnostics received from an LSP server for later delivery.
func (r *LSPDiagnosticRegistry) RegisterPending(serverName string, files []DiagnosticFile) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nextID++
	id := fmt.Sprintf("%d_%d", time.Now().UnixNano(), r.nextID)

	logger.DebugCF("lsp.diagnostics", "registering pending diagnostics", map[string]any{
		"server":      serverName,
		"fileCount":   len(files),
		"diagnosticId": id,
	})

	r.pending[id] = &pendingDiagnostic{
		serverName:     serverName,
		files:          files,
		timestamp:      time.Now().UnixMilli(),
		attachmentSent: false,
	}
}

// createDiagnosticKey produces a stable string key for a diagnostic,
// used for within-batch and cross-turn deduplication.
func createDiagnosticKey(d Diagnostic) string {
	return fmt.Sprintf("%s|%d|%d:%d-%d:%d|%s|%s",
		d.Message,
		d.Severity,
		d.Range.Start.Line, d.Range.Start.Character,
		d.Range.End.Line, d.Range.End.Character,
		d.Source,
		d.Code,
	)
}

// deduplicateFiles removes duplicate diagnostics within a batch and across turns.
func (r *LSPDiagnosticRegistry) deduplicateFiles(allFiles []DiagnosticFile) []DiagnosticFile {
	fileMap := make(map[string]map[string]struct{})
	var deduped []DiagnosticFile

	for _, file := range allFiles {
		if _, ok := fileMap[file.URI]; !ok {
			fileMap[file.URI] = make(map[string]struct{})
			deduped = append(deduped, DiagnosticFile{URI: file.URI})
		}
		seen := fileMap[file.URI]
		previouslyDelivered := r.delivered[file.URI]

		df := &deduped[len(deduped)-1]
		for _, diag := range file.Diagnostics {
			key := createDiagnosticKey(diag)
			if _, ok := seen[key]; ok {
				continue
			}
			if previouslyDelivered != nil {
				if _, ok := previouslyDelivered[key]; ok {
					continue
				}
			}
			seen[key] = struct{}{}
			df.Diagnostics = append(df.Diagnostics, diag)
		}
	}

	// Remove files that ended up with no diagnostics after dedup.
	var result []DiagnosticFile
	for _, f := range deduped {
		if len(f.Diagnostics) > 0 {
			result = append(result, f)
		}
	}
	return result
}

// CheckForDiagnostics retrieves all pending diagnostics that have not yet been
// delivered, deduplicates them, applies volume limiting, and marks them as sent.
// Returns nil if no new diagnostics are available.
func (r *LSPDiagnosticRegistry) CheckForDiagnostics() []struct {
	ServerName string
	Files      []DiagnosticFile
} {
	r.mu.Lock()
	defer r.mu.Unlock()

	logger.DebugCF("lsp.diagnostics", "checking registry", map[string]any{
		"pending": len(r.pending),
	})

	// Collect all pending diagnostic files.
	var allFiles []DiagnosticFile
	serverNames := make(map[string]struct{})
	var toMark []*pendingDiagnostic

	for id, diag := range r.pending {
		if !diag.attachmentSent {
			allFiles = append(allFiles, diag.files...)
			serverNames[diag.serverName] = struct{}{}
			toMark = append(toMark, diag)
			// Delete after marking to avoid holding references.
			_ = id
		}
	}

	if len(allFiles) == 0 {
		return nil
	}

	// Deduplicate across all files.
	deduped := r.deduplicateFiles(allFiles)

	// Mark as sent and remove from pending.
	for _, diag := range toMark {
		diag.attachmentSent = true
	}
	for id, diag := range r.pending {
		if diag.attachmentSent {
			delete(r.pending, id)
		}
	}

	originalCount := 0
	for _, f := range allFiles {
		originalCount += len(f.Diagnostics)
	}
	dedupedCount := 0
	for _, f := range deduped {
		dedupedCount += len(f.Diagnostics)
	}

	if originalCount > dedupedCount {
		logger.DebugCF("lsp.diagnostics", "deduplication removed diagnostics", map[string]any{
			"removed": originalCount - dedupedCount,
		})
	}

	// Apply volume limiting: cap per-file and total, prioritizing by severity.
	totalCount := 0
	truncated := 0
	for i := range deduped {
		// Sort by severity (Error first).
		sortDiagnosticsBySeverity(deduped[i].Diagnostics)

		if len(deduped[i].Diagnostics) > maxDiagnosticsPerFile {
			truncated += len(deduped[i].Diagnostics) - maxDiagnosticsPerFile
			deduped[i].Diagnostics = deduped[i].Diagnostics[:maxDiagnosticsPerFile]
		}

		remaining := maxTotalDiagnostics - totalCount
		if len(deduped[i].Diagnostics) > remaining {
			truncated += len(deduped[i].Diagnostics) - remaining
			deduped[i].Diagnostics = deduped[i].Diagnostics[:remaining]
		}
		totalCount += len(deduped[i].Diagnostics)
	}

	if truncated > 0 {
		logger.DebugCF("lsp.diagnostics", "volume limiting removed diagnostics", map[string]any{
			"removed": truncated,
		})
	}

	// Filter out files with no diagnostics after limiting.
	var finalFiles []DiagnosticFile
	for _, f := range deduped {
		if len(f.Diagnostics) > 0 {
			finalFiles = append(finalFiles, f)
		}
	}

	if len(finalFiles) == 0 {
		logger.DebugCF("lsp.diagnostics", "no new diagnostics to deliver", nil)
		return nil
	}

	// Track delivered diagnostics for cross-turn deduplication.
	for _, f := range finalFiles {
		if r.delivered[f.URI] == nil {
			r.delivered[f.URI] = make(map[string]struct{})
		}
		delivered := r.delivered[f.URI]
		for _, diag := range f.Diagnostics {
			delivered[createDiagnosticKey(diag)] = struct{}{}
		}
	}

	// Evict oldest entries if delivered map exceeds capacity.
	if len(r.delivered) > maxDeliveredFiles {
		r.evictDeliveredFiles()
	}

	// Build server name string.
	var nameList []string
	for name := range serverNames {
		nameList = append(nameList, name)
	}

	logger.DebugCF("lsp.diagnostics", "delivering diagnostics", map[string]any{
		"fileCount":      len(finalFiles),
		"diagnosticCount": totalCount,
		"serverCount":     len(serverNames),
	})

	return []struct {
		ServerName string
		Files      []DiagnosticFile
	}{
		{
			ServerName: strings.Join(nameList, ", "),
			Files:      finalFiles,
		},
	}
}

// ClearPending removes all pending (undelivered) diagnostics without clearing
// cross-turn deduplication state.
func (r *LSPDiagnosticRegistry) ClearPending() {
	r.mu.Lock()
	defer r.mu.Unlock()

	logger.DebugCF("lsp.diagnostics", "clearing pending diagnostics", map[string]any{
		"count": len(r.pending),
	})
	r.pending = make(map[string]*pendingDiagnostic)
}

// ResetAll clears both pending diagnostics and cross-turn deduplication state.
func (r *LSPDiagnosticRegistry) ResetAll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	logger.DebugCF("lsp.diagnostics", "resetting all diagnostic state", map[string]any{
		"pending":   len(r.pending),
		"delivered": len(r.delivered),
	})
	r.pending = make(map[string]*pendingDiagnostic)
	r.delivered = make(map[string]map[string]struct{})
}

// ClearDeliveredForFile clears the delivered diagnostics cache for a specific
// file. This should be called when a file is edited so that new diagnostics
// for that file will be shown even if they match previously delivered ones.
func (r *LSPDiagnosticRegistry) ClearDeliveredForFile(fileURI string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.delivered[fileURI]; ok {
		logger.DebugCF("lsp.diagnostics", "clearing delivered diagnostics for file", map[string]any{
			"uri": fileURI,
		})
		delete(r.delivered, fileURI)
	}
}

// PendingCount returns the number of pending undelivered diagnostic batches.
func (r *LSPDiagnosticRegistry) PendingCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.pending)
}

// evictDeliveredFiles removes a portion of the oldest entries from the delivered
// map to prevent unbounded memory growth. Must be called with mu held.
func (r *LSPDiagnosticRegistry) evictDeliveredFiles() {
	// Remove approximately one-third of entries to reduce frequency of eviction.
	target := maxDeliveredFiles * 2 / 3
	var keys []string
	for k := range r.delivered {
		keys = append(keys, k)
		if len(keys) >= len(r.delivered)-target {
			break
		}
	}
	for _, k := range keys {
		delete(r.delivered, k)
	}
	logger.DebugCF("lsp.diagnostics", "evicted delivered file cache entries", map[string]any{
		"evicted": len(keys),
	})
}

// sortDiagnosticsBySeverity sorts diagnostics by severity (Error < Warning < Info < Hint).
func sortDiagnosticsBySeverity(diags []Diagnostic) {
	// Simple insertion sort since slices are small (max 30 total).
	for i := 1; i < len(diags); i++ {
		j := i
		for j > 0 && diags[j].Severity < diags[j-1].Severity {
			diags[j], diags[j-1] = diags[j-1], diags[j]
			j--
		}
	}
}
