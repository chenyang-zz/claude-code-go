package powershell

import (
	"testing"
)

// =============================================================================
// isGitSafe tests
// =============================================================================

func TestIsGitSafe(t *testing.T) {
	tests := []struct {
		name string
		args []string
		safe bool
	}{
		// Bare git = safe (help)
		{"bare git", nil, true},
		{"empty args", []string{}, true},

		// $ rejection in any arg
		{"dollar in arg", []string{"status", "$BRANCH"}, false},
		{"dollar in path", []string{"log", "--format=$format"}, false},
		{"dollar in inline flag", []string{"-c", "user.name=$malicious", "status"}, false},

		// Global flag rejection
		{"-c global flag", []string{"-c", "user.name=foo", "status"}, false},
		{"-C global flag", []string{"-C", "/tmp", "status"}, false},
		{"--git-dir global", []string{"--git-dir", "/tmp/.git", "status"}, false},
		{"--work-tree global", []string{"--work-tree", "/tmp", "status"}, false},
		{"--config-env global", []string{"--config-env", "foo=bar", "status"}, false},
		{"--exec-path global", []string{"--exec-path", "/tmp", "status"}, false},
		{"-c attached value no =", []string{"-cuser.name=foo", "log"}, false},

		// Safe global flags (skipped)
		{"--no-warn flag before subcommand", []string{"--no-warn", "status"}, true},
		{"--paginate flag before subcommand", []string{"--paginate", "status"}, true},
		{"--no-pager flag", []string{"--no-pager", "log"}, true},
		{"--literal-pathspecs", []string{"--literal-pathspecs", "log"}, true},

		// Read-only subcommands: single-word
		{"git status", []string{"status"}, true},
		{"git status -s", []string{"status", "-s"}, true},
		{"git log -n 1", []string{"log", "-n", "1"}, true},
		{"git log --max-count 1", []string{"log", "--max-count", "1"}, true},
		// -1 shorthand is not a recognized flag; use -n 1 or --max-count 1
		{"git log -1 (unrecognized shorthand)", []string{"log", "-1"}, false},
		{"git show HEAD", []string{"show", "HEAD"}, true},
		{"git blame file.go", []string{"blame", "file.go"}, true},
		{"git describe --tags", []string{"describe", "--tags"}, true},
		{"git grep pattern", []string{"grep", "pattern"}, true},
		{"git help -a", []string{"help", "-a"}, true},
		{"git whatchanged", []string{"whatchanged"}, true},
		{"git rev-parse --git-dir", []string{"rev-parse", "--git-dir"}, true},
		{"git rev-list HEAD", []string{"rev-list", "HEAD"}, true},
		{"git shortlog -s", []string{"shortlog", "-s"}, true},
		{"git reflog", []string{"reflog"}, true},
		{"git reflog HEAD", []string{"reflog", "HEAD"}, true},
		{"git branch -l", []string{"branch", "-l"}, true},
		{"git branch --list", []string{"branch", "--list"}, true},
		{"git tag -l", []string{"tag", "-l"}, true},
		{"git tag --list", []string{"tag", "--list"}, true},
		{"git ls-files -c", []string{"ls-files", "-c"}, true},

		// Read-only subcommands: multi-word
		{"git diff HEAD~1", []string{"diff", "HEAD~1"}, true},
		{"git diff --cached", []string{"diff", "--cached"}, true},
		{"git stash list", []string{"stash", "list"}, true},
		{"git stash show", []string{"stash", "show"}, true},
		{"git ls-remote origin", []string{"ls-remote", "origin"}, true},
		{"git config --get user.name", []string{"config", "--get", "user.name"}, true},
		{"git remote show origin", []string{"remote", "show", "origin"}, true},
		{"git remote -v", []string{"remote", "-v"}, true},
		{"git worktree list", []string{"worktree", "list"}, true},
		{"git clean -n", []string{"clean", "-n"}, true},
		{"git log -S foo", []string{"log", "-S", "foo"}, true},
		{"git log -G bar", []string{"log", "-G", "bar"}, true},

		// Dangerous subcommands
		{"git push", []string{"push"}, false},
		{"git push origin main", []string{"push", "origin", "main"}, false},
		{"git commit -m msg", []string{"commit", "-m", "msg"}, false},
		{"git checkout -b new", []string{"checkout", "-b", "new"}, false},
		{"git merge feature", []string{"merge", "feature"}, false},
		{"git rebase main", []string{"rebase", "main"}, false},
		{"git reset HEAD", []string{"reset", "HEAD"}, false},
		{"git fetch origin", []string{"fetch", "origin"}, false},
		{"git pull origin main", []string{"pull", "origin", "main"}, false},
		{"git add .", []string{"add", "."}, false},
		{"git rm file.go", []string{"rm", "file.go"}, false},
		{"git mv src dst", []string{"mv", "src", "dst"}, false},
		{"git config user.name foo", []string{"config", "user.name", "foo"}, false},
		{"git branch new-branch", []string{"branch", "new-branch"}, false},
		{"git tag v1.0", []string{"tag", "v1.0"}, false},

		// git reflog dangerous callbacks
		{"git reflog expire", []string{"reflog", "expire"}, false},
		{"git reflog delete HEAD", []string{"reflog", "delete", "HEAD"}, false},

		// git remote dangerous callbacks
		{"git remote add origin url", []string{"remote", "add", "origin", "url"}, false},
		{"git remote remove origin", []string{"remote", "remove", "origin"}, false},

		// git branch with name (no --list) = dangerous
		{"git branch feat-1", []string{"branch", "feat-1"}, false},

		// git branch --list with pattern = safe
		{"git branch --list feat*", []string{"branch", "--list", "feat*"}, true},

		// git tag with name = dangerous
		{"git tag v1.0", []string{"tag", "v1.0"}, false},

		// git clean -n with -f = dangerous
		{"git clean -n -f", []string{"clean", "-n", "-f"}, false},
		{"git clean -n -x", []string{"clean", "-n", "-x"}, false},
		{"git clean -n --force", []string{"clean", "-n", "--force"}, false},
		{"git clean -n -d", []string{"clean", "-n", "-d"}, true}, // only -d is safe

		// git ls-remote URL rejection
		{"git ls-remote https://evil.com/repo", []string{"ls-remote", "https://evil.com/repo"}, false},
		{"git ls-remote git@evil.com:repo", []string{"ls-remote", "git@evil.com:repo"}, false},
		{"git ls-remote origin", []string{"ls-remote", "origin"}, true},

		// git diff unsafe flags
		{"git diff --output file.patch", []string{"diff", "--output", "file.patch"}, false},

		// git status safe flags
		{"git status --porcelain", []string{"status", "--porcelain"}, true},
		{"git status -u no", []string{"status", "-u", "no"}, true},

		// git describe safe flags
		{"git describe --always --dirty", []string{"describe", "--always", "--dirty"}, true},

		// git describe safe flags
		{"git describe --abbrev=4 --match v*", []string{"describe", "--abbrev", "4", "--match", "v*"}, true},

		// git branch safe flags with contains
		{"git branch --contains v1.0", []string{"branch", "--contains", "v1.0"}, true},

		// git branch --merged
		{"git branch --merged main", []string{"branch", "--merged", "main"}, true},

		// Unknown subcommand
		{"git unknown-cmd", []string{"unknown-cmd"}, false},
		{"git difftool HEAD HEAD", []string{"difftool", "HEAD", "HEAD"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isGitSafe(tt.args)
			if got != tt.safe {
				t.Errorf("isGitSafe(%v) = %v, want %v", tt.args, got, tt.safe)
			}
		})
	}
}

