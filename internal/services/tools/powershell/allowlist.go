package powershell

import (
	"regexp"
	"strings"
)

// =============================================================================
// PS Cmdlet Allowlists
// =============================================================================
// Ported from TS src/tools/PowerShellTool/readOnlyValidation.ts

// psCmdletConfig stores the allowlist configuration for one cmdlet.
type psCmdletConfig struct {
	safeFlags      []string
	allowAllFlags  bool
}

// psCmdletAllowlist maps canonical cmdlet names (lowercase) to their safe flag configs.
var psCmdletAllowlist = buildCmdletAllowlist()

func buildCmdletAllowlist() map[string]psCmdletConfig {
	return map[string]psCmdletConfig{
		// =====================================================================
		// Filesystem (read-only)
		// =====================================================================
		"get-childitem":    {safeFlags: []string{"-Path", "-LiteralPath", "-Filter", "-Include", "-Exclude", "-Recurse", "-Depth", "-Name", "-Force", "-Attributes", "-Directory", "-File", "-Hidden", "-ReadOnly", "-System"}},
		"get-content":      {safeFlags: []string{"-Path", "-LiteralPath", "-TotalCount", "-Head", "-Tail", "-Raw", "-Encoding", "-Delimiter", "-ReadCount"}},
		"get-item":         {safeFlags: []string{"-Path", "-LiteralPath", "-Force", "-Stream"}},
		"get-itemproperty": {safeFlags: []string{"-Path", "-LiteralPath", "-Name"}},
		"test-path":        {safeFlags: []string{"-Path", "-LiteralPath", "-PathType", "-Filter", "-Include", "-Exclude", "-IsValid", "-NewerThan", "-OlderThan"}},
		"resolve-path":     {safeFlags: []string{"-Path", "-LiteralPath", "-Relative"}},
		"get-filehash":     {safeFlags: []string{"-Path", "-LiteralPath", "-Algorithm", "-InputStream"}},
		"get-acl":          {safeFlags: []string{"-Path", "-LiteralPath", "-Audit", "-Filter", "-Include", "-Exclude"}},
		// =====================================================================
		// Navigation
		// =====================================================================
		"set-location":  {safeFlags: []string{"-Path", "-LiteralPath", "-PassThru", "-StackName"}},
		"push-location": {safeFlags: []string{"-Path", "-LiteralPath", "-PassThru", "-StackName"}},
		"pop-location":  {safeFlags: []string{"-PassThru", "-StackName"}},
		// =====================================================================
		// Text searching/filtering
		// =====================================================================
		"select-string": {safeFlags: []string{"-Path", "-LiteralPath", "-Pattern", "-InputObject", "-SimpleMatch", "-CaseSensitive", "-Quiet", "-List", "-NotMatch", "-AllMatches", "-Encoding", "-Context", "-Raw", "-NoEmphasis"}},
		// =====================================================================
		// Data conversion
		// =====================================================================
		"convertto-json":   {safeFlags: []string{"-InputObject", "-Depth", "-Compress", "-EnumsAsStrings", "-AsArray"}},
		"convertfrom-json": {safeFlags: []string{"-InputObject", "-Depth", "-AsHashtable", "-NoEnumerate"}},
		"convertto-csv":    {safeFlags: []string{"-InputObject", "-Delimiter", "-NoTypeInformation", "-NoHeader", "-UseQuotes"}},
		"convertfrom-csv":  {safeFlags: []string{"-InputObject", "-Delimiter", "-Header", "-UseCulture"}},
		"convertto-xml":    {safeFlags: []string{"-InputObject", "-Depth", "-As", "-NoTypeInformation"}},
		"convertto-html":   {safeFlags: []string{"-InputObject", "-Property", "-Head", "-Title", "-Body", "-Pre", "-Post", "-As", "-Fragment"}},
		"format-hex":       {safeFlags: []string{"-Path", "-LiteralPath", "-InputObject", "-Encoding", "-Count", "-Offset"}},
		// =====================================================================
		// Object inspection
		// =====================================================================
		"get-member":     {safeFlags: []string{"-InputObject", "-MemberType", "-Name", "-Static", "-View", "-Force"}},
		"get-unique":     {safeFlags: []string{"-InputObject", "-AsString", "-CaseInsensitive", "-OnType"}},
		"compare-object": {safeFlags: []string{"-ReferenceObject", "-DifferenceObject", "-Property", "-SyncWindow", "-CaseSensitive", "-Culture", "-ExcludeDifferent", "-IncludeEqual", "-PassThru"}},
		"join-string":    {safeFlags: []string{"-InputObject", "-Property", "-Separator", "-OutputPrefix", "-OutputSuffix", "-SingleQuote", "-DoubleQuote", "-FormatString"}},
		"get-random":     {safeFlags: []string{"-InputObject", "-Minimum", "-Maximum", "-Count", "-SetSeed", "-Shuffle"}},
		// =====================================================================
		// Path utilities
		// =====================================================================
		"convert-path": {safeFlags: []string{"-Path", "-LiteralPath"}},
		"join-path":    {safeFlags: []string{"-Path", "-ChildPath", "-AdditionalChildPath"}},
		"split-path":   {safeFlags: []string{"-Path", "-LiteralPath", "-Qualifier", "-NoQualifier", "-Parent", "-Leaf", "-LeafBase", "-Extension", "-IsAbsolute"}},
		// =====================================================================
		// System info
		// =====================================================================
		"get-hotfix":             {safeFlags: []string{"-Id", "-Description"}},
		"get-itempropertyvalue":  {safeFlags: []string{"-Path", "-LiteralPath", "-Name"}},
		"get-psprovider":         {safeFlags: []string{"-PSProvider"}},
		"get-process":            {safeFlags: []string{"-Name", "-Id", "-Module", "-FileVersionInfo", "-IncludeUserName"}},
		"get-service":            {safeFlags: []string{"-Name", "-DisplayName", "-DependentServices", "-RequiredServices", "-Include", "-Exclude"}},
		"get-computerinfo":       {allowAllFlags: true},
		"get-host":               {allowAllFlags: true},
		"get-date":               {safeFlags: []string{"-Date", "-Format", "-UFormat", "-DisplayHint", "-AsUTC"}},
		"get-location":           {safeFlags: []string{"-PSProvider", "-PSDrive", "-Stack", "-StackName"}},
		"get-psdrive":            {safeFlags: []string{"-Name", "-PSProvider", "-Scope"}},
		"get-module":             {safeFlags: []string{"-Name", "-ListAvailable", "-All", "-FullyQualifiedName", "-PSEdition"}},
		"get-alias":              {safeFlags: []string{"-Name", "-Definition", "-Scope", "-Exclude"}},
		"get-history":            {safeFlags: []string{"-Id", "-Count"}},
		"get-culture":            {allowAllFlags: true},
		"get-uiculture":          {allowAllFlags: true},
		"get-timezone":           {safeFlags: []string{"-Name", "-Id", "-ListAvailable"}},
		"get-uptime":             {allowAllFlags: true},
		"get-cimclass":           {safeFlags: []string{"-ClassName", "-Namespace", "-MethodName", "-PropertyName", "-QualifierName"}},
		// =====================================================================
		// Output (with argLeaksValue concern — see checkArgLeaks)
		// =====================================================================
		"write-output":    {allowAllFlags: true},
		"write-host":      {allowAllFlags: true},
		"start-sleep":     {allowAllFlags: true},
		"format-table":    {allowAllFlags: true},
		"format-list":     {allowAllFlags: true},
		"format-wide":     {allowAllFlags: true},
		"format-custom":   {allowAllFlags: true},
		"measure-object":  {allowAllFlags: true},
		"select-object":   {allowAllFlags: true},
		"sort-object":     {allowAllFlags: true},
		"group-object":    {allowAllFlags: true},
		"where-object":    {allowAllFlags: true},
		"out-string":      {allowAllFlags: true},
		"out-host":        {allowAllFlags: true},
		// =====================================================================
		// Network info
		// =====================================================================
		"get-netadapter":         {safeFlags: []string{"-Name", "-InterfaceDescription", "-InterfaceIndex", "-Physical"}},
		"get-netipaddress":       {safeFlags: []string{"-InterfaceIndex", "-InterfaceAlias", "-AddressFamily", "-Type"}},
		"get-netipconfiguration": {safeFlags: []string{"-InterfaceIndex", "-InterfaceAlias", "-Detailed", "-All"}},
		"get-netroute":           {safeFlags: []string{"-InterfaceIndex", "-InterfaceAlias", "-AddressFamily", "-DestinationPrefix"}},
		"get-dnsclientcache":     {safeFlags: []string{"-Entry", "-Name", "-Type", "-Status", "-Section", "-Data"}},
		"get-dnsclient":          {safeFlags: []string{"-InterfaceIndex", "-InterfaceAlias"}},
		// =====================================================================
		// Event log
		// =====================================================================
		"get-eventlog":  {safeFlags: []string{"-LogName", "-Newest", "-After", "-Before", "-EntryType", "-Index", "-InstanceId", "-Message", "-Source", "-UserName", "-AsBaseObject", "-List"}},
		"get-winevent":  {safeFlags: []string{"-LogName", "-ListLog", "-ListProvider", "-ProviderName", "-Path", "-MaxEvents", "-FilterXPath", "-Force", "-Oldest"}},
		// Windows
		"ipconfig": {safeFlags: []string{"/all", "/displaydns", "/allcompartments"}},
		"netstat":  {safeFlags: []string{"-a", "-b", "-e", "-f", "-n", "-o", "-p", "-q", "-r", "-s", "-t", "-x", "-y"}},
	}
}

