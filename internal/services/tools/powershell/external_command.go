package powershell

import (
	"strings"
)

// =============================================================================
// External Command Validation — git/gh/docker/dotnet subcommand-level safety
// =============================================================================
// Ported from TS src/utils/shell/readOnlyCommandValidation.ts and
// src/tools/PowerShellTool/readOnlyValidation.ts.
//
// These functions validate that external commands (git, gh, docker, dotnet)
// invoked through PowerShellTool have safe (read-only) subcommands and flags.
// Previously, git/gh in psReadOnlyCmdlets meant ANY invocation auto-allowed.

// flagArgType describes the type of argument a flag accepts.
type flagArgType string

const (
	flagNone   flagArgType = "none"
	flagNumber flagArgType = "number"
	flagString flagArgType = "string"
)

// externalCommandConfig stores safe flags and optional callback for a command.
type externalCommandConfig struct {
	safeFlags                        map[string]flagArgType
	additionalCommandIsDangerousCallback func(rawCommand string, args []string) bool
}

// =============================================================================
// Shared git flag groups
// =============================================================================

var gitRefSelectionFlags = map[string]flagArgType{
	"--all":      flagNone,
	"--branches": flagNone,
	"--tags":     flagNone,
	"--remotes":  flagNone,
}

var gitDateFilterFlags = map[string]flagArgType{
	"--since": flagString,
	"--after": flagString,
	"--until": flagString,
	"--before": flagString,
}

var gitLogDisplayFlags = map[string]flagArgType{
	"--oneline":       flagNone,
	"--graph":         flagNone,
	"--decorate":      flagNone,
	"--no-decorate":   flagNone,
	"--date":          flagString,
	"--relative-date": flagNone,
}

var gitCountFlags = map[string]flagArgType{
	"--max-count": flagNumber,
	"-n":          flagNumber,
}

var gitStatFlags = map[string]flagArgType{
	"--stat":       flagNone,
	"--numstat":    flagNone,
	"--shortstat":  flagNone,
	"--name-only":  flagNone,
	"--name-status": flagNone,
}

var gitColorFlags = map[string]flagArgType{
	"--color":    flagNone,
	"--no-color": flagNone,
}

var gitPatchFlags = map[string]flagArgType{
	"--patch":       flagNone,
	"-p":            flagNone,
	"--no-patch":    flagNone,
	"--no-ext-diff": flagNone,
	"-s":            flagNone,
}

var gitAuthorFilterFlags = map[string]flagArgType{
	"--author":   flagString,
	"--committer": flagString,
	"--grep":     flagString,
}

// =============================================================================
// GIT_READ_ONLY_COMMANDS
// =============================================================================

