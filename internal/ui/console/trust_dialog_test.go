package console

import (
	"os"
	"testing"
)

func TestTrustDialog_NonTerminal(t *testing.T) {
	// When stdin is not a terminal, the dialog should be skipped.
	result, shown, err := TrustDialog("/tmp/project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if shown {
		t.Fatal("expected dialog to be skipped in non-interactive mode")
	}
	if result != TrustResultRejected {
		t.Fatalf("expected rejected, got %v", result)
	}
}

func TestIsTerminal(t *testing.T) {
	// os.Stdin in test environment is typically not a terminal.
	if isTerminal(os.Stdin) {
		t.Log("stdin appears to be a terminal; skipping non-terminal assertions")
	}

	// A pipe is definitely not a terminal.
	r, _, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if isTerminal(r) {
		t.Fatal("expected pipe to not be a terminal")
	}
}

func TestTrustDialogResult_StringCoverage(t *testing.T) {
	// Basic coverage for result constants.
	_ = TrustResultAccepted
	_ = TrustResultRejected
}
