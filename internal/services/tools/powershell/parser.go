package powershell

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
	"unicode/utf16"
)

// =============================================================================
// Raw JSON types — match the output of the PS1 parse script
// =============================================================================

type rawParsedOutput struct {
	Valid              bool              `json:"valid"`
	Errors             []rawParseError   `json:"errors"`
	Statements         []rawStatement    `json:"statements"`
	Variables          []rawVariable     `json:"variables"`
	HasStopParsing     bool              `json:"hasStopParsing"`
	OriginalCommand    string            `json:"originalCommand"`
	TypeLiterals       []string          `json:"typeLiterals,omitempty"`
	HasUsingStatements bool              `json:"hasUsingStatements,omitempty"`
}

type rawParseError struct {
	Message string `json:"message"`
	ErrorID string `json:"errorId"`
}

type rawVariable struct {
	Path      string `json:"path"`
	IsSplatted bool   `json:"isSplatted"`
}

type rawStatement struct {
	Type             string              `json:"type"`
	Text             string              `json:"text"`
	Elements         []rawPipelineElement `json:"elements,omitempty"`
	NestedCommands   []rawPipelineElement `json:"nestedCommands,omitempty"`
	Redirections     []rawRedirection    `json:"redirections,omitempty"`
	SecurityPatterns *rawSecurityPatterns `json:"securityPatterns,omitempty"`
}

type rawPipelineElement struct {
	Type            string             `json:"type"`
	Text            string             `json:"text"`
	CommandElements []rawCommandElement `json:"commandElements,omitempty"`
	Redirections    []rawRedirection   `json:"redirections,omitempty"`
	ExpressionType  string             `json:"expressionType,omitempty"`
}

type rawCommandElement struct {
	Type           string              `json:"type"`
	Text           string              `json:"text"`
	Value          string              `json:"value,omitempty"`
	ExpressionType string              `json:"expressionType,omitempty"`
	Children       []rawCommandChild   `json:"children,omitempty"`
}

type rawCommandChild struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type rawRedirection struct {
	Type         string `json:"type"`
	Append       *bool  `json:"append,omitempty"`
	FromStream   string `json:"fromStream,omitempty"`
	LocationText string `json:"locationText,omitempty"`
}

type rawSecurityPatterns struct {
	HasMemberInvocations  *bool `json:"hasMemberInvocations,omitempty"`
	HasSubExpressions     *bool `json:"hasSubExpressions,omitempty"`
	HasExpandableStrings  *bool `json:"hasExpandableStrings,omitempty"`
	HasScriptBlocks       *bool `json:"hasScriptBlocks,omitempty"`
}

// =============================================================================
// Parsed AST types — same structure as TS parser.ts Parsed* types
// =============================================================================

// ParsedCommandElement is a command invocation within a pipeline segment.
type ParsedCommandElement struct {
	Name         string   `json:"name"`
	NameType     string   `json:"nameType"` // "cmdlet", "application", "unknown"
	ElementType  string   `json:"elementType"` // "CommandAst", "CommandExpressionAst", etc.
	Args         []string `json:"args"`
	Text         string   `json:"text"`
	ElementTypes []string `json:"elementTypes,omitempty"` // AST types for each element
}

// ParsedRedirection is a file redirection found in the command.
type ParsedRedirection struct {
	Operator   string `json:"operator"`   // ">", ">>", "2>", etc.
	Target     string `json:"target"`     // the file path
	IsMerging  bool   `json:"isMerging"`  // e.g., 2>&1
}

// ParsedStatement is a parsed statement from PowerShell.
type ParsedStatement struct {
	StatementType  string                `json:"statementType"`
	Commands       []ParsedCommandElement `json:"commands"`
	Redirections   []ParsedRedirection   `json:"redirections"`
	Text           string                `json:"text"`
	NestedCommands []ParsedCommandElement `json:"nestedCommands,omitempty"`
	SecurityPatterns *SecurityPatterns   `json:"securityPatterns,omitempty"`
}