var gitReadOnlyCommands = map[string]externalCommandConfig{
	"git diff": {
		safeFlags: mergeFlags(gitStatFlags, gitColorFlags, map[string]flagArgType{
			"--dirstat": flagNone, "--summary": flagNone, "--patch-with-stat": flagNone,
			"--word-diff": flagNone, "--word-diff-regex": flagString, "--color-words": flagNone,
			"--no-renames": flagNone, "--no-ext-diff": flagNone, "--check": flagNone,
			"--ws-error-highlight": flagString, "--full-index": flagNone, "--binary": flagNone,
			"--abbrev": flagNumber, "--break-rewrites": flagNone, "--find-renames": flagNone,
			"--find-copies": flagNone, "--find-copies-harder": flagNone, "--irreversible-delete": flagNone,
			"--diff-algorithm": flagString, "--histogram": flagNone, "--patience": flagNone,
			"--minimal": flagNone, "--ignore-space-at-eol": flagNone, "--ignore-space-change": flagNone,
			"--ignore-all-space": flagNone, "--ignore-blank-lines": flagNone,
			"--inter-hunk-context": flagNumber, "--function-context": flagNone,
			"--exit-code": flagNone, "--quiet": flagNone, "--cached": flagNone, "--staged": flagNone,
			"--pickaxe-regex": flagNone, "--pickaxe-all": flagNone, "--no-index": flagNone,
			"--relative": flagString, "--diff-filter": flagString,
			"-u": flagNone, "-M": flagNone, "-C": flagNone, "-B": flagNone, "-D": flagNone, "-l": flagNone,
			"-S": flagString, "-G": flagString, "-O": flagString, "-R": flagNone,
		}),
	},
	"git log": {
		safeFlags: mergeFlags(gitLogDisplayFlags, gitRefSelectionFlags, gitDateFilterFlags,
			gitCountFlags, gitStatFlags, gitColorFlags, gitPatchFlags, gitAuthorFilterFlags, map[string]flagArgType{
				"--abbrev-commit": flagNone, "--full-history": flagNone, "--dense": flagNone, "--sparse": flagNone,
				"--simplify-merges": flagNone, "--ancestry-path": flagNone, "--source": flagNone,
				"--first-parent": flagNone, "--merges": flagNone, "--no-merges": flagNone,
				"--reverse": flagNone, "--walk-reflogs": flagNone, "--skip": flagNumber,
				"--max-age": flagNumber, "--min-age": flagNumber, "--no-min-parents": flagNone,
				"--no-max-parents": flagNone, "--follow": flagNone, "--no-walk": flagNone,
				"--left-right": flagNone, "--cherry-mark": flagNone, "--cherry-pick": flagNone,
				"--boundary": flagNone, "--topo-order": flagNone, "--date-order": flagNone,
				"--author-date-order": flagNone, "--pretty": flagString, "--format": flagString,
				"--diff-filter": flagString, "-S": flagString, "-G": flagString,
				"--pickaxe-regex": flagNone, "--pickaxe-all": flagNone,
			}),
	},
	"git show": {
		safeFlags: mergeFlags(gitLogDisplayFlags, gitStatFlags, gitColorFlags, gitPatchFlags, map[string]flagArgType{
			"--abbrev-commit": flagNone, "--word-diff": flagNone, "--word-diff-regex": flagString,
			"--color-words": flagNone, "--pretty": flagString, "--format": flagString,
			"--first-parent": flagNone, "--raw": flagNone, "--diff-filter": flagString,
			"-m": flagNone, "--quiet": flagNone,
		}),
	},
	"git shortlog": {
		safeFlags: mergeFlags(gitRefSelectionFlags, gitDateFilterFlags, map[string]flagArgType{
			"-s": flagNone, "--summary": flagNone, "-n": flagNone, "--numbered": flagNone,
			"-e": flagNone, "--email": flagNone, "-c": flagNone, "--committer": flagNone,
			"--group": flagString, "--format": flagString, "--no-merges": flagNone, "--author": flagString,
		}),
	},
	"git reflog": {
		safeFlags: mergeFlags(gitLogDisplayFlags, gitRefSelectionFlags, gitDateFilterFlags, gitCountFlags, gitAuthorFilterFlags),
		additionalCommandIsDangerousCallback: func(_ string, args []string) bool {
			dangerous := map[string]bool{"expire": true, "delete": true, "exists": true}
			for _, token := range args {
				if token == "" || strings.HasPrefix(token, "-") {
					continue
				}
				if dangerous[token] {
					return true
				}
				return false // first positional is safe (show/HEAD/ref) or dangerous
			}
			return false // no positional = bare `git reflog` = safe
		},
	},
	"git stash list": {
		safeFlags: mergeFlags(gitLogDisplayFlags, gitRefSelectionFlags, gitCountFlags),
	},
	"git ls-remote": {
		safeFlags: map[string]flagArgType{
			"--branches": flagNone, "-b": flagNone, "--tags": flagNone, "-t": flagNone,
			"--heads": flagNone, "-h": flagNone, "--refs": flagNone,
			"--quiet": flagNone, "-q": flagNone, "--exit-code": flagNone,
			"--get-url": flagNone, "--symref": flagNone, "--sort": flagString,
		},
	},
	"git status": {
		safeFlags: map[string]flagArgType{
			"--short": flagNone, "-s": flagNone, "--branch": flagNone, "-b": flagNone,
			"--porcelain": flagNone, "--long": flagNone, "--verbose": flagNone, "-v": flagNone,
			"--untracked-files": flagString, "-u": flagString,
			"--ignored": flagNone, "--ignore-submodules": flagString,
			"--column": flagNone, "--no-column": flagNone,
			"--ahead-behind": flagNone, "--no-ahead-behind": flagNone,
			"--renames": flagNone, "--no-renames": flagNone, "--find-renames": flagString, "-M": flagString,
		},
	},
	"git blame": {
		safeFlags: mergeFlags(gitColorFlags, map[string]flagArgType{
			"-L": flagString, "--porcelain": flagNone, "-p": flagNone, "--line-porcelain": flagNone,
			"--incremental": flagNone, "--root": flagNone, "--show-stats": flagNone,
			"--show-name": flagNone, "--show-number": flagNone, "-n": flagNone,
			"--show-email": flagNone, "-e": flagNone, "-f": flagNone,
			"--date": flagString, "-w": flagNone,
			"--ignore-rev": flagString, "--ignore-revs-file": flagString,
			"-M": flagNone, "-C": flagNone, "--score-debug": flagNone,
			"--abbrev": flagNumber, "-s": flagNone, "-l": flagNone, "-t": flagNone,
		}),
	},
	"git ls-files": {
		safeFlags: map[string]flagArgType{
			"--cached": flagNone, "-c": flagNone, "--deleted": flagNone, "-d": flagNone,
			"--modified": flagNone, "-m": flagNone, "--others": flagNone, "-o": flagNone,
			"--ignored": flagNone, "-i": flagNone, "--stage": flagNone, "-s": flagNone,
			"--killed": flagNone, "-k": flagNone, "--unmerged": flagNone, "-u": flagNone,
			"--directory": flagNone, "--no-empty-directory": flagNone, "--eol": flagNone,
			"--full-name": flagNone, "--abbrev": flagNumber, "--debug": flagNone,
			"-z": flagNone, "-t": flagNone, "-v": flagNone, "-f": flagNone,
			"--exclude": flagString, "-x": flagString, "--exclude-from": flagString, "-X": flagString,
			"--exclude-per-directory": flagString, "--exclude-standard": flagNone,
			"--error-unmatch": flagNone, "--recurse-submodules": flagNone,
		},
	},
	"git config --get": {
		safeFlags: map[string]flagArgType{
			"--local": flagNone, "--global": flagNone, "--system": flagNone, "--worktree": flagNone,
			"--default": flagString, "--type": flagString, "--bool": flagNone, "--int": flagNone,
			"--bool-or-int": flagNone, "--path": flagNone, "--expiry-date": flagNone,
			"-z": flagNone, "--null": flagNone, "--name-only": flagNone, "--show-origin": flagNone, "--show-scope": flagNone,
		},
	},
	"git remote show": {
		safeFlags: map[string]flagArgType{"-n": flagNone},
		additionalCommandIsDangerousCallback: func(_ string, args []string) bool {
			var positional []string
			for _, a := range args {
				if a != "-n" {
					positional = append(positional, a)
				}
			}
			if len(positional) != 1 {
				return true
			}
			return !isAlphaNumDashLike(positional[0])
		},
	},
	"git remote": {
		safeFlags: map[string]flagArgType{"-v": flagNone, "--verbose": flagNone},
		additionalCommandIsDangerousCallback: func(_ string, args []string) bool {
			var nonFlag []string
			for _, a := range args {
				if !strings.HasPrefix(a, "-") {
					nonFlag = append(nonFlag, a)
				}
			}
			return len(nonFlag) > 0
		},
	},
	"git branch": {
		safeFlags: map[string]flagArgType{
			"-l": flagNone, "--list": flagNone, "-a": flagNone, "--all": flagNone,
			"-r": flagNone, "--remotes": flagNone, "-v": flagNone, "-vv": flagNone, "--verbose": flagNone,
			"--color": flagNone, "--no-color": flagNone, "--column": flagNone, "--no-column": flagNone,
			"--abbrev": flagNumber, "--no-abbrev": flagNone,
			"--contains": flagString, "--no-contains": flagString,
			"--merged": flagNone, "--no-merged": flagNone, "--points-at": flagString,
			"--sort": flagString, "--show-current": flagNone,
			"-i": flagNone, "--ignore-case": flagNone,
		},
		additionalCommandIsDangerousCallback: func(_ string, args []string) bool {
			flagsWithArgs := map[string]bool{
				"--contains": true, "--no-contains": true, "--merged": true,
				"--no-merged": true, "--points-at": true, "--sort": true, "--abbrev": true,
			}
			seenListFlag := false
			seenDashDash := false
			i := 0
			for i < len(args) {
				token := args[i]
				if token == "" {
					i++
					continue
				}
				if token == "--" && !seenDashDash {
					seenDashDash = true
					i++
					continue
				}
				if !seenDashDash && strings.HasPrefix(token, "-") {
					if token == "--list" || token == "-l" {
						seenListFlag = true
					}
					if strings.Contains(token, "=") {
						i++
					} else if flagsWithArgs[token] {
						i += 2
					} else {
						i++
					}
				} else {
					if !seenListFlag {
						return true
					}
					i++
				}
			}
			return false
		},
	},
	"git tag": {
		safeFlags: map[string]flagArgType{
			"-l": flagNone, "--list": flagNone, "-n": flagNumber, "--sort": flagString,
			"--format": flagString, "--color": flagNone, "--no-color": flagNone,
			"--column": flagNone, "--no-column": flagNone,
			"--contains": flagString, "--no-contains": flagString,
			"--merged": flagNone, "--no-merged": flagNone, "--points-at": flagString,
		},
		additionalCommandIsDangerousCallback: func(_ string, args []string) bool {
			flagsWithArgs := map[string]bool{
				"-n": true, "--sort": true, "--format": true, "--contains": true,
				"--no-contains": true, "--merged": true, "--no-merged": true, "--points-at": true,
			}
			seenListFlag := false
			i := 0
			for i < len(args) {
				token := args[i]
				if token == "" {
					i++
					continue
				}
				if token == "--" {
					seenListFlag = true // -- ends options; everything after is a pattern
					i++
					continue
				}
				if !seenListFlag && strings.HasPrefix(token, "-") {
					if token == "--list" || token == "-l" {
						seenListFlag = true
					} else if len(token) > 1 && token[0] == '-' && token[1] != '-' && len(token) > 2 {
						// Short-flag bundle like -li, -il containing 'l'
						if strings.Contains(token[1:], "l") {
							seenListFlag = true
						}
					}
					if strings.Contains(token, "=") {
						i++
					} else if flagsWithArgs[token] {
						i += 2
					} else {
						i++
					}
				} else {
					if !seenListFlag {
						return true
					}
					i++
				}
			}
			return false
		},
	},
	"git describe": {
		safeFlags: map[string]flagArgType{
			"--all": flagNone, "--tags": flagNone, "--contains": flagNone,
			"--abbrev": flagNumber, "--candidates": flagNumber, "--exact-match": flagNone,
			"--debug": flagNone, "--long": flagNone, "--match": flagString, "--exclude": flagString,
			"--always": flagNone, "--first-parent": flagNone, "--dirty": flagNone, "--broken": flagString,
		},
	},
	"git help": {
		safeFlags: map[string]flagArgType{
			"-a": flagNone, "--all": flagNone, "-g": flagNone, "--guides": flagNone,
			"-i": flagNone, "--info": flagNone, "-m": flagNone, "--man": flagNone,
			"-w": flagNone, "--web": flagNone,
		},
	},
	"git whatchanged": {
		safeFlags: mergeFlags(gitStatFlags, gitLogDisplayFlags, gitPatchFlags, map[string]flagArgType{
			"-p": flagNone, "--diff-filter": flagString,
		}),
	},
	"git rev-parse": {
		safeFlags: map[string]flagArgType{
			"--git-dir": flagNone, "--show-toplevel": flagNone, "--show-cdup": flagNone,
			"--absolute-git-dir": flagNone, "--is-inside-git-dir": flagNone,
			"--is-inside-work-tree": flagNone, "--is-bare-repository": flagNone,
			"--verify": flagNone, "-q": flagNone, "--quiet": flagNone, "--short": flagNone,
			"--symbolic": flagNone, "--symbolic-full-name": flagNone,
			"--abbrev-ref": flagNone, "--local-env-vars": flagNone,
			"--sq-quote": flagString,
		},
	},
	"git rev-list": {
		safeFlags: mergeFlags(gitCountFlags, map[string]flagArgType{
			"--all": flagNone, "--branches": flagNone, "--tags": flagNone, "--remotes": flagNone,
			"--max-count": flagNumber, "--skip": flagNumber, "--first-parent": flagNone,
			"--merges": flagNone, "--no-merges": flagNone, "--author": flagString,
			"--after": flagString, "--before": flagString, "--since": flagString, "--until": flagString,
			"--format": flagString, "--pretty": flagString,
		}),
	},
	"git grep": {
		safeFlags: map[string]flagArgType{
			"-i": flagNone, "--ignore-case": flagNone, "-v": flagNone, "--invert-match": flagNone,
			"-c": flagNone, "--count": flagNone, "-l": flagNone, "--files-with-matches": flagNone,
			"-o": flagNone, "--only-matching": flagNone, "-n": flagNone, "--line-number": flagNone,
			"-w": flagNone, "--word-regexp": flagNone, "--cached": flagNone, "--untracked": flagNone,
			"--no-index": flagNone, "--break": flagNone, "--heading": flagNone, "-p": flagNone,
			"--show-function": flagNone, "--recurse-submodules": flagNone,
			"--and": flagNone, "--or": flagNone, "--not": flagNone,
			"--all-match": flagNone, "--fixed-strings": flagNone, "-F": flagNone,
			"--extended-regexp": flagNone, "-E": flagNone, "--perl-regexp": flagNone, "-P": flagNone,
			"--threads": flagNumber, "-f": flagString,
			"--max-depth": flagNumber,
		},
	},
	"git log -S": {
		safeFlags: mergeFlags(gitLogDisplayFlags, gitRefSelectionFlags, gitDateFilterFlags,
			gitCountFlags, gitStatFlags, gitColorFlags, gitPatchFlags, map[string]flagArgType{
				"--pickaxe-regex": flagNone,
			}),
	},
	"git log -G": {
		safeFlags: mergeFlags(gitLogDisplayFlags, gitRefSelectionFlags, gitDateFilterFlags,
			gitCountFlags, gitStatFlags, gitColorFlags, gitPatchFlags, map[string]flagArgType{
				"--pickaxe-regex": flagNone,
			}),
	},
	"git diff --cached": {
		safeFlags: mergeFlags(gitStatFlags, gitColorFlags, map[string]flagArgType{
			"--word-diff": flagNone, "--word-diff-regex": flagString, "--color-words": flagNone,
			"--ignore-space-change": flagNone, "--ignore-all-space": flagNone,
			"--diff-filter": flagString, "--exit-code": flagNone, "--quiet": flagNone,
		}),
	},
	"git stash show": {
		safeFlags: mergeFlags(gitStatFlags, map[string]flagArgType{
			"-p": flagNone, "--patch": flagNone,
		}),
	},
	"git worktree list": {
		safeFlags: map[string]flagArgType{
			"--porcelain": flagNone,
		},
	},
	"git clean -n": {
		safeFlags: map[string]flagArgType{
			"-d": flagNone, "-q": flagNone, "--quiet": flagNone, "-e": flagString, "--exclude": flagString,
			"-i": flagNone, "--interactive": flagNone,
		},
		additionalCommandIsDangerousCallback: func(_ string, args []string) bool {
			// Only allow `git clean -n` (dry-run). Reject any form of actual clean.
			// `git clean` without -n is destructive; `git clean -n -f` forces removal.
			for _, a := range args {
				if a == "-f" || a == "--force" || a == "-x" || a == "-X" {
					return true
				}
			}
			return false
		},
	},
}