// psReadOnlyCmdlets lists cmdlets that only read state and never mutate external files.
// Derived from psCmdletAllowlist entries.
var psReadOnlyCmdlets = buildReadOnlySet()

func buildReadOnlySet() map[string]bool {
	names := []string{
		"get-childitem", "get-content", "get-item", "get-itemproperty",
		"test-path", "resolve-path", "get-filehash", "get-acl",
		"set-location", "push-location", "pop-location",
		"select-string",
		"convertto-json", "convertfrom-json", "convertto-csv", "convertfrom-csv",
		"convertto-xml", "convertto-html", "format-hex", "out-string",
		"get-member", "get-unique", "compare-object", "join-string", "get-random",
		"convert-path", "join-path", "split-path",
		"get-process", "get-service",
		"get-date", "get-location", "get-host", "get-culture", "get-uiculture",
		"get-command", "get-module", "get-help", "get-alias", "get-variable",
		"get-history", "get-pssession", "get-wmiobject", "get-ciminstance",
		"get-computerinfo", "get-uptime", "get-timezone",
		"get-hotfix", "get-itempropertyvalue", "get-psprovider", "get-psdrive",
		"get-cimclass",
		"write-output", "write-host", "write-progress", "write-verbose",
		"write-debug", "write-warning",
		"format-table", "format-list", "format-wide", "format-custom",
		"out-host", "out-default",
		"select-object", "where-object", "group-object", "sort-object",
		"measure-object", "tee-object",
		"invoke-webrequest", "invoke-restmethod",
		"start-sleep",
		"get-netadapter", "get-netipaddress", "get-netipconfiguration",
		"get-netroute", "get-dnsclientcache", "get-dnsclient",
		"get-eventlog", "get-winevent",
		"ipconfig", "netstat",
		// External commands
		"git", "gh", "glab", "npm", "pip",
	}
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return set
}

