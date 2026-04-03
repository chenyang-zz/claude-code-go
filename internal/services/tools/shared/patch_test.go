package shared

import "testing"

// TestBuildStructuredPatchReturnsNilWithoutChanges verifies unchanged content does not emit a patch payload.
func TestBuildStructuredPatchReturnsNilWithoutChanges(t *testing.T) {
	patch := BuildStructuredPatch("alpha\nbeta\n", "alpha\nbeta\n")
	if patch != nil {
		t.Fatalf("BuildStructuredPatch() = %#v, want nil", patch)
	}
}

// TestBuildStructuredPatchHandlesCreate verifies new-file writes become an add-only hunk.
func TestBuildStructuredPatchHandlesCreate(t *testing.T) {
	patch := BuildStructuredPatch("", "alpha\nbeta\n")
	if len(patch) != 1 {
		t.Fatalf("len(BuildStructuredPatch()) = %d, want 1", len(patch))
	}

	hunk := patch[0]
	if hunk.OldStart != 1 || hunk.OldLines != 0 || hunk.NewStart != 1 || hunk.NewLines != 2 {
		t.Fatalf("BuildStructuredPatch() hunk = %#v", hunk)
	}

	wantLines := []string{"+alpha", "+beta"}
	assertLinesEqual(t, hunk.Lines, wantLines)
}

// TestBuildStructuredPatchHandlesInPlaceReplacement verifies shared patch generation trims unchanged prefix and suffix lines.
func TestBuildStructuredPatchHandlesInPlaceReplacement(t *testing.T) {
	patch := BuildStructuredPatch("a\nbefore\nz\n", "a\nafter\nz\n")
	if len(patch) != 1 {
		t.Fatalf("len(BuildStructuredPatch()) = %d, want 1", len(patch))
	}

	hunk := patch[0]
	if hunk.OldStart != 2 || hunk.OldLines != 1 || hunk.NewStart != 2 || hunk.NewLines != 1 {
		t.Fatalf("BuildStructuredPatch() hunk = %#v", hunk)
	}

	wantLines := []string{"-before", "+after"}
	assertLinesEqual(t, hunk.Lines, wantLines)
}

// assertLinesEqual keeps line-oriented diff expectations terse in shared tests.
func assertLinesEqual(t *testing.T, got []string, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("len(lines) = %d, want %d; got = %#v", len(got), len(want), got)
	}

	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("lines[%d] = %q, want %q", index, got[index], want[index])
		}
	}
}
