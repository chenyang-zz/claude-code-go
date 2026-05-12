package powershell

import (
	"strings"
)

// =============================================================================
// CMDLET_PATH_CONFIG — maps cmdlets to their path-related parameter configs.
// Ported from TS pathValidation.ts CMDLET_PATH_CONFIG.
// =============================================================================

// operationType describes the file operation type for a cmdlet.
type operationType string

const (
	opWrite operationType = "write"
	opRead  operationType = "read"
)

// cmdletPathConfig stores path-related parameter info for one cmdlet.
type cmdletPathConfig struct {
	OperationType    operationType
	PathParams       []string // e.g., "-Path", "-LiteralPath"
	KnownSwitches    []string // e.g., "-Force", "-Recurse"
	KnownValueParams []string // e.g., "-Filter", "-Encoding"
	// LeafOnlyPathParams are parameter names that accept a leaf filename resolved
	// by PowerShell relative to another parameter (not cwd). Safe to extract only
	// when the value is a simple leaf (no "/", "\", ".", "..").
	LeafOnlyPathParams []string
	// PositionalSkip is the number of leading positional arguments to skip
	// (NOT extracted as paths). Used for cmdlets where positional-0 is a
	// non-path value (e.g., Invoke-WebRequest's -Uri).
	PositionalSkip int
	// OptionalWrite, when true, means this cmdlet only writes to disk when a
	// pathParam is present. Without a path, it's effectively a read operation.
	OptionalWrite bool
}

// cmdletPathConfigs maps canonical cmdlet names to their path configs.
// Ported from TS pathValidation.ts CMDLET_PATH_CONFIG.
var cmdletPathConfigs = buildCmdletPathConfigs()

