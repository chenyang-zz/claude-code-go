package powershell

import (
	"testing"
)

func TestIsReadOnlyPSCmdlet(t *testing.T) {
	tests := []struct {
		command  string
		readOnly bool
	}{
		{"Get-ChildItem C:\\", true},
		{"ls -la", true},
		{"Get-Content file.txt", true},
		{"select-string pattern file.txt", true},
		{"Get-Process", true},
		{"Write-Output hello", true},
		{"format-table", true},
		{"where-object { $_.CPU -gt 100 }", true},
		{"git status", true},
		// Write cmdlets
		{"Remove-Item / -Recurse", false},
		{"Set-Content file.txt 'hello'", false},
		{"New-Item -Path dir -ItemType Directory", false},
		{"Invoke-Expression 'test'", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := isReadOnlyPSCmdlet(tt.command)
			if got != tt.readOnly {
				t.Errorf("isReadOnlyPSCmdlet(%q) = %v, want %v", tt.command, got, tt.readOnly)
			}
		})
	}
}

func TestIsAcceptEditsCmdlet(t *testing.T) {
	tests := []struct {
		command string
		accept  bool
	}{
		{"New-Item -Path dir -ItemType Directory", true},
		{"Set-Content file.txt 'hello'", true},
		{"Remove-Item file.txt", true},
		{"Copy-Item src dst", true},
		{"Move-Item src dst", true},
		// Read-only cmdlets
		{"Get-ChildItem C:\\", false},
		{"Write-Output hello", false},
		{"Invoke-Expression 'test'", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := isAcceptEditsCmdlet(tt.command)
			if got != tt.accept {
				t.Errorf("isAcceptEditsCmdlet(%q) = %v, want %v", tt.command, got, tt.accept)
			}
		})
	}
}

func TestIsSafeOutputCmdlet(t *testing.T) {
	tests := []struct {
		command string
		safe    bool
	}{
		{"format-table", true},
		{"select-object Name, Id", true},
		{"sort-object Name", true},
		{"where-object { $_.CPU -gt 100 }", true},
		{"convertto-json", true},
		// Non-safe cmdlets
		{"Get-ChildItem C:\\", false},
		{"Remove-Item /", false},
		{"Set-Content file.txt 'hello'", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := isSafeOutputCmdlet(tt.command)
			if got != tt.safe {
				t.Errorf("isSafeOutputCmdlet(%q) = %v, want %v", tt.command, got, tt.safe)
			}
		})
	}
}

func TestFirstCmdlet(t *testing.T) {
	tests := []struct {
		command string
		first   string
	}{
		{"Get-ChildItem C:\\", "Get-ChildItem"},
		{"ls -la", "ls"},
		{"  git status  ", "git"},
		{"", ""},
		{"   ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := firstCmdlet(tt.command)
			if got != tt.first {
				t.Errorf("firstCmdlet(%q) = %q, want %q", tt.command, got, tt.first)
			}
		})
	}
}

