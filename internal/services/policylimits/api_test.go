package policylimits

import (
	"os"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/featureflag"
)

func TestIsAllowed_FeatureFlagDisabled(t *testing.T) {
	os.Unsetenv("CLAUDE_FEATURE_" + featureflag.FlagPolicyLimits)
	allowed, reason := IsAllowed(ActionAllowRemoteSessions)
	if !allowed {
		t.Error("feature flag disabled should default to allowed")
	}
	if reason != "" {
		t.Error("reason should be empty when feature flag disabled")
	}
}

func TestIsAllowed_FeatureFlagEnabled_NoCache(t *testing.T) {
	os.Setenv("CLAUDE_FEATURE_"+featureflag.FlagPolicyLimits, "1")
	defer os.Unsetenv("CLAUDE_FEATURE_" + featureflag.FlagPolicyLimits)

	_ = ClearCache()
	allowed, reason := IsAllowed(ActionAllowRemoteSessions)
	if !allowed {
		t.Error("no cache should fail open")
	}
	if reason != "" {
		t.Error("reason should be empty when no cache")
	}
}

func TestIsAllowed_ExplicitAllowed(t *testing.T) {
	os.Setenv("CLAUDE_FEATURE_"+featureflag.FlagPolicyLimits, "1")
	defer os.Unsetenv("CLAUDE_FEATURE_" + featureflag.FlagPolicyLimits)

	_ = SaveCache(map[string]Restriction{
		"allow_remote_sessions": {Allowed: true},
	})
	allowed, reason := IsAllowed(ActionAllowRemoteSessions)
	if !allowed {
		t.Error("explicit allowed should return true")
	}
	if reason != "" {
		t.Error("reason should be empty when allowed")
	}
}

func TestIsAllowed_ExplicitDenied(t *testing.T) {
	os.Setenv("CLAUDE_FEATURE_"+featureflag.FlagPolicyLimits, "1")
	defer os.Unsetenv("CLAUDE_FEATURE_" + featureflag.FlagPolicyLimits)

	_ = SaveCache(map[string]Restriction{
		"allow_remote_sessions": {Allowed: false},
	})
	allowed, reason := IsAllowed(ActionAllowRemoteSessions)
	if allowed {
		t.Error("explicit denied should return false")
	}
	if reason == "" {
		t.Error("reason should not be empty when denied")
	}
}

func TestIsAllowed_UnknownPolicy(t *testing.T) {
	os.Setenv("CLAUDE_FEATURE_"+featureflag.FlagPolicyLimits, "1")
	defer os.Unsetenv("CLAUDE_FEATURE_" + featureflag.FlagPolicyLimits)

	_ = SaveCache(map[string]Restriction{
		"allow_remote_sessions": {Allowed: false},
	})
	allowed, reason := IsAllowed(ActionAllowProductFeedback)
	if !allowed {
		t.Error("unknown policy should fail open")
	}
	if reason != "" {
		t.Error("reason should be empty for unknown policy")
	}
}

func TestIsAllowed_EssentialTrafficDenyOnMiss(t *testing.T) {
	os.Setenv("CLAUDE_FEATURE_"+featureflag.FlagPolicyLimits, "1")
	defer os.Unsetenv("CLAUDE_FEATURE_" + featureflag.FlagPolicyLimits)
	_ = ClearCache()

	os.Setenv("CLAUDE_CODE_ESSENTIAL_TRAFFIC_ONLY", "1")
	defer os.Unsetenv("CLAUDE_CODE_ESSENTIAL_TRAFFIC_ONLY")

	allowed, reason := IsAllowed(ActionAllowProductFeedback)
	if allowed {
		t.Error("essential-traffic-only + deny-on-miss should deny")
	}
	if reason == "" {
		t.Error("reason should explain denial")
	}
}

func TestIsAllowed_EssentialTrafficNotInDenyList(t *testing.T) {
	os.Setenv("CLAUDE_FEATURE_"+featureflag.FlagPolicyLimits, "1")
	defer os.Unsetenv("CLAUDE_FEATURE_" + featureflag.FlagPolicyLimits)
	_ = ClearCache()

	os.Setenv("CLAUDE_CODE_ESSENTIAL_TRAFFIC_ONLY", "1")
	defer os.Unsetenv("CLAUDE_CODE_ESSENTIAL_TRAFFIC_ONLY")

	allowed, _ := IsAllowed(ActionAllowRemoteSessions)
	if !allowed {
		t.Error("essential-traffic-only policy not in deny list should allow")
	}
}

func TestIsAllowedString(t *testing.T) {
	os.Setenv("CLAUDE_FEATURE_"+featureflag.FlagPolicyLimits, "1")
	defer os.Unsetenv("CLAUDE_FEATURE_" + featureflag.FlagPolicyLimits)

	_ = SaveCache(map[string]Restriction{
		"allow_remote_sessions": {Allowed: true},
	})
	allowed, _ := IsAllowedString("allow_remote_sessions")
	if !allowed {
		t.Error("IsAllowedString should work for known policy")
	}
}