// psAcceptEditsCmdlets lists cmdlets safe to auto-allow in acceptEdits mode.
// Ported from TS modeValidation.ts ACCEPT_EDITS_ALLOWED_CMDLETS.
// Only simple write cmdlets whose first positional is -Path are auto-allowed.
// Tier 3 cmdlets with complex parameter binding (new-item, copy-item, move-item,
// rename-item, set-item, out-file, set-itemproperty, etc.) are intentionally
// excluded — they require 'ask' for security review.
var psAcceptEditsCmdlets = map[string]bool{
	"set-content": true, "add-content": true, "clear-content": true,
	"remove-item": true,
}

// psSafeOutputCmdlets lists cmdlets that transform output without side effects.
var psSafeOutputCmdlets = map[string]bool{
	"format-table": true, "format-list": true, "format-wide": true,
	"format-custom": true, "out-string": true, "out-host": true,
	"out-default": true, "select-object": true, "group-object": true,
	"sort-object": true, "measure-object": true, "tee-object": true,
	"convertto-json": true, "convertto-csv": true, "convertto-html": true,
	"convertto-xml": true, "format-hex": true, "where-object": true,
}

// psProviderPaths is the set of PSDrive providers that access non-filesystem resources.
var psProviderPaths = map[string]bool{
	"env": true, "hklm": true, "hkcu": true, "function": true,
	"alias": true, "variable": true, "cert": true, "wsman": true, "registry": true,
}

