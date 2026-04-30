package lsp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	platformlsp "github.com/sheepzhao/claude-code-go/internal/platform/lsp"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const (
	// Name is the stable registry identifier for the LSP tool.
	Name = "LSP"
)

// toolDescription is the model-facing description of the LSP tool.
const toolDescription = `Interact with Language Server Protocol (LSP) servers to get code intelligence features.

Supported operations:
- goToDefinition: Find where a symbol is defined
- findReferences: Find all references to a symbol
- hover: Get hover information (documentation, type info) for a symbol
- documentSymbol: Get all symbols (functions, classes, variables) in a document
- workspaceSymbol: Search for symbols across the entire workspace
- goToImplementation: Find implementations of an interface or abstract method
- prepareCallHierarchy: Get call hierarchy item at a position (functions/methods)
- incomingCalls: Find all functions/methods that call the function at a position
- outgoingCalls: Find all functions/methods called by the function at a position

All operations require:
- filePath: The file to operate on
- line: The line number (1-based, as shown in editors)
- character: The character offset (1-based, as shown in editors)

Note: LSP servers must be configured for the file type. If no server is available, an error will be returned.`

// Input is the typed request payload for the LSP tool.
type Input struct {
	Operation string  `json:"operation"`
	FilePath  string  `json:"filePath"`
	Line      float64 `json:"line"`
	Character float64 `json:"character"`
}

// Output is the structured result returned by the LSP tool.
type Output struct {
	Operation   string `json:"operation"`
	Result      string `json:"result"`
	FilePath    string `json:"filePath"`
	ResultCount int    `json:"resultCount,omitempty"`
	FileCount   int    `json:"fileCount,omitempty"`
}

// validOperations is the set of LSP operations supported by this tool.
var validOperations = map[string]bool{
	"goToDefinition":      true,
	"findReferences":      true,
	"hover":               true,
	"documentSymbol":      true,
	"workspaceSymbol":     true,
	"goToImplementation":  true,
	"prepareCallHierarchy": true,
	"incomingCalls":       true,
	"outgoingCalls":       true,
}

// Manager is the shared LSP server manager used by the LSP tool.
// It must be set before the tool is used.
var Manager *platformlsp.Manager

// Tool implements the LSP code intelligence tool.
type Tool struct{}

// NewTool constructs an LSP tool instance.
func NewTool() *Tool {
	return &Tool{}
}

// Name returns the stable registration name.
func (t *Tool) Name() string {
	return Name
}

// Description returns the tool summary exposed to provider tool schemas.
func (t *Tool) Description() string {
	return toolDescription
}

// InputSchema returns the LSP tool input contract.
func (t *Tool) InputSchema() coretool.InputSchema {
	return inputSchema()
}

// IsReadOnly reports that LSP operations do not mutate external state.
func (t *Tool) IsReadOnly() bool {
	return true
}

// IsConcurrencySafe reports that independent invocations may run in parallel safely.
func (t *Tool) IsConcurrencySafe() bool {
	return true
}

// Invoke performs an LSP code intelligence operation. It validates the input,
// opens the target file in the LSP server, sends the appropriate request,
// and formats the result.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	if t == nil {
		return coretool.Result{}, fmt.Errorf("lsp tool: nil receiver")
	}

	input, err := coretool.DecodeInput[Input](inputSchema(), call.Input)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	if !validOperations[input.Operation] {
		return coretool.Result{
			Error: fmt.Sprintf("invalid operation: %q. Must be one of: goToDefinition, findReferences, hover, documentSymbol, workspaceSymbol, goToImplementation, prepareCallHierarchy, incomingCalls, outgoingCalls", input.Operation),
		}, nil
	}

	if Manager == nil {
		return coretool.Result{
			Output: "LSP server manager not initialized. This may indicate a startup issue.",
			Meta: map[string]any{
				"data": Output{
					Operation: input.Operation,
					Result:    "LSP server manager not initialized.",
					FilePath:  input.FilePath,
				},
			},
		}, nil
	}

	absPath := input.FilePath
	if !filepath.IsAbs(absPath) {
		cwd, _ := os.Getwd()
		absPath = filepath.Join(cwd, absPath)
	}

	// Validate the file exists and is readable.
	info, err := os.Stat(absPath)
	if err != nil {
		return coretool.Result{
			Error: fmt.Sprintf("File does not exist: %s", input.FilePath),
		}, nil
	}
	if info.IsDir() {
		return coretool.Result{
			Error: fmt.Sprintf("Path is not a file: %s", input.FilePath),
		}, nil
	}

	// Read file content and open it in the LSP server.
	content, err := os.ReadFile(absPath)
	if err != nil {
		return coretool.Result{
			Error: fmt.Sprintf("Cannot read file: %s: %v", input.FilePath, err),
		}, nil
	}

	if !Manager.IsFileOpen(absPath) {
		if err := Manager.OpenFile(absPath, string(content)); err != nil {
			logger.DebugCF("lsp.tool", "openFile failed", map[string]any{
				"file":  absPath,
				"error": err.Error(),
			})
		}
	}

	// Map operation to LSP method and params.
	method, params := getMethodAndParams(input.Operation, absPath, int(input.Line), int(input.Character))

	// Send the request.
	rawResult, err := Manager.SendRequest(absPath, method, params)
	if err != nil {
		logger.DebugCF("lsp.tool", "request failed", map[string]any{
			"operation": input.Operation,
			"method":    method,
			"error":     err.Error(),
		})
		return coretool.Result{
			Output: fmt.Sprintf("Error performing %s: %v", input.Operation, err),
			Meta: map[string]any{
				"data": Output{
					Operation: input.Operation,
					Result:    fmt.Sprintf("Error performing %s: %v", input.Operation, err),
					FilePath:  input.FilePath,
				},
			},
		}, nil
	}

	if rawResult == nil {
		ext := filepath.Ext(absPath)
		return coretool.Result{
			Output: fmt.Sprintf("No LSP server available for file type: %s", ext),
			Meta: map[string]any{
				"data": Output{
					Operation: input.Operation,
					Result:    fmt.Sprintf("No LSP server available for file type: %s", ext),
					FilePath:  input.FilePath,
				},
			},
		}, nil
	}

	// Handle call hierarchy two-step operations.
	if input.Operation == "incomingCalls" || input.Operation == "outgoingCalls" {
		rawResult, err = handleCallHierarchyStep2(Manager, absPath, input.Operation, rawResult)
		if err != nil {
			return coretool.Result{
				Output: fmt.Sprintf("Error performing %s: %v", input.Operation, err),
				Meta: map[string]any{
					"data": Output{
						Operation: input.Operation,
						Result:    fmt.Sprintf("Error: %v", err),
						FilePath:  input.FilePath,
					},
				},
			}, nil
		}
	}

	// Format the result.
	cwd, _ := os.Getwd()
	formatted, resultCount, fileCount := formatResult(input.Operation, rawResult, cwd)

	return coretool.Result{
		Output: formatted,
		Meta: map[string]any{
			"data": Output{
				Operation:   input.Operation,
				Result:      formatted,
				FilePath:    input.FilePath,
				ResultCount: resultCount,
				FileCount:   fileCount,
			},
		},
	}, nil
}

