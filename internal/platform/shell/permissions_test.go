package shell

import (
	"testing"

	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
)

// TestPermissionCheckerCheck verifies exact, prefix, wildcard, and default Bash permission outcomes.
func TestPermissionCheckerCheck(t *testing.T) {
	checker := NewPermissionChecker(coreconfig.PermissionConfig{
		Allow: []string{"Read(*)", "Bash(ls:*)"},
		Deny:  []string{"Bash(rm -rf /)"},
		Ask:   []string{"Bash(git push:*)"},
	})

	tests := []struct {
		name     string
		command  string
		want     corepermission.Decision
		wantRule string
	}{
		{
			name:     "allow with env and wrapper normalization",
			command:  "FOO=bar timeout 5 ls -la",
			want:     corepermission.DecisionAllow,
			wantRule: "Bash(ls:*)",
		},
		{
			name:     "deny exact match",
			command:  "rm -rf /",
			want:     corepermission.DecisionDeny,
			wantRule: "Bash(rm -rf /)",
		},
		{
			name:     "ask matched rule",
			command:  "git push origin main",
			want:     corepermission.DecisionAsk,
			wantRule: "Bash(git push:*)",
		},
		{
			name:    "ask by default when unmatched",
			command: "pwd",
			want:    corepermission.DecisionAsk,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checker.Check(tt.command)
			if got.Decision != tt.want {
				t.Fatalf("Check(%q) decision = %q, want %q", tt.command, got.Decision, tt.want)
			}
			if got.Rule != tt.wantRule {
				t.Fatalf("Check(%q) rule = %q, want %q", tt.command, got.Rule, tt.wantRule)
			}
		})
	}
}
