package bash

import (
	"regexp"
	"slices"
	"strings"
)

// sedCommandIsAllowed reports whether a sed command passes security validation.
// When allowFileWrites is true, the -i / --in-place flag is permitted for
// substitution commands.
func sedCommandIsAllowed(command string, allowFileWrites bool) bool {
	trimmed := strings.TrimSpace(command)
	if !strings.HasPrefix(trimmed, "sed") {
		return true // Not a sed command, not our concern.
	}
	// Ensure it's actually "sed " and not something like "sedfoo".
	rest := trimmed[len("sed"):]
	if len(rest) == 0 {
		return false // Bare "sed" has no valid expression.
	}
	if rest[0] != ' ' && rest[0] != '\t' {
		return true // Not a valid sed invocation (e.g., "sedfoo").
	}

	tokens, ok := tokenizeShellCommand(strings.TrimSpace(rest))
	if !ok {
		return false // Unparseable commands are not allowed.
	}

	// Build allowed flag set.
	allowed := map[string]bool{
		"-E":               true,
		"-r":               true,
		"--regexp-extended": true,
		"--posix":          true,
		"-n":               true,
		"--quiet":          true,
		"--silent":         true,
		"-z":               true,
		"--zero-terminated": true,
	}
	if allowFileWrites {
		allowed["-i"] = true
		allowed["--in-place"] = true
	}

	// Separate flags from positional arguments.
	var flags []string
	var args []string
	for _, t := range tokens {
		if strings.HasPrefix(t, "-") && t != "--" {
			flags = append(flags, t)
		} else {
			args = append(args, t)
		}
	}

	// Validate all flags.
	for _, flag := range flags {
		if !isFlagAllowed(flag, allowed) {
			return false
		}
	}

	// Extract expressions.
	expressions, err := extractSedExpressions(command)
	if err != nil {
		return false
	}
	if len(expressions) == 0 {
		return false
	}

	// Check each expression for dangerous operations.
	if slices.ContainsFunc(expressions, containsDangerousOperations) {
		return false
	}

	// Determine pattern type.
	var isSubst bool
	for _, expr := range expressions {
		trimmedExpr := strings.TrimSpace(expr)
		if strings.HasPrefix(trimmedExpr, "s/") {
			isSubst = true
		} else if !isPrintCommand(trimmedExpr) {
			return false // Unknown expression type.
		}
	}

	// For in-place editing mode, only substitution commands are allowed.
	if allowFileWrites && !isSubst {
		return false
	}

	// Substitution expressions must not contain semicolons (command separators).
	if isSubst {
		for _, expr := range expressions {
			if strings.Contains(expr, ";") {
				return false
			}
		}
	}

	return true
}

// isFlagAllowed validates a single flag (including combined short flags) against
// the allowlist.
func isFlagAllowed(flag string, allowed map[string]bool) bool {
	if strings.HasPrefix(flag, "--") {
		return allowed[flag]
	}
	// Combined short flags like -nE.
	if len(flag) > 2 {
		for i := 1; i < len(flag); i++ {
			if !allowed["-"+string(flag[i])] {
				return false
			}
		}
		return true
	}
	return allowed[flag]
}

// isPrintCommand checks whether cmd is a valid sed print command.
// Allowed forms: p, Np, N,Mp (where N and M are digits).
func isPrintCommand(cmd string) bool {
	if cmd == "" {
		return false
	}
	return regexp.MustCompile(`^(?:\d+|\d+,\d+)?p$`).MatchString(cmd)
}