// SecurityPatterns holds security-relevant patterns found by the PS1 parse script.
type SecurityPatterns struct {
	HasMemberInvocations  bool `json:"hasMemberInvocations,omitempty"`
	HasSubExpressions     bool `json:"hasSubExpressions,omitempty"`
	HasExpandableStrings  bool `json:"hasExpandableStrings,omitempty"`
	HasScriptBlocks       bool `json:"hasScriptBlocks,omitempty"`
}

// ParsedPowerShellCommand is the complete parsed result.
type ParsedPowerShellCommand struct {
	Valid              bool              `json:"valid"`
	Errors             []parseError      `json:"errors"`
	Statements         []ParsedStatement `json:"statements"`
	Variables          []rawVariable     `json:"variables"`
	HasStopParsing     bool              `json:"hasStopParsing"`
	OriginalCommand    string            `json:"originalCommand"`
	TypeLiterals       []string          `json:"typeLiterals,omitempty"`
	HasUsingStatements bool              `json:"hasUsingStatements,omitempty"`
}

// SecurityFlags holds security-relevant flags derived from the parsed command.
type SecurityFlags struct {
	HasSubExpressions    bool
	HasScriptBlocks      bool
	HasSplatting         bool
	HasExpandableStrings bool
	HasMemberInvocations bool
	HasAssignments       bool
	HasStopParsing       bool
}

type parseError struct {
	Message string `json:"message"`
	ErrorID string `json:"errorId"`
}

// =============================================================================
// PS1 Parse script
// =============================================================================