// =============================================================================
// isGhSafe tests
// =============================================================================

func TestIsGhSafe(t *testing.T) {
	tests := []struct {
		name string
		args []string
		safe bool
	}{
		// Bare gh = safe
		{"bare gh", nil, true},
		{"empty args", []string{}, true},

		// $ rejection
		{"dollar in args", []string{"pr", "view", "$PR_NUM"}, false},
		{"dollar in flag", []string{"pr", "list", "--search", "$label"}, false},

		// Two-word subcommands
		{"gh pr view 123", []string{"pr", "view", "123"}, true},
		{"gh pr list", []string{"pr", "list"}, true},
		{"gh pr checks", []string{"pr", "checks"}, true},
		{"gh issue view 123", []string{"issue", "view", "123"}, true},
		{"gh issue list", []string{"issue", "list"}, true},
		{"gh run view 123", []string{"run", "view", "123"}, true},
		{"gh run list", []string{"run", "list"}, true},
		{"gh repo view owner/repo", []string{"repo", "view", "owner/repo"}, true},
		{"gh workflow view", []string{"workflow", "view"}, true},
		{"gh auth status", []string{"auth", "status"}, true},
		{"gh search repos lang:go", []string{"search", "repos", "lang:go"}, true},
		{"gh search issues label:bug", []string{"search", "issues", "label:bug"}, true},
		{"gh search prs state:open", []string{"search", "prs", "state:open"}, true},
		{"gh search commits author:me", []string{"search", "commits", "author:me"}, true},
		{"gh search code lang:py", []string{"search", "code", "lang:py"}, true},
		{"gh label list", []string{"label", "list"}, true},

		// Single-word subcommands
		{"gh version", []string{"version"}, true},
		{"gh status", []string{"status"}, true},

		// --web/-w rejection via dangerous callback
		{"gh pr view --web", []string{"pr", "view", "--web"}, false},
		{"gh pr list -w", []string{"pr", "list", "-w"}, false},
		{"gh issue view --web 123", []string{"issue", "view", "--web", "123"}, false},
		{"gh repo view -w", []string{"repo", "view", "-w"}, false},
		{"gh run view --web", []string{"run", "view", "--web"}, false},
		{"gh workflow view -w", []string{"workflow", "view", "-w"}, false},
		{"gh label list --web", []string{"label", "list", "--web"}, false},

		// Flag validation
		{"gh pr list --json number", []string{"pr", "list", "--json", "number"}, true},
		{"gh pr list --state open --limit 10", []string{"pr", "list", "--state", "open", "--limit", "10"}, true},
		{"gh pr list --unknown-flag", []string{"pr", "list", "--unknown-flag"}, false},
		{"gh issue list --author me", []string{"issue", "list", "--author", "me"}, true},
		{"gh issue list --milestone v1", []string{"issue", "list", "--milestone", "v1"}, true},

		// Unknown subcommand
		{"gh unknown", []string{"unknown"}, false},
		{"gh pr create", []string{"pr", "create"}, false},
		{"gh issue create", []string{"issue", "create"}, false},
		{"gh release list", []string{"release", "list"}, false},
		{"gh repo create", []string{"repo", "create"}, false},
		{"gh gist create", []string{"gist", "create"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isGhSafe(tt.args)
			if got != tt.safe {
				t.Errorf("isGhSafe(%v) = %v, want %v", tt.args, got, tt.safe)
			}
		})
	}
}