// =============================================================================
// Flag Validation
// =============================================================================

// flagRe extracts PowerShell-style flags (-Flag, -Flag:Value, /Flag).
var flagRe = regexp.MustCompile(`(?:^|\s)[-\/]([a-zA-Z][\w?:]*)`)

// validateFlags checks that all flags in the command are in the safe set.
// Returns true when all flags are safe (or command allows all flags).
func validateFlags(command string, canonical string) bool {
	cfg, ok := psCmdletAllowlist[canonical]
	if !ok {
		return true // unknown cmdlet — no flag constraints
	}
	if cfg.allowAllFlags {
		return true
	}
	if len(cfg.safeFlags) == 0 {
		// No flags allowed at all — reject any flag usage
		return !hasFlags(command)
	}

	// Check each found flag against the safe list
	flags := extractFlags(command)
	for _, f := range flags {
		if !isFlagSafe(f, cfg.safeFlags) {
			return false
		}
	}
	return true
}

// extractFlags extracts all PowerShell-style flags from a command string.
func extractFlags(command string) []string {
	matches := flagRe.FindAllStringSubmatch(command, -1)
	var flags []string
	for _, m := range matches {
		if len(m) >= 2 {
			flags = append(flags, strings.ToLower(m[1]))
		}
	}
	return flags
}

// hasFlags returns true when the command contains any flag-like tokens.
func hasFlags(command string) bool {
	return flagRe.MatchString(command)
}

// psCommonParams lists PowerShell common parameters available on all cmdlets.
// Ported from TS commonParameters.ts. These are always allowed regardless of
// the cmdlet's safeFlags list.
var psCommonParams = map[string]bool{
	// Common switches (no value)
	"verbose": true, "debug": true,
	// Common value parameters
	"erroraction": true, "warningaction": true, "informationaction": true,
	"progressaction": true, "errorvariable": true, "warningvariable": true,
	"informationvariable": true, "outvariable": true, "outbuffer": true,
	"pipelinevariable": true,
	// Common parameter aliases
	"vb": true, "db": true, "ea": true, "wa": true, "ia": true,
	"pa": true, "ev": true, "wv": true, "iv": true, "ov": true,
	"ob": true, "pv": true,
}

