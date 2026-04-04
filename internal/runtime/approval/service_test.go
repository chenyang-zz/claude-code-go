package approval

import (
	"context"
	"testing"
)

// TestSupportedModesIncludesDefault verifies the migrated approval mode allowlist stays aligned with the settings schema subset.
func TestSupportedModesIncludesDefault(t *testing.T) {
	if !IsSupportedMode(ModeDefault) {
		t.Fatalf("IsSupportedMode(%q) = false, want true", ModeDefault)
	}
	if IsSupportedMode("auto") {
		t.Fatal(`IsSupportedMode("auto") = true, want false`)
	}
}

// TestStaticServiceDecideReturnsConfiguredResponse verifies the early-stage approval service can be injected into runtime tests deterministically.
func TestStaticServiceDecideReturnsConfiguredResponse(t *testing.T) {
	service := StaticService{
		Response: Response{
			Approved: true,
		},
	}

	resp, err := service.Decide(context.Background(), Request{
		CallID:   "toolu_1",
		ToolName: "Read",
		Path:     "/tmp/demo.txt",
		Action:   "read",
		Message:  "Claude requested permissions to read from /tmp/demo.txt, but you haven't granted it yet.",
	})
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if !resp.Approved {
		t.Fatalf("Decide() = %#v, want approved response", resp)
	}
}
