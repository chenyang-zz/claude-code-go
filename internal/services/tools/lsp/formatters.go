package lsp

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	platformlsp "github.com/sheepzhao/claude-code-go/internal/platform/lsp"
)

// parseJSON is a helper that unmarshals raw JSON into a target type.
func parseJSON(raw []byte, dst any) error {
	return json.Unmarshal(raw, dst)
}

// formatResult formats an LSP result based on the operation type.
// It returns the formatted text, result count, and file count.
func formatResult(operation string, rawResult []byte, cwd string) (formatted string, resultCount int, fileCount int) {
	switch operation {
	case "goToDefinition":
		return formatGoToDefinitionResult(rawResult, cwd)
	case "findReferences":
		return formatFindReferencesResult(rawResult, cwd)
	case "hover":
		return formatHoverResult(rawResult)
	case "documentSymbol":
		return formatDocumentSymbolResult(rawResult, cwd)
	case "workspaceSymbol":
		return formatWorkspaceSymbolResult(rawResult, cwd)
	case "goToImplementation":
		return formatGoToDefinitionResult(rawResult, cwd) // Same format as goToDefinition
	case "prepareCallHierarchy":
		return formatPrepareCallHierarchyResult(rawResult, cwd)
	case "incomingCalls":
		return formatIncomingCallsResult(rawResult, cwd)
	case "outgoingCalls":
		return formatOutgoingCallsResult(rawResult, cwd)
	default:
		return string(rawResult), 0, 0
	}
}

// convertLocationLinks converts LocationLinks to Locations for uniform handling.
// It detects LocationLink by checking for the targetUri field.
func convertLocationLinks(rawResult []byte) []platformlsp.Location {
	// Detect format: LocationLink has targetUri, Location has uri.
	// Only attempt LocationLink if targetUri is present to avoid false matches.
	var hasTargetURI bool
	if bytes := []byte(rawResult); len(bytes) > 0 {
		hasTargetURI = strings.Contains(string(bytes), `"targetUri"`)
	}

	if hasTargetURI {
		var links []platformlsp.LocationLink
		if err := json.Unmarshal(rawResult, &links); err == nil && len(links) > 0 {
			locs := make([]platformlsp.Location, 0, len(links))
			for _, link := range links {
				if link.TargetURI == "" {
					continue
				}
				loc := platformlsp.Location{
					URI:   link.TargetURI,
					Range: link.TargetSelectionRange,
				}
				if loc.Range == (platformlsp.Range{}) {
					loc.Range = link.TargetRange
				}
				locs = append(locs, loc)
			}
			if len(locs) > 0 {
				return locs
			}
		}
	}

	// Try plain locations.
	var locs []platformlsp.Location
	if err := json.Unmarshal(rawResult, &locs); err == nil {
		return filterValidLocations(locs)
	}

	// Single location.
	var loc platformlsp.Location
	if err := json.Unmarshal(rawResult, &loc); err == nil && loc.URI != "" {
		return []platformlsp.Location{loc}
	}

	return nil
}

// filterValidLocations filters out locations with undefined URIs.
func filterValidLocations(locs []platformlsp.Location) []platformlsp.Location {
	filtered := make([]platformlsp.Location, 0, len(locs))
	for _, loc := range locs {
		if loc.URI != "" {
			filtered = append(filtered, loc)
		}
	}
	return filtered
}

// formatURI converts a file URI to a human-readable path.
func formatURI(uri string, cwd string) string {
	if uri == "" {
		return "<unknown location>"
	}

	filePath := strings.TrimPrefix(uri, "file://")
	// On Windows, file:///C:/path becomes /C:/path — strip the leading slash.
	if len(filePath) > 3 && filePath[0] == '/' && filePath[2] == ':' {
		filePath = filePath[1:]
	}

	// Normalize separators.
	filePath = strings.ReplaceAll(filePath, "\\", "/")

	// Convert to relative path if cwd is provided.
	if cwd != "" {
		if rel, err := filepath.Rel(cwd, filePath); err == nil {
			rel = strings.ReplaceAll(rel, "\\", "/")
			if len(rel) < len(filePath) && !strings.HasPrefix(rel, "../../") {
				return rel
			}
		}
	}

	return filePath
}