func buildCmdletPathConfigs() map[string]cmdletPathConfig {
	return map[string]cmdletPathConfig{
		// ─── Write/create operations ────────────────────
		"set-content": {
			OperationType: opWrite,
			PathParams:    []string{"-path", "-literalpath", "-pspath", "-lp"},
			KnownSwitches: []string{"-passthru", "-force", "-whatif", "-confirm", "-usetransaction", "-nonewline"},
			KnownValueParams: []string{"-value", "-filter", "-include", "-exclude", "-credential", "-encoding", "-stream"},
		},
		"add-content": {
			OperationType: opWrite,
			PathParams:    []string{"-path", "-literalpath", "-pspath", "-lp"},
			KnownSwitches: []string{"-passthru", "-force", "-whatif", "-confirm", "-usetransaction", "-nonewline"},
			KnownValueParams: []string{"-value", "-filter", "-include", "-exclude", "-credential", "-encoding", "-stream"},
		},
		"remove-item": {
			OperationType: opWrite,
			PathParams:    []string{"-path", "-literalpath", "-pspath", "-lp"},
			KnownSwitches: []string{"-recurse", "-force", "-whatif", "-confirm", "-usetransaction"},
			KnownValueParams: []string{"-filter", "-include", "-exclude", "-credential", "-stream"},
		},
		"clear-content": {
			OperationType: opWrite,
			PathParams:    []string{"-path", "-literalpath", "-pspath", "-lp"},
			KnownSwitches: []string{"-force", "-whatif", "-confirm", "-usetransaction"},
			KnownValueParams: []string{"-filter", "-include", "-exclude", "-credential", "-stream"},
		},
		"out-file": {
			OperationType: opWrite,
			PathParams:    []string{"-filepath", "-path", "-literalpath", "-pspath", "-lp"},
			KnownSwitches: []string{"-append", "-force", "-noclobber", "-nonewline", "-whatif", "-confirm"},
			KnownValueParams: []string{"-inputobject", "-encoding", "-width"},
		},
		"tee-object": {
			OperationType: opWrite,
			PathParams:    []string{"-filepath", "-path", "-literalpath", "-pspath", "-lp"},
			KnownSwitches: []string{"-append"},
			KnownValueParams: []string{"-inputobject", "-variable", "-encoding"},
		},
		"export-csv": {
			OperationType: opWrite,
			PathParams:    []string{"-path", "-literalpath", "-pspath", "-lp"},
			KnownSwitches: []string{"-append", "-force", "-noclobber", "-notypeinformation", "-includetypeinformation", "-useculture", "-noheader", "-whatif", "-confirm"},
			KnownValueParams: []string{"-inputobject", "-delimiter", "-encoding"},
		},
		"new-item": {
			OperationType: opWrite,
			PathParams:    []string{"-path", "-literalpath", "-pspath", "-lp"},
			KnownSwitches: []string{"-force", "-whatif", "-confirm", "-usetransaction"},
			KnownValueParams: []string{"-itemtype", "-value", "-credential"},
		},
		"copy-item": {
			OperationType: opWrite,
			PathParams:    []string{"-path", "-literalpath", "-pspath", "-lp", "-destination"},
			KnownSwitches: []string{"-container", "-force", "-passthru", "-recurse", "-whatif", "-confirm", "-usetransaction"},
			KnownValueParams: []string{"-filter", "-include", "-exclude", "-credential"},
		},
		"move-item": {
			OperationType: opWrite,
			PathParams:    []string{"-path", "-literalpath", "-pspath", "-lp", "-destination"},
			KnownSwitches: []string{"-force", "-passthru", "-whatif", "-confirm", "-usetransaction"},
			KnownValueParams: []string{"-filter", "-include", "-exclude", "-credential"},
		},
		"rename-item": {
			OperationType: opWrite,
			PathParams:    []string{"-path", "-literalpath", "-pspath", "-lp"},
			KnownSwitches: []string{"-force", "-passthru", "-whatif", "-confirm"},
			KnownValueParams: []string{"-newname", "-credential"},
		},
		// ─── Read operations ────────────────────────────
		"get-content": {
			OperationType: opRead,
			PathParams:    []string{"-path", "-literalpath", "-pspath", "-lp"},
			KnownSwitches: []string{"-force"},
			KnownValueParams: []string{"-filter", "-include", "-exclude", "-encoding", "-totalcount", "-head", "-tail", "-raw"},
		},
		"get-childitem": {
			OperationType: opRead,
			PathParams:    []string{"-path", "-literalpath", "-pspath", "-lp"},
			KnownSwitches: []string{"-recurse", "-force", "-name", "-directory", "-file", "-hidden", "-readonly", "-system"},
			KnownValueParams: []string{"-filter", "-include", "-exclude", "-depth", "-attributes"},
		},
		"get-item": {
			OperationType: opRead,
			PathParams:    []string{"-path", "-literalpath", "-pspath", "-lp"},
			KnownSwitches: []string{"-force"},
			KnownValueParams: []string{"-stream"},
		},
		"get-itemproperty": {
			OperationType: opRead,
			PathParams:    []string{"-path", "-literalpath", "-pspath", "-lp"},
			KnownValueParams: []string{"-name"},
		},
		"test-path": {
			OperationType: opRead,
			PathParams:    []string{"-path", "-literalpath", "-pspath", "-lp"},
			KnownValueParams: []string{"-pathtype", "-filter", "-include", "-exclude", "-isvalid", "-newerthan", "-olderthan"},
		},
		"resolve-path": {
			OperationType: opRead,
			PathParams:    []string{"-path", "-literalpath"},
			KnownSwitches: []string{"-relative"},
		},
		"select-string": {
			OperationType: opRead,
			PathParams:    []string{"-path", "-literalpath"},
			KnownValueParams: []string{"-pattern", "-inputobject", "-simplematch", "-casesensitive", "-encoding", "-context"},
		},
		"get-acl": {
			OperationType: opRead,
			PathParams:    []string{"-path", "-literalpath"},
			KnownValueParams: []string{"-filter", "-include", "-exclude"},
		},
		// ─── Additional write operations ────────────────────
		"export-clixml": {
			OperationType:    opWrite,
			PathParams:    []string{"-path", "-literalpath", "-pspath", "-lp"},
			KnownSwitches: []string{"-force", "-noclobber", "-whatif", "-confirm"},
			KnownValueParams: []string{"-inputobject", "-depth", "-encoding"},
		},
		"set-item": {
			OperationType:    opWrite,
			PathParams:    []string{"-path", "-literalpath", "-pspath", "-lp"},
			KnownSwitches: []string{"-force", "-passthru", "-whatif", "-confirm", "-usetransaction"},
			KnownValueParams: []string{"-value", "-credential", "-filter", "-include", "-exclude"},
		},
		// ─── Additional read operations ─────────────────────
		"get-itempropertyvalue": {
			OperationType: opRead,
			PathParams:    []string{"-path", "-literalpath", "-pspath", "-lp"},
			KnownValueParams: []string{"-name"},
		},
		"get-filehash": {
			OperationType: opRead,
			PathParams:    []string{"-path", "-literalpath", "-pspath", "-lp"},
			KnownValueParams: []string{"-algorithm", "-inputstream"},
		},
		"format-hex": {
			OperationType: opRead,
			PathParams:    []string{"-path", "-literalpath", "-pspath", "-lp"},
			KnownSwitches: []string{"-raw"},
			KnownValueParams: []string{"-inputobject", "-encoding", "-count", "-offset"},
		},
		"get-winevent": {
			OperationType: opRead,
			PathParams:    []string{"-path"},
			KnownSwitches: []string{"-force", "-oldest"},
			KnownValueParams: []string{"-listlog", "-logname", "-listprovider", "-providername", "-maxevents", "-filterxpath", "-filterxml", "-filterhashtable"},
		},
		"convert-path": {
			OperationType: opRead,
			PathParams:    []string{"-path", "-literalpath", "-pspath", "-lp"},
		},
		"set-location": {
			OperationType:    opRead,
			PathParams:    []string{"-path", "-literalpath", "-pspath", "-lp"},
			KnownSwitches: []string{"-passthru", "-usetransaction"},
			KnownValueParams: []string{"-stackname"},
		},
		"push-location": {
			OperationType:    opRead,
			PathParams:    []string{"-path", "-literalpath", "-pspath", "-lp"},
			KnownSwitches: []string{"-passthru", "-usetransaction"},
			KnownValueParams: []string{"-stackname"},
		},
		"pop-location": {
			OperationType:    opRead,
			PathParams:    []string{},
			KnownSwitches: []string{"-passthru", "-usetransaction"},
			KnownValueParams: []string{"-stackname"},
		},
		// ─── Web/network cmdlets ───────────────────────────
		"invoke-webrequest": {
			OperationType:   opWrite,
			PathParams:    []string{"-outfile", "-infile"},
			PositionalSkip: 1,
			OptionalWrite: true,
			KnownSwitches: []string{"-allowinsecureredirect", "-disablekeepalive", "-passthru", "-resume", "-skipcertificatecheck", "-skipheadervalidation", "-skiphttperrorcheck", "-usebasicparsing", "-usedefaultcredentials"},
			KnownValueParams: []string{"-uri", "-method", "-body", "-contenttype", "-headers", "-maximumredirection", "-maximumretrycount", "-proxy", "-proxycredential", "-retryintervalsec", "-timeoutsec", "-useragent", "-credential", "-authentication"},
		},
		"invoke-restmethod": {
			OperationType:   opWrite,
			PathParams:    []string{"-outfile", "-infile"},
			PositionalSkip: 1,
			OptionalWrite: true,
			KnownSwitches: []string{"-allowinsecureredirect", "-disablekeepalive", "-passthru", "-resume", "-skipcertificatecheck", "-skipheadervalidation", "-skiphttperrorcheck", "-usebasicparsing", "-usedefaultcredentials"},
			KnownValueParams: []string{"-uri", "-method", "-body", "-contenttype", "-headers", "-maximumredirection", "-maximumretrycount", "-proxy", "-proxycredential", "-retryintervalsec", "-timeoutsec", "-useragent", "-credential", "-authentication"},
		},
		// ─── Archive cmdlets ────────────────────────────────
		"expand-archive": {
			OperationType:    opWrite,
			PathParams:    []string{"-path", "-literalpath", "-pspath", "-lp", "-destinationpath"},
			KnownSwitches: []string{"-force", "-passthru", "-whatif", "-confirm"},
		},
		"compress-archive": {
			OperationType:    opWrite,
			PathParams:    []string{"-path", "-literalpath", "-pspath", "-lp", "-destinationpath"},
			KnownSwitches: []string{"-force", "-update", "-passthru", "-whatif", "-confirm"},
			KnownValueParams: []string{"-compressionlevel"},
		},
		// ─── *-ItemProperty cmdlets ─────────────────────────
		"set-itemproperty": {
			OperationType:    opWrite,
			PathParams:    []string{"-path", "-literalpath", "-pspath", "-lp"},
			KnownSwitches: []string{"-passthru", "-force", "-whatif", "-confirm", "-usetransaction"},
			KnownValueParams: []string{"-name", "-value", "-type", "-filter", "-include", "-exclude", "-credential"},
		},
		"new-itemproperty": {
			OperationType:    opWrite,
			PathParams:    []string{"-path", "-literalpath", "-pspath", "-lp"},
			KnownSwitches: []string{"-force", "-whatif", "-confirm", "-usetransaction"},
			KnownValueParams: []string{"-name", "-value", "-propertytype", "-type", "-filter", "-include", "-exclude", "-credential"},
		},
		"remove-itemproperty": {
			OperationType:    opWrite,
			PathParams:    []string{"-path", "-literalpath", "-pspath", "-lp"},
			KnownSwitches: []string{"-force", "-whatif", "-confirm", "-usetransaction"},
			KnownValueParams: []string{"-name", "-filter", "-include", "-exclude", "-credential"},
		},
		"clear-item": {
			OperationType:    opWrite,
			PathParams:    []string{"-path", "-literalpath", "-pspath", "-lp"},
			KnownSwitches: []string{"-force", "-whatif", "-confirm", "-usetransaction"},
			KnownValueParams: []string{"-filter", "-include", "-exclude", "-credential"},
		},
		"export-alias": {
			OperationType:    opWrite,
			PathParams:    []string{"-path", "-literalpath", "-pspath", "-lp"},
			KnownSwitches: []string{"-append", "-force", "-noclobber", "-passthru", "-whatif", "-confirm"},
			KnownValueParams: []string{"-name", "-description", "-scope", "-as"},
		},
	}
}

