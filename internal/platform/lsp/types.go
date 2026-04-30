package lsp

// Position represents a zero-based line and character offset within a document,
// as defined by the LSP 3.17 specification.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range represents a span between two positions within a document.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location represents a document location identified by a URI and a range.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// TextDocumentIdentifier identifies a text document by its file URI.
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// TextDocumentItem represents a text document sent to the LSP server for synchronization.
type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

// InitializeParams is the parameters sent with the initialize request.
type InitializeParams struct {
	ProcessID int    `json:"processId"`
	RootURI   string `json:"rootUri,omitempty"`
	RootPath  string `json:"rootPath,omitempty"`
	// Capabilities describes the features supported by the client.
	Capabilities ClientCapabilities `json:"capabilities"`
}

// ClientCapabilities describes the set of LSP features the client supports.
type ClientCapabilities struct {
	TextDocument *TextDocumentClientCapabilities `json:"textDocument,omitempty"`
}

// TextDocumentClientCapabilities describes text-document-specific capabilities.
type TextDocumentClientCapabilities struct {
	Definition *bool `json:"definition,omitempty"`
	References *bool `json:"references,omitempty"`
	Hover      *bool `json:"hover,omitempty"`
}

// InitializeResult is the response returned from the initialize request.
type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
}

// ServerCapabilities describes the set of features the server supports.
type ServerCapabilities struct {
	TextDocumentSync       *TextDocumentSyncOptions `json:"textDocumentSync,omitempty"`
	DefinitionProvider     bool                     `json:"definitionProvider,omitempty"`
	ReferencesProvider     bool                     `json:"referencesProvider,omitempty"`
	HoverProvider          bool                     `json:"hoverProvider,omitempty"`
	DocumentSymbolProvider bool                     `json:"documentSymbolProvider,omitempty"`
	WorkspaceSymbolProvider bool                    `json:"workspaceSymbolProvider,omitempty"`
	ImplementationProvider bool                     `json:"implementationProvider,omitempty"`
	CallHierarchyProvider  bool                     `json:"callHierarchyProvider,omitempty"`
}

// TextDocumentSyncOptions describes how the client synchronizes document changes.
type TextDocumentSyncOptions struct {
	OpenClose bool `json:"openClose,omitempty"`
	Change    int  `json:"change,omitempty"`
	Save      bool `json:"save,omitempty"`
}

// ShutdownParams is the parameters for the shutdown request (empty).
type ShutdownParams struct{}

// InitializedParams is the parameters for the initialized notification (empty).
type InitializedParams struct{}

// SymbolKind enumerates the kinds of symbols that can appear in LSP results.
type SymbolKind int

const (
	SymbolKindFile          SymbolKind = 1
	SymbolKindModule        SymbolKind = 2
	SymbolKindNamespace     SymbolKind = 3
	SymbolKindPackage       SymbolKind = 4
	SymbolKindClass         SymbolKind = 5
	SymbolKindMethod        SymbolKind = 6
	SymbolKindProperty      SymbolKind = 7
	SymbolKindField         SymbolKind = 8
	SymbolKindConstructor   SymbolKind = 9
	SymbolKindEnum          SymbolKind = 10
	SymbolKindInterface     SymbolKind = 11
	SymbolKindFunction      SymbolKind = 12
	SymbolKindVariable      SymbolKind = 13
	SymbolKindConstant      SymbolKind = 14
	SymbolKindString        SymbolKind = 15
	SymbolKindNumber        SymbolKind = 16
	SymbolKindBoolean       SymbolKind = 17
	SymbolKindArray         SymbolKind = 18
	SymbolKindObject        SymbolKind = 19
	SymbolKindKey           SymbolKind = 20
	SymbolKindNull          SymbolKind = 21
	SymbolKindEnumMember    SymbolKind = 22
	SymbolKindStruct        SymbolKind = 23
	SymbolKindEvent         SymbolKind = 24
	SymbolKindOperator      SymbolKind = 25
	SymbolKindTypeParameter SymbolKind = 26
)