// =============================================================================
// isDockerSafe tests
// =============================================================================

func TestIsDockerSafe(t *testing.T) {
	tests := []struct {
		name string
		args []string
		safe bool
	}{
		// Bare docker = safe
		{"bare docker", nil, true},
		{"empty args", []string{}, true},

		// $ rejection in ALL args (not just flagArgs)
		{"dollar in any arg", []string{"logs", "$container", "tail"}, false},
		{"dollar in container name", []string{"inspect", "my$container"}, false},

		// Fast path (unconditionally read-only)
		{"docker ps", []string{"ps"}, true},
		{"docker images", []string{"images"}, true},
		{"docker port", []string{"port"}, true},
		{"docker version", []string{"version"}, true},
		{"docker top", []string{"top"}, true},
		{"docker ps -a", []string{"ps", "-a"}, true},  // fast path ignores flags
		{"docker images -q", []string{"images", "-q"}, true},

		// Flag-validated commands
		{"docker logs mycontainer", []string{"logs", "mycontainer"}, true},
		{"docker logs -f mycontainer", []string{"logs", "-f", "mycontainer"}, true},
		{"docker logs --tail 100 mycontainer", []string{"logs", "--tail", "100", "mycontainer"}, true},
		{"docker logs --since 1h", []string{"logs", "--since", "1h"}, true},
		{"docker inspect mycontainer", []string{"inspect", "mycontainer"}, true},
		{"docker inspect --format '{{.Name}}'", []string{"inspect", "--format", "{{.Name}}"}, true},
		{"docker info", []string{"info"}, true},
		{"docker info --format json", []string{"info", "--format", "json"}, true},
		{"docker history myimage", []string{"history", "myimage"}, true},
		{"docker history --no-trunc myimage", []string{"history", "--no-trunc", "myimage"}, true},
		{"docker stats", []string{"stats"}, true},
		{"docker stats --no-stream", []string{"stats", "--no-stream"}, true},
		{"docker stats -a", []string{"stats", "-a"}, true},

		// Unknown flags in validated commands
		{"docker logs --unknown mycontainer", []string{"logs", "--unknown", "mycontainer"}, false},
		{"docker inspect --unknown x", []string{"inspect", "--unknown", "x"}, false},

		// Dangerous subcommands
		{"docker run nginx", []string{"run", "nginx"}, false},
		{"docker exec -it bash", []string{"exec", "-it", "bash"}, false},
		{"docker build .", []string{"build", "."}, false},
		{"docker push myimage", []string{"push", "myimage"}, false},
		{"docker pull nginx", []string{"pull", "nginx"}, false},
		{"docker compose up", []string{"compose", "up"}, false},
		{"docker network create", []string{"network", "create"}, false},
		{"docker volume create", []string{"volume", "create"}, false},
		{"docker stop container", []string{"stop", "container"}, false},
		{"docker start container", []string{"start", "container"}, false},
		{"docker rm container", []string{"rm", "container"}, false},
		{"docker rmi image", []string{"rmi", "image"}, false},
		{"docker kill container", []string{"kill", "container"}, false},

		// Unknown subcommand
		{"docker unknown", []string{"unknown"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDockerSafe(tt.args)
			if got != tt.safe {
				t.Errorf("isDockerSafe(%v) = %v, want %v", tt.args, got, tt.safe)
			}
		})
	}
}

// =============================================================================
// isDotnetSafe tests
// =============================================================================