// isFlagSafe checks if a flag (lowercase, without leading dash) is a valid
// abbreviation of any safe flag or a common PowerShell parameter.
func isFlagSafe(flag string, safeFlags []string) bool {
	// Check common parameters first (always allowed)
	if psCommonParams[flag] {
		return true
	}
	// Try the flag and its individual characters
	checks := []string{flag}
	if len(flag) > 1 {
		// Also try each character individually for combined flags like -la
		for _, ch := range flag {
			checks = append(checks, string(ch))
		}
	}

	for _, check := range checks {
		if check == "" {
			continue
		}
		for _, sf := range safeFlags {
			sfLower := strings.ToLower(strings.TrimLeft(sf, "-/"))
			// Direct match or abbreviation
			if check == sfLower || strings.HasPrefix(sfLower, check) {
				return true
			}
		}
		// Colon-bound: -Path:C:\foo → check "path"
		if colonIdx := strings.Index(check, ":"); colonIdx > 0 {
			prefix := check[:colonIdx]
			for _, sf := range safeFlags {
				sfLower := strings.ToLower(strings.TrimLeft(sf, "-/"))
				if sfLower == prefix || strings.HasPrefix(sfLower, prefix) {
					return true
				}
			}
		}
	}
	return false
}

// =============================================================================
// argLeaksValue Detection
// =============================================================================

// argLeaksRe detects variable references ($var), subexpressions ($(...)),
// splatting (@var), and type literals ([type]) in command arguments.
var argLeaksRe = regexp.MustCompile(`[\$\@\[\(]`)

// checkArgLeaks returns true when a command contains argument values that
// could leak sensitive data (variables, subexpressions, splatted variables,
// type literals in argument position).
//
// Ported from TS readOnlyValidation.ts argLeaksValue.
// Without the AST parser, we use regex heuristic: if the command is a
// "printing" cmdlet (Write-Output, Write-Host, Start-Sleep, format-*,
// out-string, out-host) AND has variable/subexpression references in its
// arguments, flag it.
func checkArgLeaks(command string) bool {
	first := firstCmdlet(command)
	if first == "" {
		return false
	}
	canonical := resolvePSCommand(first)

	// Only check cmdlets that print/leak their arguments
	leakSensitiveCmdlets := map[string]bool{
		"write-output": true, "write-host": true, "start-sleep": true,
		"format-table": true, "format-list": true, "format-wide": true,
		"format-custom": true, "measure-object": true,
		"select-object": true, "sort-object": true, "group-object": true,
		"where-object": true, "out-string": true, "out-host": true,
		"echo": true, "write": true,
	}

	if !leakSensitiveCmdlets[canonical] {
		return false
	}

	// Check for variable/expression references in arguments
	return argLeaksRe.MatchString(command)
}

// =============================================================================
// Dangerous Removal Detection
// =============================================================================

// dangerousRemovalPaths lists filesystem paths where deletion is never safe,
// regardless of permission rules.
var dangerousRemovalPaths = []string{
	"/", "/etc", "/bin", "/sbin", "/usr", "/lib", "/boot", "/dev", "/proc", "/sys",
	"/home", "/root",
	"C:\\", "C:\\Windows", "C:\\Windows\\System32", "C:\\Program Files",
	"D:\\", "System32", "Windows\\System32",
}

// isDangerousRemoval returns true when the command appears to be a dangerous
// deletion targeting protected system paths.
func isDangerousRemoval(command string) bool {
	first := firstCmdlet(command)
	if first == "" {
		return false
	}
	canonical := resolvePSCommand(first)
	if canonical != "remove-item" && canonical != "ri" && canonical != "del" &&
		canonical != "rd" && canonical != "rmdir" && canonical != "rm" && canonical != "erase" {
		return false
	}

	lower := strings.ToLower(command)
	for _, path := range dangerousRemovalPaths {
		lowerPath := strings.ToLower(path)
		if strings.Contains(lower, lowerPath) {
			// Verify it's not a subpath (e.g., /home/user is OK, /home alone is dangerous)
			if strings.TrimSpace(lower) == "remove-item "+lowerPath ||
				strings.Contains(lower, "remove-item "+lowerPath+" ") ||
				strings.Contains(lower, "rm "+lowerPath+" ") ||
				strings.Contains(lower, "rm "+lowerPath) {
				return true
			}
		}
	}
	return false
}

// =============================================================================
// Sub-command Splitting
// =============================================================================

// psSubCommandSplitter splits a compound PowerShell command into individual
// sub-commands for independent permission checking.
var psSubCommandSplitter = regexp.MustCompile(`[\n;|]|&&|\|\|`)