// parseScriptBody is the PowerShell script used to parse commands using
// PowerShell's native AST parser. Ported from TS parser.ts PARSE_SCRIPT_BODY.
const parseScriptBody = `
if (-not $EncodedCommand) {
    Write-Output '{"valid":false,"errors":[{"message":"No command provided","errorId":"NoInput"}],"statements":[],"variables":[],"hasStopParsing":false,"originalCommand":""}'
    exit 0
}

$Command = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($EncodedCommand))

$tokens = $null
$parseErrors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseInput(
    $Command,
    [ref]$tokens,
    [ref]$parseErrors
)

$allVariables = [System.Collections.ArrayList]::new()

function Get-RawCommandElements {
    param([System.Management.Automation.Language.CommandAst]$CmdAst)
    $elems = [System.Collections.ArrayList]::new()
    foreach ($ce in $CmdAst.CommandElements) {
        $ceData = @{ type = $ce.GetType().Name; text = $ce.Extent.Text }
        if ($ce.PSObject.Properties['Value'] -and $null -ne $ce.Value -and $ce.Value -is [string]) {
            $ceData.value = $ce.Value
        }
        if ($ce -is [System.Management.Automation.Language.CommandExpressionAst]) {
            $ceData.expressionType = $ce.Expression.GetType().Name
        }
        $a=$ce.Argument;if($a){$ceData.children=@(@{type=$a.GetType().Name;text=$a.Extent.Text})}
        [void]$elems.Add($ceData)
    }
    return $elems
}

function Get-RawRedirections {
    param($Redirections)
    $result = [System.Collections.ArrayList]::new()
    foreach ($redir in $Redirections) {
        $redirData = @{ type = $redir.GetType().Name }
        if ($redir -is [System.Management.Automation.Language.FileRedirectionAst]) {
            $redirData.append = [bool]$redir.Append
            $redirData.fromStream = $redir.FromStream.ToString()
            $redirData.locationText = $redir.Location.Extent.Text
        }
        [void]$result.Add($redirData)
    }
    return $result
}

function Get-SecurityPatterns($A) {
    $p = @{}
    foreach ($n in $A.FindAll({ param($x)
        $x -is [System.Management.Automation.Language.MemberExpressionAst] -or
        $x -is [System.Management.Automation.Language.SubExpressionAst] -or
        $x -is [System.Management.Automation.Language.ArrayExpressionAst] -or
        $x -is [System.Management.Automation.Language.ExpandableStringExpressionAst] -or
        $x -is [System.Management.Automation.Language.ScriptBlockExpressionAst] -or
        $x -is [System.Management.Automation.Language.ParenExpressionAst]
    }, $true)) { switch ($n.GetType().Name) {
        'InvokeMemberExpressionAst' { $p.hasMemberInvocations = $true }
        'MemberExpressionAst' { $p.hasMemberInvocations = $true }
        'SubExpressionAst' { $p.hasSubExpressions = $true }
        'ArrayExpressionAst' { $p.hasSubExpressions = $true }
        'ParenExpressionAst' { $p.hasSubExpressions = $true }
        'ExpandableStringExpressionAst' { $p.hasExpandableStrings = $true }
        'ScriptBlockExpressionAst' { $p.hasScriptBlocks = $true }
    }}
    if ($p.Count -gt 0) { return $p }
    return $null
}

$hasStopParsing = $false
$tk = [System.Management.Automation.Language.TokenKind]
foreach ($tok in $tokens) {
    if ($tok.Kind -eq $tk::MinusMinus) { $hasStopParsing = $true; break }
    if ($tok.Kind -eq $tk::Generic -and ($tok.Text -replace "[–—―]","-") -eq "--%") {
        $hasStopParsing = $true; break
    }
}

$statements = [System.Collections.ArrayList]::new()

function Process-BlockStatements {
    param($Block)
    if (-not $Block) { return }

    foreach ($stmt in $Block.Statements) {
        $statement = @{
            type = $stmt.GetType().Name
            text = $stmt.Extent.Text
        }

        if ($stmt -is [System.Management.Automation.Language.PipelineAst]) {
            $elements = [System.Collections.ArrayList]::new()
            foreach ($element in $stmt.PipelineElements) {
                $elemData = @{
                    type = $element.GetType().Name
                    text = $element.Extent.Text
                }

                if ($element -is [System.Management.Automation.Language.CommandAst]) {
                    $elemData.commandElements = @(Get-RawCommandElements -CmdAst $element)
                    $elemData.redirections = @(Get-RawRedirections -Redirections $element.Redirections)
                } elseif ($element -is [System.Management.Automation.Language.CommandExpressionAst]) {
                    $elemData.expressionType = $element.Expression.GetType().Name
                    $elemData.redirections = @(Get-RawRedirections -Redirections $element.Redirections)
                }

                [void]$elements.Add($elemData)
            }
            $statement.elements = @($elements)

            $allNestedCmds = $stmt.FindAll(
                { param($node) $node -is [System.Management.Automation.Language.CommandAst] },
                $true
            )
            $nestedCmds = [System.Collections.ArrayList]::new()
            foreach ($cmd in $allNestedCmds) {
                if ($cmd.Parent -eq $stmt) { continue }
                $nested = @{
                    type = $cmd.GetType().Name
                    text = $cmd.Extent.Text
                    commandElements = @(Get-RawCommandElements -CmdAst $cmd)
                    redirections = @(Get-RawRedirections -Redirections $cmd.Redirections)
                }
                [void]$nestedCmds.Add($nested)
            }
            if ($nestedCmds.Count -gt 0) {
                $statement.nestedCommands = @($nestedCmds)
            }
            $r = $stmt.FindAll({param($n) $n -is [System.Management.Automation.Language.FileRedirectionAst]}, $true)
            if ($r.Count -gt 0) {
                $rr = @(Get-RawRedirections -Redirections $r)
                $statement.redirections = if ($statement.redirections) { @($statement.redirections) + $rr } else { $rr }
            }
        } else {
            $nestedCmdAsts = $stmt.FindAll(
                { param($node) $node -is [System.Management.Automation.Language.CommandAst] },
                $true
            )
            $nested = [System.Collections.ArrayList]::new()
            foreach ($cmd in $nestedCmdAsts) {
                [void]$nested.Add(@{
                    type = "CommandAst"
                    text = $cmd.Extent.Text
                    commandElements = @(Get-RawCommandElements -CmdAst $cmd)
                    redirections = @(Get-RawRedirections -Redirections $cmd.Redirections)
                })
            }
            if ($nested.Count -gt 0) {
                $statement.nestedCommands = @($nested)
            }
            $r = $stmt.FindAll({param($n) $n -is [System.Management.Automation.Language.FileRedirectionAst]}, $true)
            if ($r.Count -gt 0) { $statement.redirections = @(Get-RawRedirections -Redirections $r) }
        }

        $sp = Get-SecurityPatterns $stmt
        if ($sp) { $statement.securityPatterns = $sp }

        [void]$statements.Add($statement)
    }

    if ($Block.Traps) {
        foreach ($trap in $Block.Traps) {
            $statement = @{
                type = "TrapStatementAst"
                text = $trap.Extent.Text
            }
            $nestedCmdAsts = $trap.FindAll(
                { param($node) $node -is [System.Management.Automation.Language.CommandAst] },
                $true
            )
            $nestedCmds = [System.Collections.ArrayList]::new()
            foreach ($cmd in $nestedCmdAsts) {
                $nested = @{
                    type = $cmd.GetType().Name
                    text = $cmd.Extent.Text
                    commandElements = @(Get-RawCommandElements -CmdAst $cmd)
                    redirections = @(Get-RawRedirections -Redirections $cmd.Redirections)
                }
                [void]$nestedCmds.Add($nested)
            }
            if ($nestedCmds.Count -gt 0) {
                $statement.nestedCommands = @($nestedCmds)
            }
            $r = $trap.FindAll({param($n) $n -is [System.Management.Automation.Language.FileRedirectionAst]}, $true)
            if ($r.Count -gt 0) { $statement.redirections = @(Get-RawRedirections -Redirections $r) }
            $sp = Get-SecurityPatterns $trap
            if ($sp) { $statement.securityPatterns = $sp }
            [void]$statements.Add($statement)
        }
    }
}

Process-BlockStatements -Block $ast.BeginBlock
Process-BlockStatements -Block $ast.ProcessBlock
Process-BlockStatements -Block $ast.EndBlock
Process-BlockStatements -Block $ast.CleanBlock
Process-BlockStatements -Block $ast.DynamicParamBlock

if ($ast.ParamBlock) {
  $pb = $ast.ParamBlock
  $pn = [System.Collections.ArrayList]::new()
  foreach ($c in $pb.FindAll({param($n) $n -is [System.Management.Automation.Language.CommandAst]}, $true)) {
    [void]$pn.Add(@{type="CommandAst";text=$c.Extent.Text;commandElements=@(Get-RawCommandElements -CmdAst $c);redirections=@(Get-RawRedirections -Redirections $c.Redirections)})
  }
  $pr = $pb.FindAll({param($n) $n -is [System.Management.Automation.Language.FileRedirectionAst]}, $true)
  $ps = Get-SecurityPatterns $pb
  if ($pn.Count -gt 0 -or $pr.Count -gt 0 -or $ps) {
    $st = @{type="ParamBlockAst";text=$pb.Extent.Text}
    if ($pn.Count -gt 0) { $st.nestedCommands = @($pn) }
    if ($pr.Count -gt 0) { $st.redirections = @(Get-RawRedirections -Redirections $pr) }
    if ($ps) { $st.securityPatterns = $ps }
    [void]$statements.Add($st)
  }
}

$hasUsingStatements = $ast.UsingStatements -and $ast.UsingStatements.Count -gt 0

$varExprs = $ast.FindAll({ param($node) $node -is [System.Management.Automation.Language.VariableExpressionAst] }, $true)
foreach ($v in $varExprs) {
    [void]$allVariables.Add(@{
        path = $v.VariablePath.ToString()
        isSplatted = [bool]$v.Splatted
    })
}

$typeLiterals = [System.Collections.ArrayList]::new()
foreach ($t in $ast.FindAll({ param($n)
    $n -is [System.Management.Automation.Language.TypeExpressionAst] -or
    $n -is [System.Management.Automation.Language.TypeConstraintAst]
}, $true)) { [void]$typeLiterals.Add($t.TypeName.FullName) }

$output = @{
    valid = ($parseErrors.Count -eq 0)
    errors = @($parseErrors | ForEach-Object {
        @{ message = $_.Message; errorId = $_.ErrorId }
    })
    statements = @($statements)
    variables = @($allVariables)
    hasStopParsing = $hasStopParsing
    originalCommand = $Command
    typeLiterals = @($typeLiterals)
    hasUsingStatements = $hasUsingStatements
}
$output | ConvertTo-Json -Compress
`