func TestIsDotnetSafe(t *testing.T) {
	tests := []struct {
		name string
		args []string
		safe bool
	}{
		// Empty/unrecognized
		{"bare dotnet", []string{}, false},
		{"no args", nil, false},

		// Known read-only flags
		{"dotnet --version", []string{"--version"}, true},
		{"dotnet --info", []string{"--info"}, true},
		{"dotnet --list-runtimes", []string{"--list-runtimes"}, true},
		{"dotnet --list-sdks", []string{"--list-sdks"}, true},

		// Case insensitive
		{"dotnet --VERSION", []string{"--VERSION"}, true},
		{"dotnet --INFO", []string{"--INFO"}, true},

		// Dangerous subcommands
		{"dotnet build", []string{"build"}, false},
		{"dotnet run", []string{"run"}, false},
		{"dotnet test", []string{"test"}, false},
		{"dotnet publish", []string{"publish"}, false},
		{"dotnet restore", []string{"restore"}, false},
		{"dotnet add package", []string{"add", "package"}, false},
		{"dotnet new webapp", []string{"new", "webapp"}, false},
		{"dotnet tool install", []string{"tool", "install"}, false},

		// Mixed safe + dangerous
		{"dotnet --version build", []string{"--version", "build"}, false},
		{"dotnet --info --version", []string{"--info", "--version"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDotnetSafe(tt.args)
			if got != tt.safe {
				t.Errorf("isDotnetSafe(%v) = %v, want %v", tt.args, got, tt.safe)
			}
		})
	}
}

// =============================================================================
// isExternalCommandSafe tests
// =============================================================================

func TestIsExternalCommandSafe(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		safe    bool
	}{
		// git
		{"git status", "git", []string{"status"}, true},
		{"git push", "git", []string{"push", "origin", "main"}, false},
		{"git --no-pager log", "git", []string{"--no-pager", "log"}, true},

		// gh
		{"gh pr list", "gh", []string{"pr", "list"}, true},
		{"gh pr create", "gh", []string{"pr", "create"}, false},

		// docker
		{"docker ps", "docker", []string{"ps"}, true},
		{"docker run nginx", "docker", []string{"run", "nginx"}, false},

		// dotnet
		{"dotnet --version", "dotnet", []string{"--version"}, true},
		{"dotnet build", "dotnet", []string{"build"}, false},

		// unknown command
		{"python script.py", "python", []string{"script.py"}, false},
		{"npm install", "npm", []string{"install"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isExternalCommandSafe(tt.command, tt.args)
			if got != tt.safe {
				t.Errorf("isExternalCommandSafe(%q, %v) = %v, want %v", tt.command, tt.args, got, tt.safe)
			}
		})
	}
}

// =============================================================================
// isExternalCommandInAllowlist tests
// =============================================================================

// TestIsExternalCommandInAllowlist tests the integration point used by
// isReadOnlyPSCmdletChecked for external commands.
func TestIsExternalCommandInAllowlist(t *testing.T) {
	tests := []struct {
		name    string
		command string
		safe    bool
	}{
		// git
		{"git status", "git status", true},
		{"git diff", "git diff", true},
		{"git push", "git push origin main", false},
		{"git commit -m msg", "git commit -m msg", false},
		{"git bare (help)", "git", true},
		{"git reflog expire", "git reflog expire", false},
		{"git branch feat-1", "git branch feat-1", false},
		{"git branch --list feat*", "git branch --list feat*", true},
		{"git clean -n -f", "git clean -n -f", false},
		{"git clean -n -d", "git clean -n -d", true},

		// gh
		{"gh pr list", "gh pr list", true},
		{"gh pr create", "gh pr create", false},
		{"gh pr view --web", "gh pr view --web", false},
		{"gh status", "gh status", true},
		{"gh bare (help)", "gh", true},

		// docker
		{"docker ps", "docker ps", true},
		{"docker run nginx", "docker run nginx", false},
		{"docker logs --tail 10 myapp", "docker logs --tail 10 myapp", true},
		{"docker bare (help)", "docker", true},
		{"docker exec -it bash", "docker exec -it bash", false},

		// dotnet
		{"dotnet --version", "dotnet --version", true},
		{"dotnet build", "dotnet build", false},
		{"dotnet bare (unsafe)", "dotnet", false},

		// Non-external commands
		{"Get-ChildItem", "Get-ChildItem C:\\", false},
		{"npm install", "npm install", false},
		{"pip install", "pip install requests", false},

		// Empty command
		{"empty", "", false},

		// $ injection
		{"git with $", "git status $BRANCH", false},
		{"gh with $", "gh pr view $PR_NUM", false},
		{"docker with $", "docker logs $container", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isExternalCommandInAllowlist(tt.command)
			if got != tt.safe {
				t.Errorf("isExternalCommandInAllowlist(%q) = %v, want %v", tt.command, got, tt.safe)
			}
		})
	}
}

// =============================================================================
// validateExternalFlags tests
// =============================================================================