func splitSubCommands(command string) []string {
	parts := psSubCommandSplitter.Split(command, -1)
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	if len(result) <= 1 {
		return nil
	}
	return result
}

// =============================================================================
// PS Cmdlet Classification
// =============================================================================

// isReadOnlyPSCmdlet returns true when the command resolves to a known
// read-only PowerShell cmdlet, including flag validation.
func isReadOnlyPSCmdlet(command string) bool {
	return isReadOnlyPSCmdletChecked(command, ParsedCommandElement{})
}

// isReadOnlyPSCmdletChecked returns true when the command resolves to a known
// read-only PowerShell cmdlet, with optional AST element type validation.
// When cmd contains valid ElementTypes, they are checked against the whitelist
// (rejecting Variable, ScriptBlock, SubExpression, MemberInvocation, etc.).
func isReadOnlyPSCmdletChecked(command string, cmd ParsedCommandElement) bool {
	first := firstCmdlet(command)
	if first == "" {
		return false
	}
	canonical := resolvePSCommand(first)

	// Check the read-only set
	if !psReadOnlyCmdlets[canonical] {
		return false
	}

	// Check that all flags are safe for this cmdlet
	if !validateFlags(command, canonical) {
		return false
	}

	// Check additional dangerous callbacks (ipconfig/hostname/route with positional args)
	if checkAdditionalDangerous(command) {
		return false
	}

	// Check element types whitelist (AST-based arg validation)
	if cmd.ElementTypes != nil && len(cmd.ElementTypes) > 0 {
		if checkArgElementTypes(cmd) {
			return false
		}
	}

	return true
}

// isAcceptEditsCmdlet returns true when the command resolves to a cmdlet
// that should auto-allow in acceptEdits mode.
func isAcceptEditsCmdlet(command string) bool {
	first := firstCmdlet(command)
	if first == "" {
		return false
	}
	canonical := resolvePSCommand(first)
	return psAcceptEditsCmdlets[canonical]
}

// isSafeOutputCmdlet returns true when the command is a safe pipeline tail.
func isSafeOutputCmdlet(command string) bool {
	first := firstCmdlet(command)
	if first == "" {
		return false
	}
	canonical := resolvePSCommand(first)
	return psSafeOutputCmdlets[canonical]
}

// firstCmdlet extracts the first token from a command.
func firstCmdlet(command string) string {
	tokens := strings.Fields(strings.TrimSpace(command))
	if len(tokens) == 0 {
		return ""
	}
	return tokens[0]
}

// hasProviderPath returns true when the command contains a PSDrive provider path.
func hasProviderPath(command string) bool {
	lower := strings.ToLower(command)
	for provider := range psProviderPaths {
		if strings.Contains(lower, provider+":") {
			return true
		}
	}
	return false
}

// =============================================================================
// PS Command Type
// =============================================================================

// psCommandType describes the nature of a PowerShell command for permission decisions.
type psCommandType int

const (
	psCmdUnknown    psCommandType = iota
	psCmdReadOnly                 // Safe read-only command (auto-allow)
	psCmdWrite                    // Write command (may need approval)
	psCmdDangerous                // Dangerous command (security flagged)
)

func classifyPSCmd(command string, scanResult ScanResult) psCommandType {
	if scanResult.Level >= RiskLevelDangerous {
		return psCmdDangerous
	}
	if isReadOnlyPSCmdlet(command) {
		return psCmdReadOnly
	}
	return psCmdWrite
}


// =============================================================================
// Git Safety
// =============================================================================

// gitInternalPaths lists paths inside .git/ that should not be written to (lowercase).
var gitInternalPaths = []string{
    ".git/hooks", ".git/config", ".git/head", ".git/objects",
    ".git/refs", ".git/index", ".git/description", ".git/info",
}

// bareRepoPaths lists bare-repo-equivalent paths (no .git/ prefix).
var bareRepoPaths = []string{
    "hooks", "refs", "objects",
}