// buildParseScript creates the full PS1 script with the command embedded.
func buildParseScript(command string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(command))
	return fmt.Sprintf("$EncodedCommand = '%s'\n%s", encoded, parseScriptBody)
}

// toUtf16LeBase64 encodes a string as UTF-16LE base64 for -EncodedCommand.
func toUtf16LeBase64(text string) string {
	runes := []rune(text)
	u16 := utf16.Encode(runes)
	bytes := make([]byte, len(u16)*2)
	for i, r := range u16 {
		bytes[i*2] = byte(r)
		bytes[i*2+1] = byte(r >> 8)
	}
	return base64.StdEncoding.EncodeToString(bytes)
}

// ensureArray handles PowerShell's ConvertTo-Json unwrapping single-element arrays.
func _unused_ensureArray[T any](val any) []T {
	if val == nil {
		return nil
	}
	switch v := val.(type) {
	case []any:
		result := make([]T, 0, len(v))
		for _, item := range v {
			if typed, ok := item.(T); ok {
				result = append(result, typed)
			}
		}
		return result
	case T:
		return []T{v}
	}
	return nil
}

// =============================================================================
// Transform functions
// =============================================================================

func mapStatementType(rawType string) string {
	switch rawType {
	case "PipelineAst":
		return "PipelineAst"
	case "PipelineChainAst":
		return "PipelineChainAst"
	case "AssignmentStatementAst":
		return "AssignmentStatementAst"
	case "IfStatementAst":
		return "IfStatementAst"
	case "ForStatementAst":
		return "ForStatementAst"
	case "ForEachStatementAst":
		return "ForEachStatementAst"
	case "WhileStatementAst":
		return "WhileStatementAst"
	case "DoWhileStatementAst":
		return "DoWhileStatementAst"
	case "DoUntilStatementAst":
		return "DoUntilStatementAst"
	case "SwitchStatementAst":
		return "SwitchStatementAst"
	case "TryStatementAst":
		return "TryStatementAst"
	case "TrapStatementAst":
		return "TrapStatementAst"
	case "FunctionDefinitionAst":
		return "FunctionDefinitionAst"
	case "DataStatementAst":
		return "DataStatementAst"
	default:
		return "UnknownStatementAst"
	}
}

