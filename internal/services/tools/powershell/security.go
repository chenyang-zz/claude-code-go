package powershell

import (
	"fmt"
	"regexp"
	"strings"
)

// RiskLevel describes the severity of a security finding.
type RiskLevel int

const (
	// RiskLevelSafe indicates no security concerns.
	RiskLevelSafe RiskLevel = iota
	// RiskLevelWarning indicates a potentially suspicious pattern.
	RiskLevelWarning
	// RiskLevelDangerous indicates a clearly dangerous pattern that should be blocked.
	RiskLevelDangerous
)

// ScanResult stores the outcome of a PowerShell security scan.
type ScanResult struct {
	// Level is the highest risk level found.
	Level RiskLevel
	// Message describes the finding when Level is not Safe.
	Message string
}

// SecurityScanner performs pre-execution security checks on PowerShell commands.
type SecurityScanner struct {
	validators []psSecurityValidator
}

// psSecurityValidator checks one class of dangerous PowerShell patterns.
type psSecurityValidator struct {
	name  string
	check func(command string) (RiskLevel, string)
}

// NewSecurityScanner creates a security scanner with all PS-specific validators.
// Ported from TS src/tools/PowerShellTool/powershellSecurity.ts (24 checks total).
func NewSecurityScanner() *SecurityScanner {
	return &SecurityScanner{
		validators: []psSecurityValidator{
			// --- Phase 1 (6 checks) ---
			{name: "encoded_command", check: checkEncodedCommand},
			{name: "invoke_expression", check: checkInvokeExpression},
			{name: "download_cradle", check: checkDownloadCradle},
			{name: "nested_powershell", check: checkNestedPowerShell},
			{name: "script_block_injection", check: checkScriptBlockInjection},
			{name: "add_type", check: checkAddType},
			// --- Phase 2 (18 additional checks) ---
			{name: "dynamic_command_name", check: checkDynamicCommandName},
			{name: "download_utilities", check: checkDownloadUtilities},
			{name: "com_object", check: checkComObject},
			{name: "dangerous_filepath", check: checkDangerousFilePathExecution},
			{name: "foreach_membername", check: checkForEachMemberName},
			{name: "start_process", check: checkStartProcess},
			{name: "subexpressions", check: checkSubExpressions},
			{name: "expandable_strings", check: checkExpandableStrings},
			{name: "splatting", check: checkSplatting},
			{name: "stop_parsing", check: checkStopParsing},
			{name: "member_invocations", check: checkMemberInvocations},
			{name: "type_literals", check: checkTypeLiterals},
			{name: "invoke_item", check: checkInvokeItem},
			{name: "scheduled_task", check: checkScheduledTask},
			{name: "env_var_manipulation", check: checkEnvVarManipulation},
			{name: "module_loading", check: checkModuleLoading},
			{name: "runtime_state_manipulation", check: checkRuntimeStateManipulation},
			{name: "wmi_process_spawn", check: checkWmiProcessSpawn},
		},
	}
}

// Scan runs all validators and returns the highest risk level found.
func (s *SecurityScanner) Scan(command string) ScanResult {
	if s == nil || len(s.validators) == 0 {
		return ScanResult{Level: RiskLevelSafe}
	}

	var highestRisk RiskLevel
	var worstMessage string

	for _, v := range s.validators {
		level, msg := v.check(command)
		if level > highestRisk {
			highestRisk = level
			worstMessage = msg
		}
		if highestRisk == RiskLevelDangerous {
			// Short-circuit on the most severe finding.
			return ScanResult{Level: RiskLevelDangerous, Message: worstMessage}
		}
	}

	if highestRisk > RiskLevelSafe {
		return ScanResult{Level: highestRisk, Message: worstMessage}
	}
	return ScanResult{Level: RiskLevelSafe}
}

// =============================================================================
// Phase 1 validators (6 checks — ported in initial implementation)
// =============================================================================

// encodedCommandRe detects -EncodedCommand parameters which obscure intent.
var encodedCommandRe = regexp.MustCompile(`(?i)(?:^|\s)(?:pwsh|powershell|powershell\.exe)(?:\s|\S)*?\s-(?:e|enc|encodedcommand)\b`)