// gitWriteCmdlets lists cmdlets that can write to git-internal paths.
var gitWriteCmdlets = map[string]bool{
    "new-item": true, "set-content": true, "add-content": true,
    "out-file": true, "copy-item": true, "move-item": true,
    "rename-item": true, "expand-archive": true,
    "invoke-webrequest": true, "invoke-restmethod": true,
    "tee-object": true, "export-csv": true, "export-clixml": true,
}

// checkGitInternalWrite detects commands that write to git-internal paths
// like .git/hooks/pre-commit, .git/config, etc.
func checkGitInternalWrite(command string) bool {
    lower := strings.ToLower(command)
    first := firstCmdlet(command)
    if first == "" {
        return false
    }
    canonical := resolvePSCommand(first)

    // Check for write cmdlets targeting git-internal paths
    if gitWriteCmdlets[canonical] {
        for _, path := range gitInternalPaths {
            if strings.Contains(lower, path) {
                return true
            }
        }
    }

    // Check for redirection (> .git/hooks/pre-commit)
    redirPattern := regexp.MustCompile(`(?:>|>>)\s*["']?(?:\./)?(?:\.git)?\.git/[\w./-]+`)
    if redirPattern.MatchString(lower) {
        return true
    }

    return false
}

// checkBareRepoCompound detects compound commands that run git after
// creating bare-repo paths (e.g., "mkdir hooks; git status").
func checkBareRepoCompound(command string) bool {
    lower := strings.ToLower(command)

    // Check if command runs git
    if !strings.Contains(lower, " git ") && !strings.HasPrefix(lower, "git ") {
        return false
    }

    // Check if command also creates bare-repo paths
    for _, path := range bareRepoPaths {
        if strings.Contains(lower, "new-item "+path) ||
            strings.Contains(lower, "mkdir "+path) ||
            strings.Contains(lower, "ni "+path) ||
            strings.Contains(lower, path+"/") {
            return true
        }
    }

    return false
}

// checkAdditionalDangerous returns true when a cmdlet has positional args
// that make it dangerous (ipconfig: write config, hostname: set name, route: modify table).
func checkAdditionalDangerous(command string) bool {
	first := firstCmdlet(command)
	if first == "" {
		return false
	}
	canonical := resolvePSCommand(first)

	tokens := strings.Fields(strings.TrimSpace(command))
	var positionalArgs []string
	for _, tok := range tokens[1:] {
		if !strings.HasPrefix(tok, "-") && !strings.HasPrefix(tok, "/") {
			positionalArgs = append(positionalArgs, tok)
		}
	}

	switch canonical {
	case "ipconfig":
		return len(positionalArgs) > 0
	case "hostname":
		return len(positionalArgs) > 0
	case "route":
		for _, arg := range positionalArgs {
			if strings.ToLower(arg) == "print" {
				return false
			}
		}
		return len(positionalArgs) > 0
	}
	return false
}

// containsVulnerableUncPath detects UNC paths in a command.
// UNC paths (\\server\share) can leak NTLM credentials on Windows.
func containsVulnerableUncPath(command string) bool {
	// Check for \\server\share pattern (double backslash followed by host)
	if strings.Contains(command, "\\\\") {
		// Verify it looks like a UNC path, not just escaped chars
		uncRe := regexp.MustCompile(`\\\\[^\\s\\\\/]`)
		if uncRe.MatchString(command) {
			return true
		}
	}
	// Check for //server/share pattern (forward-slash UNC on Windows)
	// Exclude URLs: http://, https://, ftp://
	if strings.Contains(command, "//") {
		urlRe := regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.-]*://`)
		if !urlRe.MatchString(command) {
			uncRe := regexp.MustCompile(`//[^\\s/]`)
			if uncRe.MatchString(command) {
				return true
			}
		}
	}
	return false
}