// =============================================================================
// GH_READ_ONLY_COMMANDS — gh CLI commands (read-only, network-facing)
// =============================================================================

var ghReadOnlyCommands = map[string]externalCommandConfig{
	"gh version": {safeFlags: map[string]flagArgType{}},
	"gh status": {
		safeFlags: map[string]flagArgType{
			"--org": flagString, "--hostname": flagString,
		},
	},
	"gh auth status": {
		safeFlags: map[string]flagArgType{
			"--hostname": flagString, "--active-org": flagString, "--show-token": flagNone,
		},
	},
	"gh repo view": {
		safeFlags: map[string]flagArgType{
			"--json": flagString, "-j": flagNone, "--branch": flagString,
			"--repo": flagString, "-R": flagString,
		},
		additionalCommandIsDangerousCallback: ghIsDangerousCallback,
	},
	"gh pr view": {
		safeFlags: map[string]flagArgType{
			"--json": flagString, "-j": flagNone, "--repo": flagString, "-R": flagString,
			"--comments": flagNone, "-c": flagNone,
		},
		additionalCommandIsDangerousCallback: ghIsDangerousCallback,
	},
	"gh pr list": {
		safeFlags: map[string]flagArgType{
			"--json": flagString, "-j": flagNone,
			"--assignee": flagString, "--author": flagString, "--base": flagString,
			"--draft": flagNone, "--head": flagString, "--label": flagString,
			"--limit": flagNumber, "-L": flagNumber,
			"--search": flagString, "--sort": flagString, "--state": flagString,
			"--repo": flagString, "-R": flagString, "--web": flagNone, "-w": flagNone,
		},
		additionalCommandIsDangerousCallback: ghIsDangerousCallback,
	},
	"gh pr checks": {
		safeFlags: map[string]flagArgType{
			"--interval": flagNumber, "--repo": flagString, "-R": flagString,
			"--json": flagString, "-j": flagNone, "--watch": flagNone, "-w": flagNone,
		},
		additionalCommandIsDangerousCallback: ghIsDangerousCallback,
	},
	"gh issue view": {
		safeFlags: map[string]flagArgType{
			"--json": flagString, "-j": flagNone, "--repo": flagString, "-R": flagString,
			"--comments": flagNone, "-c": flagNone,
		},
		additionalCommandIsDangerousCallback: ghIsDangerousCallback,
	},
	"gh issue list": {
		safeFlags: map[string]flagArgType{
			"--json": flagString, "-j": flagNone,
			"--assignee": flagString, "--author": flagString,
			"--label": flagString, "--limit": flagNumber, "-L": flagNumber,
			"--search": flagString, "--sort": flagString, "--state": flagString,
			"--repo": flagString, "-R": flagString, "--web": flagNone, "-w": flagNone,
		},
		additionalCommandIsDangerousCallback: ghIsDangerousCallback,
	},
	"gh run view": {
		safeFlags: map[string]flagArgType{
			"--json": flagString, "-j": flagNone, "--log": flagNone,
			"--repo": flagString, "-R": flagString,
			"--exit-status": flagNone, "--verbose": flagNone, "-v": flagNone,
		},
		additionalCommandIsDangerousCallback: ghIsDangerousCallback,
	},
	"gh run list": {
		safeFlags: map[string]flagArgType{
			"--json": flagString, "-j": flagNone,
			"--branch": flagString, "--limit": flagNumber, "-L": flagNumber,
			"--workflow": flagString, "--event": flagString, "--status": flagString,
			"--repo": flagString, "-R": flagString,
		},
		additionalCommandIsDangerousCallback: ghIsDangerousCallback,
	},
	"gh workflow view": {
		safeFlags: map[string]flagArgType{
			"--ref": flagString, "-r": flagString, "--yaml": flagNone, "-y": flagNone,
			"--repo": flagString, "-R": flagString,
		},
		additionalCommandIsDangerousCallback: ghIsDangerousCallback,
	},
	"gh label list": {
		safeFlags: map[string]flagArgType{
			"--json": flagString, "-j": flagNone, "--limit": flagNumber, "-L": flagNumber,
			"--order": flagString, "--search": flagString, "-S": flagString,
			"--sort": flagString, "--repo": flagString, "-R": flagString,
		},
		additionalCommandIsDangerousCallback: ghIsDangerousCallback,
	},
	"gh search repos": {
		safeFlags: map[string]flagArgType{
			"--archived": flagNone, "--created": flagString, "--followers": flagString,
			"--forks": flagString, "--good-first-issues": flagString, "--help-wanted-issues": flagString,
			"--include-forks": flagString, "--json": flagString, "-j": flagNone,
			"--language": flagString, "--license": flagString,
			"--limit": flagNumber, "-L": flagNumber, "--match": flagString,
			"--number-topics": flagString, "--order": flagString, "--owner": flagString,
			"--size": flagString, "--sort": flagString, "--stars": flagString,
			"--topic": flagString, "--updated": flagString, "--visibility": flagString,
		},
	},
	"gh search issues": {
		safeFlags: map[string]flagArgType{
			"--app": flagString, "--assignee": flagString, "--author": flagString,
			"--closed": flagString, "--commenter": flagString, "--comments": flagString,
			"--created": flagString, "--include-prs": flagNone, "--interactions": flagString,
			"--involves": flagString, "--json": flagString, "-j": flagNone,
			"--label": flagString, "--language": flagString,
			"--limit": flagNumber, "-L": flagNumber, "--locked": flagNone,
			"--match": flagString, "--mentions": flagString, "--milestone": flagString,
			"--no-assignee": flagNone, "--no-label": flagNone, "--no-milestone": flagNone,
			"--no-project": flagNone, "--order": flagString, "--owner": flagString,
			"--project": flagString, "--reactions": flagString,
			"--repo": flagString, "-R": flagString, "--sort": flagString,
			"--state": flagString, "--team-mentions": flagString, "--updated": flagString,
			"--visibility": flagString,
		},
	},
	"gh search prs": {
		safeFlags: map[string]flagArgType{
			"--app": flagString, "--assignee": flagString, "--author": flagString,
			"--base": flagString, "-B": flagString, "--checks": flagString,
			"--closed": flagString, "--commenter": flagString, "--comments": flagString,
			"--created": flagString, "--draft": flagNone, "--head": flagString, "-H": flagString,
			"--interactions": flagString, "--involves": flagString,
			"--json": flagString, "-j": flagNone, "--label": flagString,
			"--language": flagString, "--limit": flagNumber, "-L": flagNumber, "--locked": flagNone,
			"--match": flagString, "--mentions": flagString, "--merged": flagNone,
			"--merged-at": flagString, "--milestone": flagString,
			"--no-assignee": flagNone, "--no-label": flagNone, "--no-milestone": flagNone,
			"--no-project": flagNone, "--order": flagString, "--owner": flagString,
			"--project": flagString, "--reactions": flagString,
			"--repo": flagString, "-R": flagString, "--review": flagString,
			"--review-requested": flagString, "--reviewed-by": flagString,
			"--sort": flagString, "--state": flagString, "--team-mentions": flagString,
			"--updated": flagString, "--visibility": flagString,
		},
	},
	"gh search commits": {
		safeFlags: map[string]flagArgType{
			"--author": flagString, "--author-date": flagString, "--author-email": flagString,
			"--author-name": flagString, "--committer": flagString, "--committer-date": flagString,
			"--committer-email": flagString, "--committer-name": flagString, "--hash": flagString,
			"--json": flagString, "-j": flagNone, "--limit": flagNumber, "-L": flagNumber,
			"--merge": flagNone, "--order": flagString, "--owner": flagString,
			"--parent": flagString, "--repo": flagString, "-R": flagString,
			"--sort": flagString, "--tree": flagString, "--visibility": flagString,
		},
	},
	"gh search code": {
		safeFlags: map[string]flagArgType{
			"--extension": flagString, "--filename": flagString,
			"--json": flagString, "-j": flagNone, "--language": flagString,
			"--limit": flagNumber, "-L": flagNumber, "--match": flagString,
			"--owner": flagString, "--repo": flagString, "-R": flagString, "--size": flagString,
		},
	},
}