// extractSedExpressions extracts the sed script expressions from a full sed
// command string. It handles -e / --expression flags and bare positional
// expressions.
func extractSedExpressions(command string) ([]string, error) {
	trimmed := strings.TrimSpace(command)
	if !strings.HasPrefix(trimmed, "sed") {
		return nil, nil
	}
	rest := trimmed[len("sed"):]
	if len(rest) == 0 || (rest[0] != ' ' && rest[0] != '\t') {
		return nil, nil
	}

	withoutSed := strings.TrimSpace(rest)

	// Reject dangerous flag combinations like -ew, -eW, -ee, -we.
	if matched, _ := regexp.MatchString(`-e[wWe]`, withoutSed); matched {
		return nil, nil
	}
	if matched, _ := regexp.MatchString(`-w[eE]`, withoutSed); matched {
		return nil, nil
	}

	tokens, ok := tokenizeShellCommand(withoutSed)
	if !ok {
		return nil, nil
	}

	var expressions []string
	var foundEFlag, foundExpression bool

	for i := 0; i < len(tokens); i++ {
		arg := tokens[i]

		if (arg == "-e" || arg == "--expression") && i+1 < len(tokens) {
			foundEFlag = true
			expressions = append(expressions, tokens[i+1])
			i++
			continue
		}

		if strings.HasPrefix(arg, "--expression=") {
			foundEFlag = true
			expressions = append(expressions, strings.TrimPrefix(arg, "--expression="))
			continue
		}

		if strings.HasPrefix(arg, "-e=") {
			foundEFlag = true
			expressions = append(expressions, strings.TrimPrefix(arg, "-e="))
			continue
		}

		if strings.HasPrefix(arg, "-") {
			continue
		}

		if !foundEFlag && !foundExpression {
			expressions = append(expressions, arg)
			foundExpression = true
			continue
		}

		// Remaining non-flag arguments are filenames — stop collecting expressions.
		break
	}

	return expressions, nil
}

// sedHasFileArgs reports whether a sed command includes file arguments.
func sedHasFileArgs(command string) bool {
	trimmed := strings.TrimSpace(command)
	if !strings.HasPrefix(trimmed, "sed") {
		return false
	}
	rest := trimmed[len("sed"):]
	if len(rest) == 0 || (rest[0] != ' ' && rest[0] != '\t') {
		return false
	}

	tokens, ok := tokenizeShellCommand(strings.TrimSpace(rest))
	if !ok {
		return true // Assume dangerous if parsing fails.
	}

	argCount := 0
	hasEFlag := false

	for i := 0; i < len(tokens); i++ {
		arg := tokens[i]

		if (arg == "-e" || arg == "--expression") && i+1 < len(tokens) {
			hasEFlag = true
			i++
			continue
		}
		if strings.HasPrefix(arg, "--expression=") || strings.HasPrefix(arg, "-e=") {
			hasEFlag = true
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}

		argCount++

		if hasEFlag {
			return true
		}
		if argCount > 1 {
			return true
		}
	}

	return false
}