// getMethodAndParams maps an LSPTool operation to the corresponding LSP method
// and parameters. Line and character are 1-based (user-facing) and get converted
// to 0-based (LSP protocol).
func getMethodAndParams(operation, fileURI string, line, character int) (string, any) {
	// Convert to 0-based LSP protocol positions.
	pos := platformlsp.Position{
		Line:      line - 1,
		Character: character - 1,
	}

	type textDocumentPositionParams struct {
		TextDocument platformlsp.TextDocumentIdentifier `json:"textDocument"`
		Position     platformlsp.Position               `json:"position"`
	}

	type referenceParams struct {
		TextDocument platformlsp.TextDocumentIdentifier `json:"textDocument"`
		Position     platformlsp.Position               `json:"position"`
		Context      map[string]any                     `json:"context"`
	}

	type documentSymbolParams struct {
		TextDocument platformlsp.TextDocumentIdentifier `json:"textDocument"`
	}

	td := platformlsp.TextDocumentIdentifier{URI: "file://" + fileURI}

	switch operation {
	case "goToDefinition":
		return "textDocument/definition", textDocumentPositionParams{
			TextDocument: td,
			Position:     pos,
		}
	case "findReferences":
		return "textDocument/references", referenceParams{
			TextDocument: td,
			Position:     pos,
			Context:      map[string]any{"includeDeclaration": true},
		}
	case "hover":
		return "textDocument/hover", textDocumentPositionParams{
			TextDocument: td,
			Position:     pos,
		}
	case "documentSymbol":
		return "textDocument/documentSymbol", documentSymbolParams{
			TextDocument: td,
		}
	case "workspaceSymbol":
		return "workspace/symbol", map[string]any{"query": ""}
	case "goToImplementation":
		return "textDocument/implementation", textDocumentPositionParams{
			TextDocument: td,
			Position:     pos,
		}
	case "prepareCallHierarchy":
		return "textDocument/prepareCallHierarchy", textDocumentPositionParams{
			TextDocument: td,
			Position:     pos,
		}
	case "incomingCalls":
		// First step: prepareCallHierarchy
		return "textDocument/prepareCallHierarchy", textDocumentPositionParams{
			TextDocument: td,
			Position:     pos,
		}
	case "outgoingCalls":
		// First step: prepareCallHierarchy
		return "textDocument/prepareCallHierarchy", textDocumentPositionParams{
			TextDocument: td,
			Position:     pos,
		}
	default:
		return "", nil
	}
}

// handleCallHierarchyStep2 performs the second step of call hierarchy:
// using the CallHierarchyItem from prepareCallHierarchy to request
// incoming or outgoing calls.
func handleCallHierarchyStep2(mgr *platformlsp.Manager, filePath, operation string, prepareResult []byte) ([]byte, error) {
	// Parse the prepareCallHierarchy result (CallHierarchyItem[]).
	var items []platformlsp.CallHierarchyItem
	if err := parseJSON(prepareResult, &items); err != nil || len(items) == 0 {
		return nil, fmt.Errorf("no call hierarchy item found at this position")
	}

	item := items[0]
	var method string
	if operation == "incomingCalls" {
		method = "callHierarchy/incomingCalls"
	} else {
		method = "callHierarchy/outgoingCalls"
	}

	return mgr.SendRequest(filePath, method, map[string]any{"item": item})
}

func inputSchema() coretool.InputSchema {
	return coretool.InputSchema{
		Properties: map[string]coretool.FieldSchema{
			"operation": {
				Type:        coretool.ValueKindString,
				Description: "The LSP operation to perform: goToDefinition, findReferences, hover, documentSymbol, workspaceSymbol, goToImplementation, prepareCallHierarchy, incomingCalls, outgoingCalls",
				Required:    true,
			},
			"filePath": {
				Type:        coretool.ValueKindString,
				Description: "The absolute or relative path to the file",
				Required:    true,
			},
			"line": {
				Type:        coretool.ValueKindInteger,
				Description: "The line number (1-based, as shown in editors)",
				Required:    true,
			},
			"character": {
				Type:        coretool.ValueKindInteger,
				Description: "The character offset (1-based, as shown in editors)",
				Required:    true,
			},
		},
	}
}