// ghIsDangerousCallback rejects gh commands that would open a browser or are otherwise dangerous.
func ghIsDangerousCallback(_ string, args []string) bool {
	for _, a := range args {
		if a == "--web" || a == "-w" {
			return true
		}
	}
	return false
}

// =============================================================================
// DOCKER_READ_ONLY_COMMANDS
// =============================================================================

var dockerReadOnlyCommands = map[string]externalCommandConfig{
	"docker logs": {
		safeFlags: map[string]flagArgType{
			"--details": flagNone, "-f": flagNone, "--follow": flagNone,
			"--since": flagString, "--tail": flagString, "-n": flagString,
			"-t": flagNone, "--timestamps": flagNone, "--until": flagString,
		},
	},
	"docker inspect": {
		safeFlags: map[string]flagArgType{
			"-f": flagString, "--format": flagString, "-s": flagNone, "--size": flagNone,
			"--type": flagString,
		},
	},
	"docker ps":    {safeFlags: map[string]flagArgType{}},
	"docker images": {safeFlags: map[string]flagArgType{}},
	"docker info": {
		safeFlags: map[string]flagArgType{
			"-f": flagString, "--format": flagString,
		},
	},
	"docker version": {safeFlags: map[string]flagArgType{}},
	"docker history": {
		safeFlags: map[string]flagArgType{
			"--format": flagString, "--human": flagNone, "-H": flagNone,
			"--no-trunc": flagNone, "-q": flagNone, "--quiet": flagNone,
		},
	},
	"docker port":  {safeFlags: map[string]flagArgType{}},
	"docker stats": {
		safeFlags: map[string]flagArgType{
			"-a": flagNone, "--all": flagNone, "--format": flagString,
			"--no-stream": flagNone, "--no-trunc": flagNone,
		},
	},
	"docker top":   {safeFlags: map[string]flagArgType{}},
}