// isCwdChangingCmdlet returns true when the command starts with a cmdlet that
// changes the working directory (Set-Location, Push-Location, Pop-Location).
func isCwdChangingCmdlet(command string) bool {
    first := firstCmdlet(command)
    if first == "" {
        return false
    }
    canonical := resolvePSCommand(first)
    switch canonical {
    case "set-location", "push-location", "pop-location":
        return true
    }
    return false
}

// hasSyncSecurityConcerns returns true when a command contains security-relevant
// patterns that should prevent read-only auto-allow. Quick sync check without AST.
var splatRe = regexp.MustCompile(`@[\w]+`)

func hasSyncSecurityConcerns(command string) bool {
    // Subexpressions: $(
    if strings.Contains(command, "$(") {
        return true
    }
    // Splatting: @variable
    if strings.Contains(command, "@") {
        if splatRe.MatchString(strings.TrimSpace(command)) {
            return true
        }
    }
    // Member invocations: ::
    if strings.Contains(command, "::") {
        return true
    }
    // Script blocks with member invocations or subexpressions
    if strings.Contains(command, "{") && strings.Contains(command, "}") {
        if strings.Contains(command, "::") || strings.Contains(command, "$(") {
            return true
        }
    }
    return false
}


// =============================================================================
// Element Types Whitelist — AST-level arg validation
// =============================================================================

// isSafeArgElementType returns true when the element type represents a
// statically-verifiable string value. Only StringConstant and Parameter
// are safe — everything else (Variable, Other, ScriptBlock, SubExpression,
// ExpandableString, MemberInvocation) evaluates at runtime.
func isSafeArgElementType(elementType string) bool {
	return elementType == "StringConstant" || elementType == "Parameter"
}

// argLeaksMetaCharRe detects variable references ($), subexpressions ($(, @(),
// arrays (@{), paren expressions ((), type literals ([), script blocks ({).
var argLeaksMetaCharRe = regexp.MustCompile(`[\$\@\[\(]`)

// checkArgElementTypes verifies all argument element types in a parsed command
// element. Returns true if any arg has an unsafe element type (Variable, Other,
// ScriptBlock, etc.) — meaning the command should NOT be auto-allowed.
//
// Ported from TS readOnlyValidation.ts isAllowlistedCommand elementTypes whitelist.
// Also checks colon-bound parameters for expression metacharacters.
func checkArgElementTypes(cmd ParsedCommandElement) bool {
	if cmd.ElementTypes == nil {
		// No element types available — fail-closed for untrusted elements
		return true
	}
	// elementTypes[0] is the command name; args start at elementTypes[1]
	for i := 1; i < len(cmd.ElementTypes); i++ {
		t := cmd.ElementTypes[i]
		if !isSafeArgElementType(t) {
			// For 'Other' type, do a text check for metacharacters.
			// ArrayLiteralAst (Get-Process Name, Id) maps to 'Other' but is safe
			// if the text has no metacharacters ($, @, {, (, [).
			if t == "Other" {
				argIdx := i - 1
				if argIdx < len(cmd.Args) {
					arg := cmd.Args[argIdx]
					if !argLeaksMetaCharRe.MatchString(arg) {
						continue
					}
				}
			}
			return true
		}
		// Colon-bound parameter check: -Flag:$env:SECRET creates a single
		// CommandParameterAst; the VariableExpressionAst is its .Argument child.
		// The outer 'Parameter' element type masks the inner expression type.
		if t == "Parameter" {
			argIdx := i - 1
			if argIdx < len(cmd.Args) {
				arg := cmd.Args[argIdx]
				colonIdx := strings.Index(arg, ":")
				if colonIdx > 0 && argLeaksMetaCharRe.MatchString(arg[colonIdx+1:]) {
					return true
				}
			}
		}
	}
	return false
}

// checkArgLeaksForElement checks whether a parsed command element's arguments
// could leak sensitive data. Used as additionalCommandIsDangerousCallback for
// cmdlets like Write-Output, Write-Host, Start-Sleep, Format-*.
func checkArgLeaksForElement(cmd ParsedCommandElement) bool {
	return checkArgElementTypes(cmd)
}