func mapElementType(rawType string) string {
	switch rawType {
	case "ScriptBlockExpressionAst":
		return "ScriptBlock"
	case "SubExpressionAst":
		return "SubExpression"
	case "ExpandableStringExpressionAst":
		return "ExpandableString"
	case "InvokeMemberExpressionAst", "MemberExpressionAst":
		return "MemberInvocation"
	case "VariableExpressionAst":
		return "Variable"
	case "StringConstantExpressionAst", "StringExpandableExpressionAst":
		return "StringConstant"
	case "CommandParameterAst":
		return "Parameter"
	default:
		return "Other"
	}
}

func classifyCommandName(name string) string {
	// If the name contains path separators, it's an application (script/exe)
	if strings.ContainsAny(name, "/\\") {
		return "application"
	}
	// If it has a file extension, it's an application
	if strings.Contains(name, ".") {
		return "application"
	}
	return "cmdlet"
}

func stripModulePrefix(name string) string {
	if idx := strings.LastIndex(name, "\\"); idx >= 0 {
		return name[idx+1:]
	}
	return name
}

func transformCommandElements(rawElems []rawCommandElement) ([]string, []string) {
	var args []string
	var types []string
	for _, e := range rawElems {
		args = append(args, e.Text)
		types = append(types, mapElementType(e.Type))
	}
	return args, types
}