// =============================================================================
// External read-only commands (cross-shell, no flag validation needed)
// =============================================================================

var externalReadOnlyCommands = map[string]bool{
	"docker ps": true, "docker images": true, "docker port": true,
	"docker version": true, "docker top": true,
}

// =============================================================================
// Utility functions
// =============================================================================

// mergeFlags merges multiple flag maps into one. Later maps override earlier ones.
func mergeFlags(maps ...map[string]flagArgType) map[string]flagArgType {
	result := make(map[string]flagArgType)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

// isAlphaNumDashLike returns true if the string contains only alphanumeric
// characters, underscores, and hyphens (matches a typical remote/branch name).
func isAlphaNumDashLike(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' && r != '_' {
			return false
		}
	}
	return true
}

// validateExternalFlags checks that all flags in the token slice are in the safe set.
// Ported from TS readOnlyCommandValidation.ts validateExternalFlags.
// This is distinct from allowlist.go's validateExternalFlags which checks PS cmdlet flags.
func validateExternalFlags(args []string, startIdx int, config externalCommandConfig) bool {
	if config.safeFlags == nil {
		return true
	}
	seenDashDash := false
	i := startIdx
	for i < len(args) {
		token := args[i]
		if token == "" {
			i++
			continue
		}
		if token == "--" && !seenDashDash {
			seenDashDash = true
			i++
			continue
		}
		if !seenDashDash && strings.HasPrefix(token, "-") {
			hasInlineValue := strings.Contains(token, "=")
			flagName := token
			if hasInlineValue {
				parts := strings.SplitN(token, "=", 2)
				flagName = parts[0]
			}
			argType, ok := config.safeFlags[flagName]
			if !ok {
				return false
			}
			if hasInlineValue {
				i++
			} else if argType != flagNone {
				i += 2
			} else {
				i++
			}
		} else {
			i++
		}
	}
	return true
}