// isCmdletInPathConfig returns true when the cmdlet has a path config entry.
func isCmdletInPathConfig(canonical string) bool {
	_, ok := cmdletPathConfigs[canonical]
	return ok
}

// =============================================================================
// extractPathsFromCommand — extracts file paths from a parsed command element.
// Ported from TS pathValidation.ts extractPathsFromCommand.
// =============================================================================

// safePathElementTypes lists AST types whose values are safe for path extraction.
// StringConstant means a literal string path (safe).
var safePathElementTypes = map[string]bool{
	"StringConstant": true,
	"Parameter":      true,
}

// extractPathsResult holds the result of path extraction.
type extractPathsResult struct {
	Paths                   []string
	OperationType           operationType
	HasUnvalidatablePathArg bool
	OptionalWrite           bool
}

// extractPathsFromCommand extracts file paths from a parsed command element
// using CMDLET_PATH_CONFIG to identify path parameters.
func extractPathsFromCommand(cmd ParsedCommandElement) extractPathsResult {
	canonical := resolvePSCommand(cmd.Name)
	config, ok := cmdletPathConfigs[canonical]
	if !ok {
		return extractPathsResult{
			OperationType: opRead,
		}
	}

	var paths []string
	var hasUnvalidatable bool
	args := cmd.Args
	types := cmd.ElementTypes

	// Build param lookup maps
	switchParam := make(map[string]bool)
	for _, s := range config.KnownSwitches {
		switchParam[strings.ToLower(s)] = true
	}
	valueParam := make(map[string]bool)
	for _, v := range config.KnownValueParams {
		valueParam[strings.ToLower(v)] = true
	}
	pathParam := make(map[string]bool)
	for _, p := range config.PathParams {
		pathParam[strings.ToLower(p)] = true
	}
	leafOnlyParam := make(map[string]bool)
	for _, p := range config.LeafOnlyPathParams {
		leafOnlyParam[strings.ToLower(p)] = true
	}

	positionalsSeen := 0
	positionalSkip := config.PositionalSkip

	for i := 0; i < len(args); i++ {
		arg := args[i]
		argType := ""
		if i < len(types) {
			argType = types[i]
		}

		// Check if this arg is a parameter name
		if strings.HasPrefix(arg, "-") {
			paramLower := strings.ToLower(arg)

			// Handle colon-bound: -Path:value
			if colonIdx := strings.Index(paramLower, ":"); colonIdx > 0 {
				paramName := paramLower[:colonIdx]
				value := arg[colonIdx+1:]
				if pathParam[paramName] {
					// Colon-bound paths can't be validated for element type without children
					hasUnvalidatable = true
					paths = append(paths, value)
				} else if leafOnlyParam[paramName] {
					hasUnvalidatable = true
					paths = append(paths, value)
				}
				continue
			}

			// Check if known param
			if switchParam[paramLower] {
				continue
			}
			if pathParam[paramLower] || leafOnlyParam[paramLower] {
				isLeafOnly := leafOnlyParam[paramLower]
				// Next arg is the path value
				if i+1 < len(args) {
					nextType := ""
					if i+1 < len(types) {
						nextType = types[i+1]
					}
					nextArg := args[i+1]
					if !strings.HasPrefix(nextArg, "-") {
						if !safePathElementTypes[nextType] {
							hasUnvalidatable = true
						}
						// For leaf-only params, flag non-leaf values (/, \, ., ..)
						if isLeafOnly && (strings.Contains(nextArg, "/") || strings.Contains(nextArg, "\\") || nextArg == "." || nextArg == "..") {
							hasUnvalidatable = true
						}
						paths = append(paths, nextArg)
						i++ // skip the value
						continue
					}
				}
			}
			if valueParam[paramLower] {
				// Skip the value parameter
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
					i++
				}
				continue
			}
			continue // unknown param, skip
		}

		// Non-flag positional arg
		// Skip leading positionals that are non-path values (e.g., iwr's -Uri)
		if positionalsSeen < positionalSkip {
			positionalsSeen++
			continue
		}
		positionalsSeen++
		if !safePathElementTypes[argType] {
			hasUnvalidatable = true
		}
		paths = append(paths, arg)
	}

	return extractPathsResult{
		Paths:                   paths,
		OperationType:           config.OperationType,
		HasUnvalidatablePathArg: hasUnvalidatable,
		OptionalWrite:           config.OptionalWrite,
	}
}