func transformRedirections(rawReds []rawRedirection) []ParsedRedirection {
	var result []ParsedRedirection
	for _, r := range rawReds {
		pr := ParsedRedirection{}
		if r.Type == "MergingRedirectionAst" {
			pr.IsMerging = true
			pr.Operator = r.FromStream + ">&1"
			if pr.Operator == ">&1" {
				pr.Operator = "2>&1"
			}
		} else if r.LocationText != "" {
			pr.Target = r.LocationText
			if r.FromStream != "" && r.FromStream != "Output" {
				prefix := "1"
				switch r.FromStream {
				case "Error": prefix = "2"
				case "Warning": prefix = "3"
				case "Verbose": prefix = "4"
				case "Debug": prefix = "5"
				case "Information": prefix = "6"
				case "Progress": prefix = "7"
				}
				pr.Operator = prefix + ">"
			} else {
				pr.Operator = ">"
			}
			if r.Append != nil && *r.Append {
				pr.Operator += ">"
			}
		}
		result = append(result, pr)
	}
	return result
}

func transformPipelineElement(elem rawPipelineElement) *ParsedCommandElement {
	ce := &ParsedCommandElement{
		Text:        elem.Text,
		ElementType: elem.Type,
	}
	if len(elem.CommandElements) > 0 {
		first := elem.CommandElements[0]
		name := first.Text
		if first.Value != "" {
			name = first.Value
		}
		ce.Name = stripModulePrefix(name)
		ce.NameType = classifyCommandName(name)

		args, types := transformCommandElements(elem.CommandElements)
		// First element is the command name, skip it
		if len(args) > 0 {
			ce.Args = args[1:]
		}
		if len(types) > 0 {
			ce.ElementTypes = types[1:]
		}
	}
	return ce
}

func transformStatement(raw rawStatement) ParsedStatement {
	stmt := ParsedStatement{
		StatementType: mapStatementType(raw.Type),
		Text:          raw.Text,
	}

	// Transform ALL pipeline elements - preserve non-CommandAst elements
	// so checkPathConstraints can detect expression pipeline sources.
	for _, elem := range raw.Elements {
		if elem.Type == "CommandAst" {
			ce := transformPipelineElement(elem)
			if ce != nil {
				stmt.Commands = append(stmt.Commands, *ce)
			}
		} else {
			// Preserve non-CommandAst elements (CommandExpressionAst, etc.)
			// with their element type for expression source detection.
			stmt.Commands = append(stmt.Commands, ParsedCommandElement{
				Text:        elem.Text,
				ElementType: elem.Type,
			})
		}
	}

	// Transform nested commands from control flow
	for _, nc := range raw.NestedCommands {
		if nc.Type == "CommandAst" {
			ce := transformPipelineElement(nc)
			if ce != nil {
				stmt.NestedCommands = append(stmt.NestedCommands, *ce)
			}
		}
	}

	// Transform redirections
	if len(raw.Redirections) > 0 {
		stmt.Redirections = transformRedirections(raw.Redirections)
	}

	// Transform security patterns
	if raw.SecurityPatterns != nil {
		s := &SecurityPatterns{}
		if raw.SecurityPatterns.HasMemberInvocations != nil && *raw.SecurityPatterns.HasMemberInvocations {
			s.HasMemberInvocations = true
		}
		if raw.SecurityPatterns.HasSubExpressions != nil && *raw.SecurityPatterns.HasSubExpressions {
			s.HasSubExpressions = true
		}
		if raw.SecurityPatterns.HasExpandableStrings != nil && *raw.SecurityPatterns.HasExpandableStrings {
			s.HasExpandableStrings = true
		}
		if raw.SecurityPatterns.HasScriptBlocks != nil && *raw.SecurityPatterns.HasScriptBlocks {
			s.HasScriptBlocks = true
		}
		stmt.SecurityPatterns = s
	}

	return stmt
}

func transformRawOutput(raw rawParsedOutput) ParsedPowerShellCommand {
	result := ParsedPowerShellCommand{
		Valid:              raw.Valid,
		OriginalCommand:    raw.OriginalCommand,
		HasStopParsing:     raw.HasStopParsing,
		TypeLiterals:       raw.TypeLiterals,
		HasUsingStatements: raw.HasUsingStatements,
	}

	// Transform errors
	for _, e := range raw.Errors {
		result.Errors = append(result.Errors, parseError{
			Message: e.Message,
			ErrorID: e.ErrorID,
		})
	}

	// Transform variables
	result.Variables = raw.Variables

	// Transform statements
	for _, s := range raw.Statements {
		result.Statements = append(result.Statements, transformStatement(s))
	}

	return result
}