func TestValidateExternalFlags(t *testing.T) {
	config := externalCommandConfig{
		safeFlags: map[string]flagArgType{
			"--verbose":  flagNone,
			"-v":         flagNone,
			"--output":   flagString,
			"--count":    flagNumber,
			"--format":   flagString,
			"--name":     flagString,
			"--json":     flagString,
			"--limit":    flagNumber,
			"-L":         flagNumber,
			"--all":      flagNone,
			"--no-color": flagNone,
		},
	}

	tests := []struct {
		name     string
		args     []string
		startIdx int
		safe     bool
	}{
		// No args
		{"empty args", []string{}, 0, true},

		// Boolean flags
		{"-v", []string{"-v"}, 0, true},
		{"--verbose", []string{"--verbose"}, 0, true},
		{"--all", []string{"--all"}, 0, true},

		// String flags
		{"--output file.txt", []string{"--output", "file.txt"}, 0, true},
		{"--format json", []string{"--format", "json"}, 0, true},

		// Number flags
		{"--count 10", []string{"--count", "10"}, 0, true},
		{"--limit 5", []string{"--limit", "5"}, 0, true},
		{"-L 100", []string{"-L", "100"}, 0, true},

		// Inline value with =
		{"--output=file.txt", []string{"--output=file.txt"}, 0, true},
		{"--limit=10", []string{"--limit=10"}, 0, true},
		{"--count=5", []string{"--count=5"}, 0, true},
		{"--json=number", []string{"--json=number"}, 0, true},
		{"--name=foo", []string{"--name=foo"}, 0, true},

		// Start index
		{"startIdx=1 skip first", []string{"skip", "--verbose"}, 1, true},
		{"startIdx=2", []string{"a", "b", "--verbose", "--all"}, 2, true},

		// Unknown flags
		{"--unknown flag", []string{"--unknown"}, 0, false},
		{"--unsafe-flag", []string{"--unsafe-flag"}, 0, false},
		{"--output --unknown", []string{"--output", "file.txt", "--unknown"}, 0, false},

		// -- dash-dash handling
		{"-- before positional", []string{"--", "positional"}, 0, true},
		{"-- before unknown", []string{"--", "--unknown"}, 0, true},
		{"-- before flag", []string{"--", "--verbose"}, 0, true},

		// Empty tokens in between
		{"empty token", []string{"", "--verbose"}, 0, true},
		{"empty token between flags", []string{"--verbose", "", "--all"}, 0, true},

		// Mixed
		{"--verbose --output file.txt", []string{"--verbose", "--output", "file.txt"}, 0, true},
		{"--verbose --unknown", []string{"--verbose", "--unknown"}, 0, false},
		{"--json --output file.txt", []string{"--json", "json", "--output", "file.txt"}, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateExternalFlags(tt.args, tt.startIdx, config)
			if got != tt.safe {
				t.Errorf("validateExternalFlags(%v, %d, config) = %v, want %v", tt.args, tt.startIdx, got, tt.safe)
			}
		})
	}
}

func TestValidateExternalFlagsNilConfig(t *testing.T) {
	// nil safeFlags should return true
	config := externalCommandConfig{safeFlags: nil}
	if !validateExternalFlags([]string{"--anything"}, 0, config) {
		t.Error("expected nil safeFlags to allow everything")
	}
}

// =============================================================================
// mergeFlags tests
// =============================================================================

func TestMergeFlags(t *testing.T) {
	t.Run("merge two maps", func(t *testing.T) {
		a := map[string]flagArgType{"--verbose": flagNone, "--output": flagString}
		b := map[string]flagArgType{"--quiet": flagNone, "--output": flagNone} // --output overridden
		result := mergeFlags(a, b)

		if result["--verbose"] != flagNone {
			t.Error("--verbose should be flagNone")
		}
		if result["--quiet"] != flagNone {
			t.Error("--quiet should be flagNone")
		}
		if result["--output"] != flagNone { // b overrides a
			t.Error("--output should be overridden to flagNone from b")
		}
		if len(result) != 3 {
			t.Errorf("expected 3 keys, got %d", len(result))
		}
	})

	t.Run("single map", func(t *testing.T) {
		a := map[string]flagArgType{"--verbose": flagNone}
		result := mergeFlags(a)
		if result["--verbose"] != flagNone {
			t.Error("--verbose should be flagNone")
		}
		if len(result) != 1 {
			t.Errorf("expected 1 key, got %d", len(result))
		}
	})

	t.Run("no maps", func(t *testing.T) {
		result := mergeFlags()
		if len(result) != 0 {
			t.Errorf("expected empty map, got %d keys", len(result))
		}
	})

	t.Run("three maps", func(t *testing.T) {
		a := map[string]flagArgType{"-v": flagNone}
		b := map[string]flagArgType{"--verbose": flagNone}
		c := map[string]flagArgType{"-v": flagString}
		result := mergeFlags(a, b, c)
		if len(result) != 2 {
			t.Errorf("expected 2 keys, got %d", len(result))
		}
		if result["-v"] != flagString { // c overrides a
			t.Error("-v should be overridden to flagString from c")
		}
	})
}

// =============================================================================
// isAlphaNumDashLike tests
// =============================================================================