// checkPathConstraintsResult holds the outcome of path constraint validation.
type checkPathConstraintsResult struct {
	ShouldAsk    bool
	Message      string
	ExtractedPaths []string
}

// checkPathConstraints validates file paths in a command using the AST parser
// and CMDLET_PATH_CONFIG. Returns ask when:
// - Expression pipeline sources are detected (variable piped to cmdlet)
// - Path arguments have unvalidatable element types (Variable, ScriptBlock)
// Dangerous removal targeting is handled separately by permissions.go Step 0.
// Ported from TS pathValidation.ts checkPathConstraints.
func checkPathConstraints(command string, parsed *ParsedPowerShellCommand) checkPathConstraintsResult {
	if parsed == nil || !parsed.Valid {
		return checkPathConstraintsResult{}
	}

	var allPaths []string
	var hasExpressionSource bool
	var hasUnvalidatable bool

	for _, stmt := range parsed.Statements {
		for _, cmd := range stmt.Commands {
			// Check for non-CommandAst pipeline elements (expression sources)
			if cmd.ElementType != "CommandAst" {
				hasExpressionSource = true
				continue
			}

			// Extract paths using CMDLET_PATH_CONFIG
			result := extractPathsFromCommand(cmd)
			allPaths = append(allPaths, result.Paths...)
			if result.HasUnvalidatablePathArg {
				hasUnvalidatable = true
			}
		}
	}

	// Expression pipeline source + path-operating cmdlet = unvalidatable path.
	// Also triggers when allPaths is empty (piped paths have no explicit args).
	if hasExpressionSource {
		return checkPathConstraintsResult{
			ShouldAsk:       true,
			Message:         "Command pipes data through the pipeline to a path-operating cmdlet — the piped path cannot be validated",
			ExtractedPaths:  allPaths,
		}
	}

	// Unvalidatable path args (Variable, ScriptBlock instead of StringConstant)
	if hasUnvalidatable {
		return checkPathConstraintsResult{
			ShouldAsk:       true,
			Message:         "Command uses variable or expression-based paths which cannot be statically validated",
			ExtractedPaths:  allPaths,
		}
	}

	return checkPathConstraintsResult{
		ExtractedPaths: allPaths,
	}
}