func TestSplitSubCommands(t *testing.T) {
	tests := []struct {
		command  string
		expected []string
	}{
		{
			"Get-Process; Get-Service",
			[]string{"Get-Process", "Get-Service"},
		},
		{
			"Get-ChildItem | Where-Object { $_.Name -eq 'test' }",
			[]string{"Get-ChildItem", "Where-Object { $_.Name -eq 'test' }"},
		},
		{
			"Set-Location /tmp && Get-ChildItem",
			[]string{"Set-Location /tmp", "Get-ChildItem"},
		},
		{
			"Get-Process",
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := splitSubCommands(tt.command)
			if tt.expected == nil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}
			if len(got) != len(tt.expected) {
				t.Errorf("expected %d parts, got %d: %v", len(tt.expected), len(got), got)
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("part[%d] = %q, want %q", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestHasProviderPath(t *testing.T) {
	tests := []struct {
		command string
		has     bool
	}{
		{"Get-ChildItem env:PATH", true},
		{"Set-Item -Path env:MYVAR -Value test", true},
		{"Get-ChildItem HKLM:\\SOFTWARE", true},
		{"Set-Location function:", true},
		{"Write-Output $env:PATH", true},
		{"Get-ChildItem C:\\Windows", false},
		{"cat file.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := hasProviderPath(tt.command)
			if got != tt.has {
				t.Errorf("hasProviderPath(%q) = %v, want %v", tt.command, got, tt.has)
			}
		})
	}
}

func TestClassifyPSCmd(t *testing.T) {
	tests := []struct {
		command    string
		scanResult ScanResult
		cmdType    psCommandType
	}{
		{
			"Get-ChildItem C:\\",
			ScanResult{Level: RiskLevelSafe},
			psCmdReadOnly,
		},
		{
			"Remove-Item /",
			ScanResult{Level: RiskLevelSafe},
			psCmdWrite,
		},
		{
			"Invoke-Expression 'test'",
			ScanResult{Level: RiskLevelDangerous},
			psCmdDangerous,
		},
		{
			"Get-Content file.txt",
			ScanResult{Level: RiskLevelSafe},
			psCmdReadOnly,
		},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := classifyPSCmd(tt.command, tt.scanResult)
			if got != tt.cmdType {
				t.Errorf("classifyPSCmd(%q, %v) = %d, want %d", tt.command, tt.scanResult.Level, got, tt.cmdType)
			}
		})
	}
}

func TestResolveAliases(t *testing.T) {
	tests := []struct {
		input    string
		resolved string
	}{
		{"ls", "get-childitem"},
		{"rm", "remove-item"},
		{"cat", "get-content"},
		{"cd", "set-location"},
		{"echo", "write-output"},
		{"sleep", "start-sleep"},
		{"iex", "invoke-expression"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := resolvePSCommand(tt.input)
			if got != tt.resolved {
				t.Errorf("resolvePSCommand(%q) = %q, want %q", tt.input, got, tt.resolved)
			}
		})
	}
}


func TestValidateFlags(t *testing.T) {
    tests := []struct {
        command   string
        canonical string
        safe      bool
    }{
        {"Get-ChildItem -Recurse", "get-childitem", true},
        {"Get-ChildItem -Depth 3", "get-childitem", true},
        {"Get-ChildItem -Force -Hidden", "get-childitem", true},
        {"Get-Content -Raw file.txt", "get-content", true},
        {"Some-Cmdlet -UnknownFlag", "some-cmdlet", true},
    }
    for _, tt := range tests {
        t.Run(tt.command, func(t *testing.T) {
            got := validateFlags(tt.command, tt.canonical)
            if got != tt.safe {
                t.Errorf("validateFlags(%q, %q) = %v, want %v", tt.command, tt.canonical, got, tt.safe)
            }
        })
    }
}

func TestExtractFlags(t *testing.T) {
    tests := []struct {
        command string
        flags   []string
    }{
        {"Get-ChildItem -Recurse -Depth 3", []string{"recurse", "depth"}},
        {"Get-Content -Raw -Tail 10", []string{"raw", "tail"}},
        {"ls -la", []string{"la"}},
        {"Write-Output hello", []string{}},
    }
    for _, tt := range tests {
        t.Run(tt.command, func(t *testing.T) {
            got := extractFlags(tt.command)
            if len(got) != len(tt.flags) {
                t.Errorf("extractFlags(%q) = %v, want %v", tt.command, got, tt.flags)
                return
            }
            for i := range got {
                if got[i] != tt.flags[i] {
                    t.Errorf("flag[%d] = %q, want %q", i, got[i], tt.flags[i])
                }
            }
        })
    }
}

func TestCheckArgLeaks(t *testing.T) {
    tests := []struct {
        command string
        leak    bool
    }{
        {"Write-Output $env:SECRET", true},
        {"Write-Output hello", false},
        {"Get-ChildItem $env:PATH", false},
        {"Write-Host $var", true},
        {"echo $env:HOME", true},
    }
    for _, tt := range tests {
        t.Run(tt.command, func(t *testing.T) {
            got := checkArgLeaks(tt.command)
            if got != tt.leak {
                t.Errorf("checkArgLeaks(%q) = %v, want %v", tt.command, got, tt.leak)
            }
        })
    }
}