func checkEncodedCommand(command string) (RiskLevel, string) {
	if encodedCommandRe.MatchString(command) {
		return RiskLevelDangerous, "PowerShell command uses -EncodedCommand which obscures intent"
	}

	// Also catch standalone encoded command usage
	if matches := regexp.MustCompile(`(?i)-e(ncodedcommand)?\b`).FindStringSubmatch(command); matches != nil {
		return RiskLevelDangerous, "PowerShell command uses -EncodedCommand which obscures intent"
	}

	return RiskLevelSafe, ""
}

// invokeExpressionRe detects Invoke-Expression and its alias iex.
var invokeExpressionRe = regexp.MustCompile(`(?i)(?:^|[\s;|&])(?:Invoke-Expression|iex)\b`)

func checkInvokeExpression(command string) (RiskLevel, string) {
	if invokeExpressionRe.MatchString(command) {
		return RiskLevelDangerous, "Command uses Invoke-Expression which can execute arbitrary code"
	}
	return RiskLevelSafe, ""
}

// downloadCradleRe detects patterns like Invoke-WebRequest ... | Invoke-Expression.
var downloadCradleRe = regexp.MustCompile(`(?i)(Invoke-WebRequest|iwr|Invoke-RestMethod|irm|curl|wget)[\s\S]{0,200}[\|;][\s\S]{0,100}(Invoke-Expression|iex)`)

func checkDownloadCradle(command string) (RiskLevel, string) {
	if downloadCradleRe.MatchString(command) {
		return RiskLevelDangerous, "Command downloads and executes remote code"
	}
	return RiskLevelSafe, ""
}

// nestedPSRe detects spawning a nested PowerShell process.
var nestedPSRe = regexp.MustCompile(`(?i)(?:^|[\s;|&])(?:pwsh|powershell|powershell\.exe)(?:\s+-[a-z])`)

func checkNestedPowerShell(command string) (RiskLevel, string) {
	if nestedPSRe.MatchString(command) {
		return RiskLevelWarning, "Command spawns a nested PowerShell process which may be dangerous"
	}
	return RiskLevelSafe, ""
}

// dangerousScriptBlockCmdlets lists cmdlets where script blocks can execute arbitrary code.
var dangerousScriptBlockCmdlets = map[string]bool{
	"invoke-command":        true,
	"start-job":             true,
	"start-threadjob":       true,
	"register-scheduledjob": true,
}

// scriptBlockRe detects script blocks ({ ... }).
var scriptBlockRe = regexp.MustCompile(`\{[^}]*\}`)

func checkScriptBlockInjection(command string) (RiskLevel, string) {
	if !scriptBlockRe.MatchString(command) {
		return RiskLevelSafe, ""
	}

	lower := strings.ToLower(command)
	for cmdlet := range dangerousScriptBlockCmdlets {
		if strings.Contains(lower, cmdlet) {
			pattern := regexp.MustCompile(`(?i)(?:^|[\s;|&])` + regexp.QuoteMeta(cmdlet) + `\b`)
			if pattern.MatchString(command) {
				return RiskLevelDangerous, fmt.Sprintf(
					"Command uses %s with a script block which may execute arbitrary code", cmdlet)
			}
		}
	}
	return RiskLevelSafe, ""
}

// addTypeRe detects Add-Type which compiles and loads .NET code.
var addTypeRe = regexp.MustCompile(`(?i)(?:^|[\s;|&])Add-Type\b`)

func checkAddType(command string) (RiskLevel, string) {
	if addTypeRe.MatchString(command) {
		return RiskLevelDangerous, "Command uses Add-Type which compiles and loads .NET code"
	}
	return RiskLevelSafe, ""
}

// =============================================================================
// Phase 2 validators (18 additional checks — ported from powershellSecurity.ts)
// =============================================================================

// ---------------------------------------------------------------------------
// 2. checkDynamicCommandName: & <expression> where command name is dynamic
// ---------------------------------------------------------------------------
// Detects invocation operator & with non-literal expressions. A command name
// that is not a simple string constant (e.g. & $function:Invoke-Expression,
// & ('iex','x')[0], & ('i'+'ex')) cannot be statically validated.
var dynamicCmdNameRe = regexp.MustCompile(`(?i)&[\s]*[\$\(]`)

