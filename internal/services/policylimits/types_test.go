package policylimits

import (
	"encoding/json"
	"testing"
)

func TestPolicyLimitsResponse_JSON(t *testing.T) {
	resp := PolicyLimitsResponse{
		Restrictions: map[string]Restriction{
			"allow_remote_sessions":  {Allowed: false},
			"allow_product_feedback": {Allowed: true},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded PolicyLimitsResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(decoded.Restrictions) != 2 {
		t.Fatalf("expected 2 restrictions, got %d", len(decoded.Restrictions))
	}
	if decoded.Restrictions["allow_remote_sessions"].Allowed {
		t.Error("allow_remote_sessions should be false")
	}
	if !decoded.Restrictions["allow_product_feedback"].Allowed {
		t.Error("allow_product_feedback should be true")
	}
}

func TestPolicyAction_String(t *testing.T) {
	if string(ActionAllowRemoteSessions) != "allow_remote_sessions" {
		t.Error("ActionAllowRemoteSessions mismatch")
	}
	if string(ActionAllowProductFeedback) != "allow_product_feedback" {
		t.Error("ActionAllowProductFeedback mismatch")
	}
}