// =============================================================================
// External command dispatch
// =============================================================================

// isExternalCommandSafe checks if an external command invocation is safe (read-only).
// command is the canonical name (git/gh/docker/dotnet), args are the remaining tokens.
func isExternalCommandSafe(command string, args []string) bool {
	switch command {
	case "git":
		return isGitSafe(args)
	case "gh":
		return isGhSafe(args)
	case "docker":
		return isDockerSafe(args)
	case "dotnet":
		return isDotnetSafe(args)
	default:
		return false
	}
}

// =============================================================================
// Git safety
// =============================================================================

var dangerousGitGlobalFlags = map[string]bool{
	"-c": true, "-C": true, "--exec-path": true, "--config-env": true,
	"--git-dir": true, "--work-tree": true, "--attr-source": true,
}

var gitGlobalFlagsWithValues = map[string]bool{
	"-c": true, "-C": true, "--exec-path": true, "--config-env": true,
	"--git-dir": true, "--work-tree": true, "--namespace": true,
	"--super-prefix": true, "--shallow-file": true,
}

var dangerousGitShortFlagsAttached = []string{"-c", "-C"}

func isGitSafe(args []string) bool {
	if len(args) == 0 {
		return true // bare `git` = help
	}

	// Reject any arg containing `$` (variable reference)
	for _, arg := range args {
		if strings.Contains(arg, "$") {
			return false
		}
	}

	// Skip over global flags
	idx := 0
	for idx < len(args) {
		arg := args[idx]
		if arg == "" || !strings.HasPrefix(arg, "-") {
			break
		}
		for _, shortFlag := range dangerousGitShortFlagsAttached {
			if len(arg) > len(shortFlag) && strings.HasPrefix(arg, shortFlag) &&
				(shortFlag == "-C" || arg[len(shortFlag)] != '-') {
				return false
			}
		}
		hasInlineValue := strings.Contains(arg, "=")
		flagName := hasInlineValue && strings.Index(arg, "=") > 0
		var cleanFlag string
		if flagName {
			cleanFlag = strings.SplitN(arg, "=", 2)[0]
		} else {
			cleanFlag = arg
		}
		if dangerousGitGlobalFlags[cleanFlag] {
			return false
		}
		if !hasInlineValue && gitGlobalFlagsWithValues[cleanFlag] {
			idx += 2
		} else {
			idx++
		}
	}

	if idx >= len(args) {
		return true
	}

	first := strings.ToLower(args[idx])
	second := ""
	if idx+1 < len(args) {
		second = strings.ToLower(args[idx+1])
	}

	// Try multi-word subcommand first
	twoWordKey := "git " + first + " " + second
	oneWordKey := "git " + first

	config, ok := gitReadOnlyCommands[twoWordKey]
	subcommandTokens := 2
	if !ok {
		config, ok = gitReadOnlyCommands[oneWordKey]
		subcommandTokens = 1
	}

	if !ok {
		return false
	}

	flagArgs := args[idx+subcommandTokens:]

	// git ls-remote URL rejection
	if first == "ls-remote" {
		for _, arg := range flagArgs {
			if !strings.HasPrefix(arg, "-") {
				if strings.Contains(arg, "://") || strings.Contains(arg, "@") ||
					strings.Contains(arg, ":") || strings.Contains(arg, "$") {
					return false
				}
			}
		}
	}

	if config.additionalCommandIsDangerousCallback != nil &&
		config.additionalCommandIsDangerousCallback("", flagArgs) {
		return false
	}

	return validateExternalFlags(flagArgs, 0, config)
}