func TestIsDangerousRemoval(t *testing.T) {
    tests := []struct {
        command    string
        dangerous bool
    }{
        {"Remove-Item / -Recurse -Force", true},
        {"rm /etc -rf", true},
        {"Remove-Item C:\\ -Recurse", true},
        {"Remove-Item /home/user/temp -Recurse", false},
        {"Get-ChildItem C:\\", false},
        {"Remove-Item ./temp -Recurse", false},
    }
    for _, tt := range tests {
        t.Run(tt.command, func(t *testing.T) {
            got := isDangerousRemoval(tt.command)
            if got != tt.dangerous {
                t.Errorf("isDangerousRemoval(%q) = %v, want %v", tt.command, got, tt.dangerous)
            }
        })
    }
}

func TestValidateFlagsWithAbbreviation(t *testing.T) {
    if !validateFlags("Get-ChildItem -Rec", "get-childitem") {
        t.Error("expected '-Rec' to be valid abbreviation for '-Recurse'")
    }
    if !validateFlags("Get-ChildItem -Lit C:\\", "get-childitem") {
        t.Error("expected '-Lit' to be valid abbreviation for '-LiteralPath'")
    }
}


func TestCheckGitInternalWrite(t *testing.T) {
    tests := []struct {
        command string
        unsafe  bool
    }{
        {"Set-Content .git/hooks/pre-commit '#!/bin/sh'", true},
        {"New-Item -Path .git/config -ItemType File", true},
        {"Add-Content .git/HEAD 'ref: refs/heads/main'", true},
        {"echo 'data' > .git/hooks/pre-commit", true},
        {"Get-ChildItem .git", false},
        {"git status", false},
        {"Set-Content README.md 'hello'", false},
    }
    for _, tt := range tests {
        t.Run(tt.command, func(t *testing.T) {
            got := checkGitInternalWrite(tt.command)
            if got != tt.unsafe {
                t.Errorf("checkGitInternalWrite(%q) = %v, want %v", tt.command, got, tt.unsafe)
            }
        })
    }
}

func TestCheckBareRepoCompound(t *testing.T) {
    tests := []struct {
        command string
        unsafe  bool
    }{
        {"mkdir hooks; New-Item hooks/pre-commit -ItemType File; git status", true},
        {"New-Item -ItemType Directory refs/heads; git add .", true},
        {"git status", false},
        {"mkdir myproject; git status", false},  // "mkdir myproject" is not a bare-repo path
    }
    for _, tt := range tests {
        t.Run(tt.command, func(t *testing.T) {
            got := checkBareRepoCompound(tt.command)
            if got != tt.unsafe {
                t.Errorf("checkBareRepoCompound(%q) = %v, want %v", tt.command, got, tt.unsafe)
            }
        })
    }
}


func TestIsCwdChangingCmdlet(t *testing.T) {
    tests := []struct {
        command string
        changes bool
    }{
        {"Set-Location /tmp", true},
        {"cd /tmp", true},
        {"pushd /tmp", true},
        {"popd", true},
        {"Push-Location /tmp", true},
        {"Get-ChildItem C:\\", false},
        {"Get-Process", false},
    }
    for _, tt := range tests {
        t.Run(tt.command, func(t *testing.T) {
            got := isCwdChangingCmdlet(tt.command)
            if got != tt.changes {
                t.Errorf("isCwdChangingCmdlet(%q) = %v, want %v", tt.command, got, tt.changes)
            }
        })
    }
}

func TestHasSyncSecurityConcerns(t *testing.T) {
    tests := []struct {
        command  string
        unsafe   bool
    }{
        {"Get-Content $(env:PATH)", true},
        {"Remove-Item @params", true},
        {"[System.IO.File]::ReadAllText('file')", true},
        {"Get-ChildItem C:\\", false},
        {"Write-Output hello", false},
        {"git status", false},
    }
    for _, tt := range tests {
        t.Run(tt.command, func(t *testing.T) {
            got := hasSyncSecurityConcerns(tt.command)
            if got != tt.unsafe {
                t.Errorf("hasSyncSecurityConcerns(%q) = %v, want %v", tt.command, got, tt.unsafe)
            }
        })
    }
}