// symbolKindNames maps SymbolKind values to human-readable names.
var symbolKindNames = map[SymbolKind]string{
	SymbolKindFile:          "File",
	SymbolKindModule:        "Module",
	SymbolKindNamespace:     "Namespace",
	SymbolKindPackage:       "Package",
	SymbolKindClass:         "Class",
	SymbolKindMethod:        "Method",
	SymbolKindProperty:      "Property",
	SymbolKindField:         "Field",
	SymbolKindConstructor:   "Constructor",
	SymbolKindEnum:          "Enum",
	SymbolKindInterface:     "Interface",
	SymbolKindFunction:      "Function",
	SymbolKindVariable:      "Variable",
	SymbolKindConstant:      "Constant",
	SymbolKindString:        "String",
	SymbolKindNumber:        "Number",
	SymbolKindBoolean:       "Boolean",
	SymbolKindArray:         "Array",
	SymbolKindObject:        "Object",
	SymbolKindKey:           "Key",
	SymbolKindNull:          "Null",
	SymbolKindEnumMember:    "EnumMember",
	SymbolKindStruct:        "Struct",
	SymbolKindEvent:         "Event",
	SymbolKindOperator:      "Operator",
	SymbolKindTypeParameter: "TypeParameter",
}

// SymbolKindName returns the human-readable name for a SymbolKind.
func SymbolKindName(k SymbolKind) string {
	if name, ok := symbolKindNames[k]; ok {
		return name
	}
	return "Unknown"
}

// DocumentSymbol represents a top-level or nested symbol in a document
// (hierarchical format, LSP 3.16+).
type DocumentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           SymbolKind       `json:"kind"`
	Tags           []int            `json:"tags,omitempty"`
	Deprecated     bool             `json:"deprecated,omitempty"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

// SymbolInformation represents a flat symbol entry (legacy format).
type SymbolInformation struct {
	Name          string     `json:"name"`
	Kind          SymbolKind `json:"kind"`
	Tags          []int      `json:"tags,omitempty"`
	Deprecated    bool       `json:"deprecated,omitempty"`
	Location      Location   `json:"location"`
	ContainerName string     `json:"containerName,omitempty"`
}

// MarkupContent represents formatted text content (e.g., Markdown or plaintext).
type MarkupContent struct {
	Kind  string `json:"kind"` // "markdown" or "plaintext"
	Value string `json:"value"`
}

// Hover contains the hover information for a position in a document.
type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

// CallHierarchyItem represents an item in a call hierarchy.
type CallHierarchyItem struct {
	Name           string     `json:"name"`
	Kind           SymbolKind `json:"kind"`
	Tags           []int      `json:"tags,omitempty"`
	Detail         string     `json:"detail,omitempty"`
	URI            string     `json:"uri"`
	Range          Range      `json:"range"`
	SelectionRange Range      `json:"selectionRange"`
	Data           any        `json:"data,omitempty"`
}

// CallHierarchyIncomingCall represents an incoming call to a function or method.
type CallHierarchyIncomingCall struct {
	From       CallHierarchyItem `json:"from"`
	FromRanges []Range           `json:"fromRanges"`
}

// CallHierarchyOutgoingCall represents an outgoing call from a function or method.
type CallHierarchyOutgoingCall struct {
	To         CallHierarchyItem `json:"to"`
	FromRanges []Range           `json:"fromRanges"`
}

// LocationLink represents a link from a source location to a target location.
type LocationLink struct {
	OriginSelectionRange *Range `json:"originSelectionRange,omitempty"`
	TargetURI            string `json:"targetUri"`
	TargetRange          Range  `json:"targetRange"`
	TargetSelectionRange Range  `json:"targetSelectionRange"`
}

// TextEdit represents a textual edit to apply to a document.
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// DiagnosticSeverity enumerates the severity levels defined in LSP DiagnosticSeverity.
type DiagnosticSeverity int

const (
	SeverityError       DiagnosticSeverity = 1
	SeverityWarning     DiagnosticSeverity = 2
	SeverityInformation DiagnosticSeverity = 3
	SeverityHint        DiagnosticSeverity = 4
)

// SeverityString returns the human-readable name for a DiagnosticSeverity.
func SeverityString(s DiagnosticSeverity) string {
	switch s {
	case SeverityError:
		return "Error"
	case SeverityWarning:
		return "Warning"
	case SeverityInformation:
		return "Info"
	case SeverityHint:
		return "Hint"
	default:
		return "Error"
	}
}

// Diagnostic represents a single diagnostic message (error, warning, info, hint)
// reported by an LSP server for a specific range in a text document.
type Diagnostic struct {
	Message  string             `json:"message"`
	Severity DiagnosticSeverity `json:"severity,omitempty"`
	Range    Range              `json:"range"`
	Source   string             `json:"source,omitempty"`
	Code     string             `json:"code,omitempty"`
}

// DiagnosticFile groups a set of diagnostics belonging to a single file identified by URI.
type DiagnosticFile struct {
	URI         string       `json:"uri"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// PublishDiagnosticsParams is the parameters sent with the
// textDocument/publishDiagnostics notification from an LSP server.
type PublishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}