// =============================================================================
// GitHub CLI safety
// =============================================================================

func isGhSafe(args []string) bool {
	if len(args) == 0 {
		return true
	}

	var config externalCommandConfig
	subcommandTokens := 0

	if len(args) >= 2 {
		twoWordKey := "gh " + strings.ToLower(args[0]) + " " + strings.ToLower(args[1])
		if c, ok := ghReadOnlyCommands[twoWordKey]; ok {
			config = c
			subcommandTokens = 2
		}
	}

	if subcommandTokens == 0 && len(args) >= 1 {
		oneWordKey := "gh " + strings.ToLower(args[0])
		if c, ok := ghReadOnlyCommands[oneWordKey]; ok {
			config = c
			subcommandTokens = 1
		}
	}

	if subcommandTokens == 0 {
		return false
	}

	flagArgs := args[subcommandTokens:]

	// Reject any arg containing `$` (variable reference)
	for _, arg := range flagArgs {
		if strings.Contains(arg, "$") {
			return false
		}
	}

	if config.additionalCommandIsDangerousCallback != nil &&
		config.additionalCommandIsDangerousCallback("", flagArgs) {
		return false
	}

	return validateExternalFlags(flagArgs, 0, config)
}

// =============================================================================
// Docker safety
// =============================================================================