// containsDangerousOperations checks whether a sed expression contains
// operations that could write files or execute commands.
func containsDangerousOperations(expression string) bool {
	cmd := strings.TrimSpace(expression)
	if cmd == "" {
		return false
	}

	// Reject non-ASCII characters (Unicode homoglyphs).
	if matched, _ := regexp.MatchString(`[^\x01-\x7F]`, cmd); matched {
		return true
	}

	// Reject curly braces and newlines.
	if strings.ContainsAny(cmd, "{}\n") {
		return true
	}

	// Reject comments, but allow s# delimiter.
	if idx := strings.Index(cmd, "#"); idx >= 0 && !(idx > 0 && cmd[idx-1] == 's') {
		return true
	}

	// Reject negation operator.
	if matched, _ := regexp.MatchString(`^!|[/\d$]!`, cmd); matched {
		return true
	}

	// Reject GNU step address format.
	if matched, _ := regexp.MatchString(`\d\s*~\s*\d|,\s*~\s*\d|\$\s*~\s*\d`, cmd); matched {
		return true
	}

	// Reject comma at start.
	if strings.HasPrefix(cmd, ",") {
		return true
	}

	// Reject comma followed by +/-.
	if matched, _ := regexp.MatchString(`,\s*[+-]`, cmd); matched {
		return true
	}

	// Reject backslash tricks.
	if matched, _ := regexp.MatchString(`s\\|\\[|#%@]`, cmd); matched {
		return true
	}

	// Reject escaped slashes followed by w/W.
	if matched, _ := regexp.MatchString(`\\/.*[wW]`, cmd); matched {
		return true
	}

	// Reject malformed patterns: /pattern w file, /pattern e cmd.
	if matched, _ := regexp.MatchString(`\/[^/]*\s+[wWeE]`, cmd); matched {
		return true
	}

	// Reject malformed substitution commands that don't follow the normal pattern.
	if strings.HasPrefix(cmd, "s/") && !regexp.MustCompile(`^s\/[^/]*\/[^/]*\/[^/]*$`).MatchString(cmd) {
		return true
	}

	// Reject s commands that end with dangerous chars and aren't properly formed.
	if matched, _ := regexp.MatchString(`^s.`, cmd); matched {
		if matched2, _ := regexp.MatchString(`[wWeE]$`, cmd); matched2 {
			flags, ok := parseSedSubstitutionFlags(cmd)
			if !ok || strings.ContainsAny(flags, "wWeE") {
				return true
			}
		}
	}

	// Reject dangerous write commands.
	writePatterns := []string{
		`^[wW]\s*\S+`,
		`^\d+\s*[wW]\s*\S+`,
		`^\$\s*[wW]\s*\S+`,
		`^\/[^/]*\/[IMim]*\s*[wW]\s*\S+`,
		`^\d+,\d+\s*[wW]\s*\S+`,
		`^\d+,\$\s*[wW]\s*\S+`,
		`^\/[^/]*\/[IMim]*,\/[^/]*\/[IMim]*\s*[wW]\s*\S+`,
	}
	for _, pattern := range writePatterns {
		if matched, _ := regexp.MatchString(pattern, cmd); matched {
			return true
		}
	}

	// Reject dangerous execute commands.
	execPatterns := []string{
		`^e`,
		`^\d+\s*e`,
		`^\$\s*e`,
		`^\/[^/]*\/[IMim]*\s*e`,
		`^\d+,\d+\s*e`,
		`^\d+,\$\s*e`,
		`^\/[^/]*\/[IMim]*,\/[^/]*\/[IMim]*\s*e`,
	}
	for _, pattern := range execPatterns {
		if matched, _ := regexp.MatchString(pattern, cmd); matched {
			return true
		}
	}

	// Check substitution flags for w/W/e/E.
	if strings.HasPrefix(cmd, "s") {
		flags, ok := parseSedSubstitutionFlags(cmd)
		if ok && strings.ContainsAny(flags, "wWeE") {
			return true
		}
	}

	// Check y command with dangerous chars.
	if strings.HasPrefix(cmd, "y") {
		if strings.ContainsAny(cmd, "wWeE") {
			return true
		}
	}

	return false
}

// parseSedSubstitutionFlags extracts the flags portion from a sed substitution
// command s<delim>pattern<delim>replacement<delim>flags. It returns the flags
// string and true if the command has a valid substitution structure.
// Go's regexp package does not support backreferences, so this is done manually.
func parseSedSubstitutionFlags(cmd string) (string, bool) {
	if !strings.HasPrefix(cmd, "s") || len(cmd) < 2 {
		return "", false
	}
	delim := cmd[1]
	if delim == '\\' || delim == '\n' {
		return "", false
	}

	delimCount := 0
	lastDelimPos := -1
	escaped := false
	for i := 2; i < len(cmd); i++ {
		if escaped {
			escaped = false
			continue
		}
		if cmd[i] == '\\' {
			escaped = true
			continue
		}
		if cmd[i] == delim {
			delimCount++
			lastDelimPos = i
			if delimCount == 3 {
				return cmd[i+1:], true
			}
		}
	}

	if delimCount < 2 {
		return "", false
	}
	return cmd[lastDelimPos+1:], true
}