func checkDynamicCommandName(command string) (RiskLevel, string) {
	if dynamicCmdNameRe.MatchString(command) {
		return RiskLevelWarning, "Command name is a dynamic expression which cannot be statically validated"
	}
	return RiskLevelSafe, ""
}

// ---------------------------------------------------------------------------
// 6. checkDownloadUtilities: standalone download tools (LOLBAS)
// ---------------------------------------------------------------------------
// Detects Start-BitsTransfer, certutil -urlcache, bitsadmin /transfer.
var downloadUtilRe = regexp.MustCompile(`(?i)(?:^|[\s;|&])(Start-BitsTransfer)\b`)

func checkDownloadUtilities(command string) (RiskLevel, string) {
	if downloadUtilRe.MatchString(command) {
		return RiskLevelDangerous, "Command downloads files via BITS transfer"
	}

	// certutil -urlcache or certutil /urlcache
	if regexp.MustCompile(`(?i)(?:^|[\s;|&])(certutil|certutil\.exe)\b`).MatchString(command) &&
		regexp.MustCompile(`(?i)[\s\-/]urlcache\b`).MatchString(command) {
		return RiskLevelDangerous, "Command uses certutil to download from a URL"
	}

	// bitsadmin /transfer
	if regexp.MustCompile(`(?i)(?:^|[\s;|&])(bitsadmin|bitsadmin\.exe)\b`).MatchString(command) &&
		regexp.MustCompile(`(?i)[\s\-/]transfer\b`).MatchString(command) {
		return RiskLevelDangerous, "Command downloads files via BITS transfer"
	}

	return RiskLevelSafe, ""
}

// ---------------------------------------------------------------------------
// 8. checkComObject: New-Object -ComObject (COM object instantiation)
// ---------------------------------------------------------------------------
// COM objects (WScript.Shell, Shell.Application, etc.) have their own
// execution capabilities — no IEX required.
var comObjectRe = regexp.MustCompile(`(?i)(?:^|[\s;|&])New-Object\b[\s\S]*\-[Cc]om(?:Object)?\b`)

func checkComObject(command string) (RiskLevel, string) {
	if comObjectRe.MatchString(command) {
		return RiskLevelWarning, "Command instantiates a COM object which may have execution capabilities"
	}
	return RiskLevelSafe, ""
}

// ---------------------------------------------------------------------------
// 9. checkDangerousFilePathExecution: cmdlet -FilePath <script>
// ---------------------------------------------------------------------------
// Dangerous cmdlets (Invoke-Command, Start-Job, etc.) invoked with -FilePath
// run an arbitrary script file. Without -FilePath, script blocks are visible
// to checkScriptBlockInjection.
var (
	dangerousFilepathCmdlets = []string{
		"Invoke-Command", "Start-Job", "Start-ThreadJob", "Register-ScheduledJob",
	}
	dangerousFilepathRe = regexp.MustCompile(`(?i)(?:^|[\s;|&])` +
		joinPattern(dangerousFilepathCmdlets) +
		`\b[\s\S]*[\s\-/](?:FilePath|LiteralPath|f|l)\b`)
)

// joinPattern joins cmdlet names with | for regex alternation.
func joinPattern(cmdlets []string) string {
	return "(?:" + strings.Join(cmdlets, "|") + ")"
}

func checkDangerousFilePathExecution(command string) (RiskLevel, string) {
	if dangerousFilepathRe.MatchString(command) {
		return RiskLevelDangerous, "Command executes an arbitrary script file via -FilePath"
	}
	return RiskLevelSafe, ""
}

// ---------------------------------------------------------------------------
// 10. checkForEachMemberName: ForEach-Object -MemberName
// ---------------------------------------------------------------------------
// Invokes a method by string name on every piped object — semantically
// equivalent to | % { $_.Method() } but without a script block in the AST.
var foreachMemberNameRe = regexp.MustCompile(`(?i)(?:^|[\s;|&])(?:ForEach-Object|foreach)\b[\s\S]*[\s\-/][Mm]emberName\b`)