// =============================================================================
// Parser execution
// =============================================================================

// pwshParseTimeout is the timeout for pwsh parse calls.
// const pwshParseTimeout = 10 * time.Second

// ParsePowerShellCommand parses a PowerShell command using pwsh's native AST.
// Returns a ParsedPowerShellCommand with statements, commands, and element types.
// When pwsh is unavailable or parsing fails, returns an invalid result.
func ParsePowerShellCommand(command string) ParsedPowerShellCommand {
	// Check for pwsh availability
	pwshPath, err := exec.LookPath("pwsh")
	if err != nil {
		// Try powershell.exe on Windows
		pwshPath, err = exec.LookPath("powershell.exe")
		if err != nil {
			return ParsedPowerShellCommand{
				Valid:           false,
				Errors:          []parseError{{Message: "PowerShell is not available", ErrorID: "NoPowerShell"}},
				OriginalCommand: command,
			}
		}
	}

	// Build the script with the command embedded
	script := buildParseScript(command)
	encodedScript := toUtf16LeBase64(script)

	// Call pwsh with -EncodedCommand (5s timeout)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, pwshPath, "-NoProfile", "-NonInteractive", "-NoLogo", "-EncodedCommand", encodedScript)
	output, err := cmd.Output()
	if err != nil {
		return ParsedPowerShellCommand{
			Valid:           false,
			Errors:          []parseError{{Message: fmt.Sprintf("pwsh execution failed: %v", err), ErrorID: "PwshError"}},
			OriginalCommand: command,
		}
	}

	// Parse JSON output
	var raw rawParsedOutput
	if err := json.Unmarshal(output, &raw); err != nil {
		return ParsedPowerShellCommand{
			Valid:           false,
			Errors:          []parseError{{Message: fmt.Sprintf("JSON parse failed: %v", err), ErrorID: "JsonError"}},
			OriginalCommand: command,
		}
	}

	return transformRawOutput(raw)
}

// DeriveSecurityFlags derives security-relevant flags from the parsed command structure.
// Ported from TS parser.ts deriveSecurityFlags.
func DeriveSecurityFlags(parsed ParsedPowerShellCommand) SecurityFlags {
	flags := SecurityFlags{
		HasStopParsing: parsed.HasStopParsing,
	}

	checkElements := func(cmd ParsedCommandElement) {
		if cmd.ElementTypes == nil {
			return
		}
		for _, et := range cmd.ElementTypes {
			switch et {
			case "ScriptBlock":
				flags.HasScriptBlocks = true
			case "SubExpression":
				flags.HasSubExpressions = true
			case "ExpandableString":
				flags.HasExpandableStrings = true
			case "MemberInvocation":
				flags.HasMemberInvocations = true
			}
		}
	}

	for _, stmt := range parsed.Statements {
		if stmt.StatementType == "AssignmentStatementAst" {
			flags.HasAssignments = true
		}
		for _, cmd := range stmt.Commands {
			checkElements(cmd)
		}
		if stmt.NestedCommands != nil {
			for _, cmd := range stmt.NestedCommands {
				checkElements(cmd)
			}
		}
		// securityPatterns provides a belt-and-suspenders check
		if stmt.SecurityPatterns != nil {
			if stmt.SecurityPatterns.HasMemberInvocations {
				flags.HasMemberInvocations = true
			}
			if stmt.SecurityPatterns.HasSubExpressions {
				flags.HasSubExpressions = true
			}
			if stmt.SecurityPatterns.HasExpandableStrings {
				flags.HasExpandableStrings = true
			}
			if stmt.SecurityPatterns.HasScriptBlocks {
				flags.HasScriptBlocks = true
			}
		}
	}

	for _, v := range parsed.Variables {
		if v.IsSplatted {
			flags.HasSplatting = true
			break
		}
	}

	return flags
}