// formatLocation formats a Location with file path and line/character position.
func formatLocation(loc platformlsp.Location, cwd string) string {
	filePath := formatURI(loc.URI, cwd)
	line := loc.Range.Start.Line + 1 // Convert to 1-based
	character := loc.Range.Start.Character + 1
	return fmt.Sprintf("%s:%d:%d", filePath, line, character)
}

// formatGoToDefinitionResult formats goToDefinition and goToImplementation results.
func formatGoToDefinitionResult(rawResult []byte, cwd string) (string, int, int) {
	locs := convertLocationLinks(rawResult)

	if len(locs) == 0 {
		return "No definition found.", 0, 0
	}
	if len(locs) == 1 {
		return fmt.Sprintf("Defined in %s", formatLocation(locs[0], cwd)), 1, 1
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d definitions:\n", len(locs)))
	for _, loc := range locs {
		sb.WriteString(fmt.Sprintf("  %s\n", formatLocation(loc, cwd)))
	}
	return sb.String(), len(locs), countUniqueFiles(locs)
}

// formatFindReferencesResult formats findReferences results.
func formatFindReferencesResult(rawResult []byte, cwd string) (string, int, int) {
	var locs []platformlsp.Location
	if err := json.Unmarshal(rawResult, &locs); err != nil || len(locs) == 0 {
		return "No references found.", 0, 0
	}

	validLocs := filterValidLocations(locs)
	if len(validLocs) == 0 {
		return "No references found.", 0, 0
	}
	if len(validLocs) == 1 {
		return fmt.Sprintf("Found 1 reference:\n  %s", formatLocation(validLocs[0], cwd)), 1, 1
	}

	// Group by file.
	byFile := groupByFile(validLocs, cwd)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d references across %d files:\n", len(validLocs), len(byFile)))

	for filePath, fileLocs := range byFile {
		sb.WriteString(fmt.Sprintf("\n%s:\n", filePath))
		for _, loc := range fileLocs {
			line := loc.Range.Start.Line + 1
			character := loc.Range.Start.Character + 1
			sb.WriteString(fmt.Sprintf("  Line %d:%d\n", line, character))
		}
	}
	return sb.String(), len(validLocs), len(byFile)
}

// formatHoverResult formats hover results.
func formatHoverResult(rawResult []byte) (string, int, int) {
	var hover platformlsp.Hover
	if err := json.Unmarshal(rawResult, &hover); err != nil {
		return "No hover information available.", 0, 0
	}

	content := hover.Contents.Value
	if content == "" {
		return "No hover information available.", 0, 0
	}

	if hover.Range != nil {
		line := hover.Range.Start.Line + 1
		character := hover.Range.Start.Character + 1
		return fmt.Sprintf("Hover info at %d:%d:\n\n%s", line, character, content), 1, 1
	}

	return content, 1, 1
}

// formatDocumentSymbolResult formats documentSymbol results.
// Handles both DocumentSymbol[] (hierarchical) and SymbolInformation[] (flat) formats.
func formatDocumentSymbolResult(rawResult []byte, cwd string) (string, int, int) {
	// Try DocumentSymbol[] first.
	var docSymbols []platformlsp.DocumentSymbol
	if err := json.Unmarshal(rawResult, &docSymbols); err == nil && len(docSymbols) > 0 {
		var sb strings.Builder
		sb.WriteString("Document symbols:\n")
		count := 0
		for _, sym := range docSymbols {
			count += formatDocumentSymbolNode(&sb, sym, 0)
		}
		return sb.String(), count, 1
	}

	// Fall back to SymbolInformation[].
	var symInfos []platformlsp.SymbolInformation
	if err := json.Unmarshal(rawResult, &symInfos); err == nil && len(symInfos) > 0 {
		return formatWorkspaceSymbolResult(rawResult, cwd)
	}

	return "No symbols found in document.", 0, 0
}

// formatDocumentSymbolNode formats a single DocumentSymbol with indentation.
// Returns the count of all symbols including this one and its children.
func formatDocumentSymbolNode(sb *strings.Builder, sym platformlsp.DocumentSymbol, indent int) int {
	prefix := strings.Repeat("  ", indent)
	kindName := platformlsp.SymbolKindName(sym.Kind)
	line := fmt.Sprintf("%s%s (%s)", prefix, sym.Name, kindName)
	if sym.Detail != "" {
		line += " " + sym.Detail
	}
	line += fmt.Sprintf(" - Line %d", sym.Range.Start.Line+1)
	sb.WriteString(line + "\n")

	count := 1
	for _, child := range sym.Children {
		count += formatDocumentSymbolNode(sb, child, indent+1)
	}
	return count
}

// formatWorkspaceSymbolResult formats workspaceSymbol results.
func formatWorkspaceSymbolResult(rawResult []byte, cwd string) (string, int, int) {
	var syms []platformlsp.SymbolInformation
	if err := json.Unmarshal(rawResult, &syms); err != nil || len(syms) == 0 {
		return "No symbols found in workspace.", 0, 0
	}

	validSyms := make([]platformlsp.SymbolInformation, 0, len(syms))
	for _, sym := range syms {
		if sym.Location.URI != "" {
			validSyms = append(validSyms, sym)
		}
	}
	if len(validSyms) == 0 {
		return "No symbols found in workspace.", 0, 0
	}

	// Group by file.
	byFile := make(map[string][]platformlsp.SymbolInformation)
	for _, sym := range validSyms {
		filePath := formatURI(sym.Location.URI, cwd)
		byFile[filePath] = append(byFile[filePath], sym)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d symbols in workspace:\n", len(validSyms)))

	for filePath, fileSyms := range byFile {
		sb.WriteString(fmt.Sprintf("\n%s:\n", filePath))
		for _, sym := range fileSyms {
			kindName := platformlsp.SymbolKindName(sym.Kind)
			line := sym.Location.Range.Start.Line + 1
			symLine := fmt.Sprintf("  %s (%s) - Line %d", sym.Name, kindName, line)
			if sym.ContainerName != "" {
				symLine += fmt.Sprintf(" in %s", sym.ContainerName)
			}
			sb.WriteString(symLine + "\n")
		}
	}

	// Count unique files.
	files := make(map[string]bool)
	for _, sym := range validSyms {
		files[sym.Location.URI] = true
	}

	return sb.String(), len(validSyms), len(files)
}

// formatPrepareCallHierarchyResult formats prepareCallHierarchy results.
func formatPrepareCallHierarchyResult(rawResult []byte, cwd string) (string, int, int) {
	var items []platformlsp.CallHierarchyItem
	if err := json.Unmarshal(rawResult, &items); err != nil || len(items) == 0 {
		return "No call hierarchy item found at this position.", 0, 0
	}

	if len(items) == 1 {
		return fmt.Sprintf("Call hierarchy item: %s", formatCallHierarchyItem(items[0], cwd)), 1, 1
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d call hierarchy items:\n", len(items)))
	for _, item := range items {
		sb.WriteString(fmt.Sprintf("  %s\n", formatCallHierarchyItem(item, cwd)))
	}

	files := make(map[string]bool)
	for _, item := range items {
		if item.URI != "" {
			files[item.URI] = true
		}
	}

	return sb.String(), len(items), len(files)
}

// formatCallHierarchyItem formats a single CallHierarchyItem.
func formatCallHierarchyItem(item platformlsp.CallHierarchyItem, cwd string) string {
	if item.URI == "" {
		return fmt.Sprintf("%s (%s) - <unknown location>", item.Name, platformlsp.SymbolKindName(item.Kind))
	}

	filePath := formatURI(item.URI, cwd)
	line := item.Range.Start.Line + 1
	kindName := platformlsp.SymbolKindName(item.Kind)
	result := fmt.Sprintf("%s (%s) - %s:%d", item.Name, kindName, filePath, line)
	if item.Detail != "" {
		result += fmt.Sprintf(" [%s]", item.Detail)
	}
	return result
}

// formatIncomingCallsResult formats incomingCalls results.
func formatIncomingCallsResult(rawResult []byte, cwd string) (string, int, int) {
	var calls []platformlsp.CallHierarchyIncomingCall
	if err := json.Unmarshal(rawResult, &calls); err != nil || len(calls) == 0 {
		return "No incoming calls found (nothing calls this function).", 0, 0
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d incoming calls:\n", len(calls)))

	// Group by file.
	byFile := make(map[string][]platformlsp.CallHierarchyIncomingCall)
	for _, call := range calls {
		filePath := formatURI(call.From.URI, cwd)
		byFile[filePath] = append(byFile[filePath], call)
	}

	for filePath, fileCalls := range byFile {
		sb.WriteString(fmt.Sprintf("\n%s:\n", filePath))
		for _, call := range fileCalls {
			kindName := platformlsp.SymbolKindName(call.From.Kind)
			line := call.From.Range.Start.Line + 1
			callLine := fmt.Sprintf("  %s (%s) - Line %d", call.From.Name, kindName, line)
			if len(call.FromRanges) > 0 {
				sites := make([]string, len(call.FromRanges))
				for i, r := range call.FromRanges {
					sites[i] = fmt.Sprintf("%d:%d", r.Start.Line+1, r.Start.Character+1)
				}
				callLine += fmt.Sprintf(" [calls at: %s]", strings.Join(sites, ", "))
			}
			sb.WriteString(callLine + "\n")
		}
	}

	files := make(map[string]bool)
	for _, call := range calls {
		if call.From.URI != "" {
			files[call.From.URI] = true
		}
	}

	return sb.String(), len(calls), len(files)
}

// formatOutgoingCallsResult formats outgoingCalls results.
func formatOutgoingCallsResult(rawResult []byte, cwd string) (string, int, int) {
	var calls []platformlsp.CallHierarchyOutgoingCall
	if err := json.Unmarshal(rawResult, &calls); err != nil || len(calls) == 0 {
		return "No outgoing calls found (this function calls nothing).", 0, 0
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d outgoing calls:\n", len(calls)))

	// Group by file.
	byFile := make(map[string][]platformlsp.CallHierarchyOutgoingCall)
	for _, call := range calls {
		filePath := formatURI(call.To.URI, cwd)
		byFile[filePath] = append(byFile[filePath], call)
	}

	for filePath, fileCalls := range byFile {
		sb.WriteString(fmt.Sprintf("\n%s:\n", filePath))
		for _, call := range fileCalls {
			kindName := platformlsp.SymbolKindName(call.To.Kind)
			line := call.To.Range.Start.Line + 1
			callLine := fmt.Sprintf("  %s (%s) - Line %d", call.To.Name, kindName, line)
			if len(call.FromRanges) > 0 {
				sites := make([]string, len(call.FromRanges))
				for i, r := range call.FromRanges {
					sites[i] = fmt.Sprintf("%d:%d", r.Start.Line+1, r.Start.Character+1)
				}
				callLine += fmt.Sprintf(" [called from: %s]", strings.Join(sites, ", "))
			}
			sb.WriteString(callLine + "\n")
		}
	}

	files := make(map[string]bool)
	for _, call := range calls {
		if call.To.URI != "" {
			files[call.To.URI] = true
		}
	}

	return sb.String(), len(calls), len(files)
}

// groupByFile groups locations by their formatted file path.
func groupByFile(locs []platformlsp.Location, cwd string) map[string][]platformlsp.Location {
	byFile := make(map[string][]platformlsp.Location)
	for _, loc := range locs {
		filePath := formatURI(loc.URI, cwd)
		byFile[filePath] = append(byFile[filePath], loc)
	}
	return byFile
}

// countUniqueFiles counts unique file URIs from a slice of locations.
func countUniqueFiles(locs []platformlsp.Location) int {
	files := make(map[string]bool)
	for _, loc := range locs {
		if loc.URI != "" {
			files[loc.URI] = true
		}
	}
	return len(files)
}