func checkForEachMemberName(command string) (RiskLevel, string) {
	if foreachMemberNameRe.MatchString(command) {
		return RiskLevelWarning, "ForEach-Object -MemberName invokes methods by string name which cannot be validated"
	}
	return RiskLevelSafe, ""
}

// ---------------------------------------------------------------------------
// 11. checkStartProcess: Start-Process -Verb RunAs (privilege escalation)
// ---------------------------------------------------------------------------
// Two vectors: (1) -Verb RunAs — UAC elevation, (2) launching PowerShell
// executables — nested invocation bypassing checkEncodedCommand.
var (
	startProcessVerbRunAsRe = regexp.MustCompile(`(?i)(?:^|[\s;|&])Start-Process\b[\s\S]*[\s\-/][Vv]erb[\s:=]+[Rr]unAs\b`)
	startProcessPSLaunchRe  = regexp.MustCompile(`(?i)(?:^|[\s;|&])Start-Process\b[\s\S]*(?:pwsh|powershell|powershell\.exe)`)
)

func checkStartProcess(command string) (RiskLevel, string) {
	if startProcessVerbRunAsRe.MatchString(command) {
		return RiskLevelDangerous, "Command requests elevated privileges via Start-Process -Verb RunAs"
	}
	if startProcessPSLaunchRe.MatchString(command) {
		return RiskLevelWarning, "Start-Process launches a nested PowerShell process which cannot be validated"
	}
	return RiskLevelSafe, ""
}

// ---------------------------------------------------------------------------
// 13. checkSubExpressions: $() subexpressions
// ---------------------------------------------------------------------------
// Subexpressions can hide command execution inside $().
var subExprRe = regexp.MustCompile(`\$\(`)

func checkSubExpressions(command string) (RiskLevel, string) {
	if subExprRe.MatchString(command) {
		return RiskLevelWarning, "Command contains subexpressions $() which may hide command execution"
	}
	return RiskLevelSafe, ""
}

// ---------------------------------------------------------------------------
// 14. checkExpandableStrings: "$var" or "$(...)" inside double quotes
// ---------------------------------------------------------------------------
// Double-quoted strings with embedded expressions can hide variable
// interpolation or command execution.
var expandableStrRe = regexp.MustCompile(`"[^"]*\$[\w\(]`)

func checkExpandableStrings(command string) (RiskLevel, string) {
	if expandableStrRe.MatchString(command) {
		return RiskLevelWarning, "Command contains expandable strings with embedded expressions"
	}
	return RiskLevelSafe, ""
}

// ---------------------------------------------------------------------------
// 15. checkSplatting: @variable splatting
// ---------------------------------------------------------------------------
// Splatting (@variable) can obscure arguments at runtime.
var splattingRe = regexp.MustCompile(`@[\w]+`)

func checkSplatting(command string) (RiskLevel, string) {
	if splattingRe.MatchString(command) {
		return RiskLevelWarning, "Command uses splatting (@variable) which can obscure arguments"
	}
	return RiskLevelSafe, ""
}

// ---------------------------------------------------------------------------
// 16. checkStopParsing: --% stop-parsing token
// ---------------------------------------------------------------------------
// The --% token prevents further PowerShell parsing, which can hide
// subsequent arguments from security analysis.
var stopParsingRe = regexp.MustCompile(`\-\-%`)

func checkStopParsing(command string) (RiskLevel, string) {
	if stopParsingRe.MatchString(command) {
		return RiskLevelWarning, "Command uses stop-parsing token (--%) which prevents further parsing"
	}
	return RiskLevelSafe, ""
}

// ---------------------------------------------------------------------------
// 17. checkMemberInvocations: .NET method calls (:: static, .Method())
// ---------------------------------------------------------------------------
// .NET method invocations can access system APIs. Detects both static
// method calls (::) and instance method calls (.Method()).
var memberInvocationRe = regexp.MustCompile(`::[\w]+\(|\.\w+\(`)

func checkMemberInvocations(command string) (RiskLevel, string) {
	if memberInvocationRe.MatchString(command) {
		return RiskLevelWarning, "Command invokes .NET methods which can access system APIs"
	}
	return RiskLevelSafe, ""
}

