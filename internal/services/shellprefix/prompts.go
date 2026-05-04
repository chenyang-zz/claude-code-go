// Package shellprefix extracts shell command prefixes via the Haiku helper.
//
// All prompt literals in this file are taken verbatim from
// src/utils/shell/prefix.ts to preserve model behaviour.
package shellprefix

import (
	"strings"
)

// systemPrompt is the system message used for command prefix extraction.
// Mirrors the TS default path (policy spec placed in user prompt, not
// system prompt). The GrowthBook-controlled alternative path is not
// ported.
const systemPrompt = `Your task is to process shell tool commands that an AI coding agent wants to run.

This policy spec defines how to determine the prefix of a shell tool command:`

// querySource is the identifier passed to the haiku layer for log
// correlation. Mirrors the TS concept (caller-supplied in TS, hardcoded
// here since we only serve the shell tool).
const querySource = "shell_prefix"

// dangerousShellPrefixes is the complete set of shell executables that
// must never be accepted as bare prefixes. Allowing e.g. "bash" would
// let any command through, defeating the permission system. Includes
// Unix shells and Windows equivalents.
//
// Verbatim copy of DANGEROUS_SHELL_PREFIXES from prefix.ts.
var dangerousShellPrefixes = newDangerousSet()

func newDangerousSet() dangerousSet {
	return dangerousSet{
		"sh":            {},
		"bash":          {},
		"zsh":           {},
		"fish":          {},
		"csh":           {},
		"tcsh":          {},
		"ksh":           {},
		"dash":          {},
		"cmd":           {},
		"cmd.exe":       {},
		"powershell":    {},
		"powershell.exe": {},
		"pwsh":          {},
		"pwsh.exe":      {},
		"bash.exe":      {},
	}
}

// dangerousSet is a lookup set for shell executables that must never be
// accepted as bare command prefixes.
type dangerousSet map[string]struct{}

func (ds dangerousSet) has(prefix string) bool {
	_, ok := ds[strings.ToLower(prefix)]
	return ok
}