func TestIsAlphaNumDashLike(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		// Valid
		{"origin", true},
		{"main", true},
		{"feature-branch", true},
		{"FEATURE_BRANCH", true},
		{"v1.0.0", false}, // dots not allowed
		{"feature/branch", false}, // slashes not allowed
		{"my_branch", true},
		{"MyBranch_123", true},
		// Invalid
		{"", false},
		{"has space", false},
		{"url://bad", false},
		{"path/file", false},
		{"user@host", false},
		{"with colon:", false},
		{"with$dollar", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isAlphaNumDashLike(tt.input)
			if got != tt.valid {
				t.Errorf("isAlphaNumDashLike(%q) = %v, want %v", tt.input, got, tt.valid)
			}
		})
	}
}

// =============================================================================
// ghIsDangerousCallback tests
// =============================================================================

func TestGhIsDangerousCallback(t *testing.T) {
	tests := []struct {
		name string
		args []string
		dangerous bool
	}{
		{"no args", []string{}, false},
		{"--web flag", []string{"--web"}, true},
		{"-w flag", []string{"-w"}, true},
		{"safe flags only", []string{"--json", "number"}, false},
		{"mixed safe and --web", []string{"--json", "number", "--web"}, true},
		{"mixed safe and -w", []string{"-R", "owner/repo", "-w"}, true},
		{"flag-like but not -w", []string{"--watch"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ghIsDangerousCallback("", tt.args)
			if got != tt.dangerous {
				t.Errorf("ghIsDangerousCallback(_, %v) = %v, want %v", tt.args, got, tt.dangerous)
			}
		})
	}
}

// =============================================================================
// Edge cases and integration tests
// =============================================================================