// ---------------------------------------------------------------------------
// 18. checkTypeLiterals: [TypeName] outside CLM allowlist
// ---------------------------------------------------------------------------
// Detects .NET type literals like [Reflection.Assembly], [IO.Pipes],
// [Diagnostics.Process] — types outside Microsoft's ConstrainedLanguage
// allowlist that can access sensitive system APIs.
var typeLiteralRe = regexp.MustCompile(`\[[\w\.]+\]`)

// clmAllowedTypes is the subset of .NET types considered safe by
// ConstrainedLanguage Mode. Ported from TS clmTypes.ts (~100 entries).
// Namespace is stripped before lookup, so only the short name is needed.
var clmAllowedTypes = map[string]bool{
	"alias": true, "allowemptycollection": true, "allowemptystring": true,
	"allownull": true, "argumentcompleter": true, "argumentcompletions": true,
	"array": true, "bigint": true, "bool": true, "boolean": true,
	"byte": true, "char": true, "cimclass": true, "cimconverter": true,
	"ciminstance": true, "cimtype": true, "cmdletbinding": true,
	"cultureinfo": true, "datetime": true, "decimal": true, "double": true,
	"dsclocalconfigurationmanager": true, "dscproperty": true,
	"dscresource": true, "experimentaction": true, "experimental": true,
	"experimentalfeature": true, "float": true, "guid": true,
	"hashtable": true, "int": true, "int16": true, "int32": true,
	"int64": true, "ipaddress": true, "ipendpoint": true, "long": true,
	"mailaddress": true, "norunspaceaffinity": true, "nullstring": true,
	"objectsecurity": true, "ordered": true, "outputtype": true,
	"parameter": true, "physicaladdress": true, "pscredential": true,
	"pscustomobject": true, "psdefaultvalue": true, "pslistmodifier": true,
	"psobject": true, "psprimitivedictionary": true, "pstypenameattribute": true,
	"ref": true, "regex": true, "sbyte": true, "securestring": true,
	"semver": true, "short": true, "single": true, "string": true,
	"supportswildcards": true, "switch": true, "timespan": true,
	"uint": true, "uint16": true, "uint32": true, "uint64": true,
	"ulong": true, "uri": true, "ushort": true,
	"validatecount": true, "validatedrive": true, "validatelength": true,
	"validatenotnull": true, "validatenotnullorempty": true,
	"validatenotnullorwhitespace": true, "validatepattern": true,
	"validaterange": true, "validatescript": true, "validateset": true,
	"validatetrusteddata": true, "validateuserdrive": true,
	"version": true, "void": true, "wildcardpattern": true,
	"x500distinguishedname": true, "x509certificate": true, "xml": true,
	// Also store base type
	"object": true,
	// ModuleSpecification
	"modulespecification": true,
}

func checkTypeLiterals(command string) (RiskLevel, string) {
	matches := typeLiteralRe.FindAllString(command, -1)
	for _, m := range matches {
		typeName := strings.Trim(m, "[]")
		lower := strings.ToLower(typeName)
		// Strip namespace prefix for matching
		if idx := strings.LastIndex(lower, "."); idx >= 0 {
			lower = lower[idx+1:]
		}
		if !clmAllowedTypes[lower] {
			return RiskLevelDangerous,
				fmt.Sprintf("Command uses .NET type [%s] outside the ConstrainedLanguage allowlist", typeName)
		}
	}
	return RiskLevelSafe, ""
}

// ---------------------------------------------------------------------------
// 19. checkInvokeItem: Invoke-Item / ii (ShellExecute)
// ---------------------------------------------------------------------------
// Invoke-Item opens files with the default handler (ShellExecute). On
// executable files this runs arbitrary code.
var invokeItemRe = regexp.MustCompile(`(?i)(?:^|[\s;|&])(?:Invoke-Item|ii)\b`)

func checkInvokeItem(command string) (RiskLevel, string) {
	if invokeItemRe.MatchString(command) {
		return RiskLevelDangerous, "Invoke-Item opens files with the default handler which can execute arbitrary code"
	}
	return RiskLevelSafe, ""
}

