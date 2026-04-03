package permission

import "testing"

func TestDecisionValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		decision Decision
		want     bool
	}{
		{name: "allow", decision: DecisionAllow, want: true},
		{name: "ask", decision: DecisionAsk, want: true},
		{name: "deny", decision: DecisionDeny, want: true},
		{name: "unknown", decision: Decision("maybe"), want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.decision.Valid(); got != tt.want {
				t.Fatalf("Decision.Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}