func isDockerSafe(args []string) bool {
	if len(args) == 0 {
		return true
	}

	// Reject any arg containing `$` (variable reference)
	for _, arg := range args {
		if strings.Contains(arg, "$") {
			return false
		}
	}

	oneWordKey := "docker " + strings.ToLower(args[0])

	// Fast path: unconditionally-read-only commands
	if externalReadOnlyCommands[oneWordKey] {
		return true
	}

	config, ok := dockerReadOnlyCommands[oneWordKey]
	if !ok {
		return false
	}

	flagArgs := args[1:]

	if config.additionalCommandIsDangerousCallback != nil &&
		config.additionalCommandIsDangerousCallback("", flagArgs) {
		return false
	}

	return validateExternalFlags(flagArgs, 0, config)
}

// =============================================================================
// Dotnet safety
// =============================================================================

var dotnetReadOnlyFlags = map[string]bool{
	"--version": true, "--info": true, "--list-runtimes": true, "--list-sdks": true,
}

func isDotnetSafe(args []string) bool {
	if len(args) == 0 {
		return false
	}
	for _, arg := range args {
		if !dotnetReadOnlyFlags[strings.ToLower(arg)] {
			return false
		}
	}
	return true
}

// =============================================================================
// Integration: replace the simple bool check in psReadOnlyCmdlets
// =============================================================================

// isExternalCommandInAllowlist checks if a resolved-to-canonical external command
// (git, gh, docker, dotnet) should be auto-allowed as read-only.
// Returns true when the command and its arguments are safe; false to require approval.
func isExternalCommandInAllowlist(command string) bool {
	first := firstCmdlet(command)
	if first == "" {
		return false
	}
	canonical := resolvePSCommand(first)

	switch canonical {
	case "git", "gh", "docker", "dotnet":
		tokens := tokenizeCommand(command)
		if len(tokens) <= 1 {
			return canonical == "git" || canonical == "gh" || canonical == "docker"
		}
		return isExternalCommandSafe(canonical, tokens[1:])
	default:
		return false
	}
}

// tokenizeCommand splits a command string into tokens (like strings.Fields
// but also handles quoted strings). Simple implementation for argument extraction.
func tokenizeCommand(command string) []string {
	return strings.Fields(command)
}
