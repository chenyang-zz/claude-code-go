package console

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/runtime/approval"
)

// TestApprovalRendererPromptApproves verifies the CLI approval renderer accepts a yes-style response.
func TestApprovalRendererPromptApproves(t *testing.T) {
	var out bytes.Buffer
	renderer := NewApprovalRenderer(NewPrinter(&out), strings.NewReader("y\n"))

	resp, err := renderer.Prompt(context.Background(), approval.Prompt{
		Title: "Read wants to read",
		Body:  "Claude requested permissions to read from /tmp/demo.txt, but you haven't granted it yet.",
	})
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	if !resp.Approved {
		t.Fatalf("Prompt() = %#v, want approved response", resp)
	}
	if !strings.Contains(out.String(), "Approve? [y/N]: ") {
		t.Fatalf("Prompt() output = %q, want approval question", out.String())
	}
}