// ---------------------------------------------------------------------------
// 20. checkScheduledTask: scheduled-task persistence primitives
// ---------------------------------------------------------------------------
// Register-ScheduledTask, New-ScheduledTask, etc. and schtasks /create.
var (
	scheduledTaskCmdletsRe = regexp.MustCompile(`(?i)(?:^|[\s;|&])(?:Register-ScheduledTask|New-ScheduledTask|New-ScheduledTaskAction|Set-ScheduledTask)\b`)
	schtasksCreateRe       = regexp.MustCompile(`(?i)(?:^|[\s;|&])(?:schtasks|schtasks\.exe)\b[\s\S]*[\s\-/](?:[Cc]reate|[Cc]hange)\b`)
)

func checkScheduledTask(command string) (RiskLevel, string) {
	if scheduledTaskCmdletsRe.MatchString(command) {
		return RiskLevelDangerous, "Command creates or modifies a scheduled task (persistence primitive)"
	}
	if schtasksCreateRe.MatchString(command) {
		return RiskLevelDangerous, "schtasks with create/change modifies scheduled tasks (persistence primitive)"
	}
	return RiskLevelSafe, ""
}

// ---------------------------------------------------------------------------
// 21. checkEnvVarManipulation: environment variable writes
// ---------------------------------------------------------------------------
// Detects cmdlets that write to env: scope (Set-Item env:, Remove-Item env:).
// Environment variable manipulation can affect future command resolution.
var (
	envWriteCmdletsRe = regexp.MustCompile(`(?i)(?:^|[\s;|&])(?:Set-Item|New-Item|Remove-Item|Clear-Item|Set-Content|Add-Content)\b`)
	envVarRefRe       = regexp.MustCompile(`(?:\$)?env:[\w]+`)
)

func checkEnvVarManipulation(command string) (RiskLevel, string) {
	if !envVarRefRe.MatchString(command) {
		return RiskLevelSafe, ""
	}
	if envWriteCmdletsRe.MatchString(command) {
		return RiskLevelWarning, "Command modifies environment variables"
	}
	return RiskLevelSafe, ""
}

// ---------------------------------------------------------------------------
// 22. checkModuleLoading: Import-Module / Install-Module / etc.
// ---------------------------------------------------------------------------
// Module-loading cmdlets execute a .psm1's top-level script body or
// download from repositories — same risk as Invoke-Expression.
var moduleLoadingRe = regexp.MustCompile(`(?i)(?:^|[\s;|&])(?:Import-Module|Install-Module|Save-Module|Update-Module|Install-Script|Save-Script)\b`)

func checkModuleLoading(command string) (RiskLevel, string) {
	if moduleLoadingRe.MatchString(command) {
		return RiskLevelDangerous,
			"Command loads, installs, or downloads a PowerShell module or script, which can execute arbitrary code"
	}
	return RiskLevelSafe, ""
}

// ---------------------------------------------------------------------------
// 23. checkRuntimeStateManipulation: Set-Alias / Set-Variable
// ---------------------------------------------------------------------------
// Set-Alias can hijack command resolution; Set-Variable can poison
// $PSDefaultParameterValues. Neither can be validated statically.
var runtimeStateRe = regexp.MustCompile(`(?i)(?:^|[\s;|&])(?:Set-Alias|New-Alias|Set-Variable|New-Variable)\b`)

func checkRuntimeStateManipulation(command string) (RiskLevel, string) {
	if runtimeStateRe.MatchString(command) {
		return RiskLevelWarning,
			"Command creates or modifies an alias or variable that can affect future command resolution"
	}
	return RiskLevelSafe, ""
}

// ---------------------------------------------------------------------------
// 24. checkWmiProcessSpawn: Invoke-WmiMethod / Invoke-CimMethod
// ---------------------------------------------------------------------------
// WMI/CIM methods can spawn arbitrary processes via Win32_Process Create,
// bypassing checkStartProcess entirely.
var wmiSpawnRe = regexp.MustCompile(`(?i)(?:^|[\s;|&])(?:Invoke-WmiMethod|Invoke-CimMethod)\b`)

func checkWmiProcessSpawn(command string) (RiskLevel, string) {
	if wmiSpawnRe.MatchString(command) {
		return RiskLevelDangerous,
			"Command can spawn arbitrary processes via WMI/CIM (Win32_Process Create)"
	}
	return RiskLevelSafe, ""
}
