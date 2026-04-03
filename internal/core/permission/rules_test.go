package permission

import "testing"

func TestRuleValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		rule    Rule
		wantErr bool
	}{
		{
			name: "valid rule",
			rule: Rule{
				Source:   RuleSourceSession,
				Decision: DecisionAllow,
				BaseDir:  "/workspace",
				Pattern:  "src/**/*.go",
			},
		},
		{
			name: "missing pattern",
			rule: Rule{
				Source:   RuleSourceSession,
				Decision: DecisionAllow,
			},
			wantErr: true,
		},
		{
			name: "invalid source",
			rule: Rule{
				Source:   RuleSource("unknown"),
				Decision: DecisionAllow,
				Pattern:  "**",
			},
			wantErr: true,
		},
		{
			name: "invalid decision",
			rule: Rule{
				Source:   RuleSourceSession,
				Decision: Decision("unknown"),
				Pattern:  "**",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.rule.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Rule.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRuleSetValidate(t *testing.T) {
	t.Parallel()

	rules := RuleSet{
		Read: []Rule{
			{
				Source:   RuleSourceProjectSettings,
				Decision: DecisionAllow,
				BaseDir:  "/workspace",
				Pattern:  "src/**",
			},
		},
		Write: []Rule{
			{
				Source:   RuleSourceSession,
				Decision: DecisionAsk,
				BaseDir:  "/workspace",
				Pattern:  ".claude/**",
			},
		},
	}

	if err := rules.Validate(); err != nil {
		t.Fatalf("RuleSet.Validate() error = %v", err)
	}
}