// TestExternalCommandEdgeCases covers tricky boundary conditions.
func TestExternalCommandEdgeCases(t *testing.T) {
	t.Run("git with only global flags", func(t *testing.T) {
		// git --no-pager --literal-pathspecs should be safe (bare git = help)
		if !isGitSafe([]string{"--no-pager", "--literal-pathspecs"}) {
			t.Error("expected git with only global flags to be safe")
		}
	})

	t.Run("git with dangerous attached short flag", func(t *testing.T) {
		// -C attached to value without space: -C/tmp
		if isGitSafe([]string{"-C/tmp", "status"}) {
			t.Error("expected git with -C/tmp to be unsafe")
		}
	})

	t.Run("git -C /tmp status is unsafe", func(t *testing.T) {
		if isGitSafe([]string{"-C", "/tmp", "status"}) {
			t.Error("expected git with -C /tmp to be unsafe because -C changes directory")
		}
	})

	t.Run("gh with two-word subcommand uses correct index", func(t *testing.T) {
		// gh pr view 123 with --json flag
		if !isGhSafe([]string{"pr", "view", "123", "--json", "title"}) {
			t.Error("expected gh pr view with --json to be safe")
		}
	})

	t.Run("gh two-word fallback to one-word", func(t *testing.T) {
		// gh status with flag
		if !isGhSafe([]string{"status", "--org", "myorg"}) {
			t.Error("expected gh status --org to be safe")
		}
	})

	t.Run("docker with unknown subcommand but safe fast path not matched", func(t *testing.T) {
		if isDockerSafe([]string{"unknown"}) {
			t.Error("expected docker unknown to be unsafe")
		}
	})

	t.Run("dotnet empty args means unsafe", func(t *testing.T) {
		if isDotnetSafe([]string{}) {
			t.Error("expected dotnet with no args to be unsafe")
		}
	})

	t.Run("isExternalCommandSafe with unknown command", func(t *testing.T) {
		if isExternalCommandSafe("python", []string{"script.py"}) {
			t.Error("expected python to be unsafe")
		}
	})

	t.Run("isExternalCommandInAllowlist with dotnet bare", func(t *testing.T) {
		// dotnet with no args is unsafe
		if isExternalCommandInAllowlist("dotnet") {
			t.Error("expected bare dotnet to be unsafe")
		}
	})

	t.Run("isExternalCommandInAllowlist with non-external command", func(t *testing.T) {
		if isExternalCommandInAllowlist("Get-ChildItem C:\\") {
			t.Error("expected Get-ChildItem to not be handled by external command allowlist")
		}
	})

	t.Run("tokenizeCommand handles basic splitting", func(t *testing.T) {
		tokens := tokenizeCommand("git log --oneline -5")
		expected := []string{"git", "log", "--oneline", "-5"}
		if len(tokens) != len(expected) {
			t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
		}
		for i := range tokens {
			if tokens[i] != expected[i] {
				t.Errorf("token[%d] = %q, want %q", i, tokens[i], expected[i])
			}
		}
	})

	t.Run("tokenizeCommand handles multiple spaces", func(t *testing.T) {
		tokens := tokenizeCommand("git   status   -s")
		expected := []string{"git", "status", "-s"}
		if len(tokens) != len(expected) {
			t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
		}
	})

	t.Run("tokenizeCommand handles double-quoted string", func(t *testing.T) {
		tokens := tokenizeCommand("git log --format=\"%H %s\"")
		expected := []string{"git", "log", "--format=%H %s"}
		if len(tokens) != len(expected) {
			t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
		}
		for i := range tokens {
			if tokens[i] != expected[i] {
				t.Errorf("token[%d] = %q, want %q", i, tokens[i], expected[i])
			}
		}
	})

	t.Run("tokenizeCommand handles single-quoted string", func(t *testing.T) {
		tokens := tokenizeCommand("Write-Output 'Hello World'")
		expected := []string{"Write-Output", "Hello World"}
		if len(tokens) != len(expected) {
			t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
		}
	})

	t.Run("tokenizeCommand handles single quotes inside double quotes", func(t *testing.T) {
		tokens := tokenizeCommand("Write-Output \"it's fine\"")
		expected := []string{"Write-Output", "it's fine"}
		if len(tokens) != len(expected) {
			t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
		}
	})

	t.Run("tokenizeCommand handles backtick escape inside double quotes", func(t *testing.T) {
		tokens := tokenizeCommand("Write-Output \"hello`\"world\"")
		expected := []string{"Write-Output", "hello\"world"}
		if len(tokens) != len(expected) {
			t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
		}
	})

	t.Run("tokenizeCommand handles backtick escape outside quotes", func(t *testing.T) {
		tokens := tokenizeCommand("echo `$env:VAR")
		expected := []string{"echo", "$env:VAR"}
		if len(tokens) != len(expected) {
			t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
		}
	})

	t.Run("tokenizeCommand handles empty double quotes", func(t *testing.T) {
		tokens := tokenizeCommand("Write-Output \"\"")
		expected := []string{"Write-Output", ""}
		if len(tokens) != len(expected) {
			t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
		}
	})

	t.Run("tokenizeCommand handles empty single quotes", func(t *testing.T) {
		tokens := tokenizeCommand("Write-Output ''")
		expected := []string{"Write-Output", ""}
		if len(tokens) != len(expected) {
			t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
		}
	})

	t.Run("tokenizeCommand handles escaped single quote inside single quotes", func(t *testing.T) {
		tokens := tokenizeCommand("Write-Output 'it''s cool'")
		expected := []string{"Write-Output", "it's cool"}
		if len(tokens) != len(expected) {
			t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
		}
	})

	t.Run("tokenizeCommand handles flag with quoted value", func(t *testing.T) {
		tokens := tokenizeCommand("gh pr list --search \"is:open label:bug\"")
		expected := []string{"gh", "pr", "list", "--search", "is:open label:bug"}
		if len(tokens) != len(expected) {
			t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
		}
	})

	t.Run("tokenizeCommand handles mixed quoting in git command", func(t *testing.T) {
		tokens := tokenizeCommand("git log --author=\"John Doe\" --grep='fix bug' -5")
		expected := []string{"git", "log", "--author=John Doe", "--grep=fix bug", "-5"}
		if len(tokens) != len(expected) {
			t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
		}
	})

	t.Run("tokenizeCommand handles tab and newline whitespace", func(t *testing.T) {
		tokens := tokenizeCommand("git\tstatus\n-v")
		expected := []string{"git", "status", "-v"}
		if len(tokens) != len(expected) {
			t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
		}
	})

	t.Run("tokenizeCommand handles empty command", func(t *testing.T) {
		tokens := tokenizeCommand("")
		if len(tokens) != 0 {
			t.Errorf("expected 0 tokens, got %d: %v", len(tokens), tokens)
		}
	})

	t.Run("tokenizeCommand handles whitespace-only command", func(t *testing.T) {
		tokens := tokenizeCommand("   \t  \n  ")
		if len(tokens) != 0 {
			t.Errorf("expected 0 tokens, got %d: %v", len(tokens), tokens)
		}
	})

	t.Run("tokenizeCommand handles trailing backtick", func(t *testing.T) {
		tokens := tokenizeCommand("echo hi`")
		expected := []string{"echo", "hi"}
		if len(tokens) != len(expected) {
			t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
		}
	})

	t.Run("tokenizeCommand handles unmatched double quote", func(t *testing.T) {
		tokens := tokenizeCommand("echo \"hello world")
		expected := []string{"echo", "hello world"}
		if len(tokens) != len(expected) {
			t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
		}
	})


}

// =============================================================================
// Real-world scenario tests
// =============================================================================

// TestExternalCommandRealWorldScenarios tests realistic command invocations
// that might appear in actual agent usage.
func TestExternalCommandRealWorldScenarios(t *testing.T) {
	tests := []struct {
		name    string
		command string
		safe    bool
	}{
		// Developer workspace inspection
		{"git status -s", "git status -s", true},
		{"git diff --stat", "git diff --stat", true},
		{"git diff --cached --stat", "git diff --cached --stat", true},
		{"git log --oneline -n 5", "git log --oneline -n 5", true},
		{"git log --oneline --max-count 5", "git log --oneline --max-count 5", true},
		// -5 shorthand is not recognized by flag validation; use -n 5 instead
		{"git log --oneline -5 (unrecognized shorthand)", "git log --oneline -5", false},
		{"git log --all --graph --oneline", "git log --all --graph --oneline", true},
		{"git show HEAD", "git show HEAD", true},
		{"git blame main.go", "git blame main.go", true},
		{"git branch -l", "git branch -l", true},
		{"git branch --list feat*", "git branch --list feat*", true},
		{"git tag -l", "git tag -l", true},
		{"git tag --list v*", "git tag --list v*", true},
		{"git describe --tags --always", "git describe --tags --always", true},
		{"git reflog", "git reflog", true},
		{"git shortlog -s -n", "git shortlog -s -n", true},
		{"git rev-parse --git-dir", "git rev-parse --git-dir", true},
		{"git rev-parse --show-toplevel", "git rev-parse --show-toplevel", true},
		{"git ls-files -c", "git ls-files -c", true},
		{"git worktree list", "git worktree list", true},
		{"git grep -n 'TODO'", "git grep -n 'TODO'", true},
		{"git grep -i 'func main'", "git grep -i 'func main'", true},

		// GitHub CLI inspection
		{"gh pr list --state open --limit 10", "gh pr list --state open --limit 10", true},
		{"gh pr view 123 --json title,body", "gh pr view 123 --json title,body", true},
		{"gh pr checks 123", "gh pr checks 123", true},
		{"gh issue list --label bug", "gh issue list --label bug", true},
		{"gh issue view 456", "gh issue view 456", true},
		{"gh run list --limit 5", "gh run list --limit 5", true},
		{"gh run view 789", "gh run view 789", true},
		{"gh workflow view main.yaml", "gh workflow view main.yaml", true},
		{"gh repo view owner/repo", "gh repo view owner/repo", true},
		{"gh auth status", "gh auth status", true},
		{"gh search repos 'lang:go stars:>1000'", "gh search repos 'lang:go stars:>1000'", true},
		{"gh search issues 'label:bug state:open'", "gh search issues 'label:bug state:open'", true},
		{"gh search code 'lang:python filename:main'", "gh search code 'lang:python filename:main'", true},
		{"gh label list -R owner/repo", "gh label list -R owner/repo", true},

		// Docker inspection
		{"docker ps -a", "docker ps -a", true},
		{"docker images", "docker images", true},
		{"docker logs --tail 50 myapp", "docker logs --tail 50 myapp", true},
		{"docker inspect mycontainer", "docker inspect mycontainer", true},
		{"docker stats --no-stream", "docker stats --no-stream", true},
		{"docker info", "docker info", true},
		{"docker version", "docker version", true},
		{"docker history myimage", "docker history myimage", true},

		// Dotnet inspection
		{"dotnet --version", "dotnet --version", true},
		{"dotnet --list-runtimes", "dotnet --list-runtimes", true},
		{"dotnet --list-sdks", "dotnet --list-sdks", true},
		{"dotnet --info", "dotnet --info", true},

		// Dangerous operations (should be blocked)
		{"git push origin main", "git push origin main", false},
		{"git commit -m msg", "git commit -m msg", false},
		{"git remote add origin url", "git remote add origin url", false},
		{"git config user.name foo", "git config user.name foo", false},
		{"git reflog expire --all", "git reflog expire --all", false},
		{"git clean -n -f -d", "git clean -n -f -d", false},
		{"gh pr create --title x --body y", "gh pr create --title x --body y", false},
		{"gh issue create --title x", "gh issue create --title x", false},
		{"gh pr view --web", "gh pr view --web", false},
		{"docker run nginx", "docker run nginx", false},
		{"docker exec -it bash", "docker exec -it bash", false},
		{"docker build -t myimage .", "docker build -t myimage .", false},
		{"docker push myimage", "docker push myimage", false},
		{"dotnet build", "dotnet build", false},
		{"dotnet run", "dotnet run", false},

		// Variable injection (should be blocked)
		{"git status $BRANCH", "git status $BRANCH", false},
		{"git log --format=$format", "git log --format=$format", false},
		{"gh pr view $PR_NUM", "gh pr view $PR_NUM", false},
		{"docker logs $container", "docker logs $container", false},
		{"git -c user.name=$malicious status", "git -c user.name=$malicious status", false},

		// Global flag attacks
		{"git --git-dir=/tmp/.git status", "git --git-dir=/tmp/.git status", false},
		{"git --work-tree=/tmp branch", "git --work-tree=/tmp branch", false},
		{"git -C /somewhere status", "git -C /somewhere status", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isExternalCommandInAllowlist(tt.command)
			if got != tt.safe {
				t.Errorf("isExternalCommandInAllowlist(%q) = %v, want %v", tt.command, got, tt.safe)
			}
		})
	}
}
