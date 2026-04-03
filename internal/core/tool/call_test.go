package tool

import (
	"testing"
	"time"
)

// TestReadStateSnapshotLookup verifies that the minimal read snapshot can expose read metadata by path.
func TestReadStateSnapshotLookup(t *testing.T) {
	readAt := time.Date(2026, time.April, 3, 10, 0, 0, 0, time.UTC)
	snapshot := ReadStateSnapshot{
		Files: map[string]ReadState{
			"/tmp/project/main.go": {
				ReadAt:    readAt,
				IsPartial: true,
			},
		},
	}

	state, ok := snapshot.Lookup("/tmp/project/main.go")
	if !ok {
		t.Fatal("Lookup() ok = false, want true")
	}
	if !state.ReadAt.Equal(readAt) {
		t.Fatalf("Lookup() ReadAt = %v, want %v", state.ReadAt, readAt)
	}
	if !state.IsPartial {
		t.Fatal("Lookup() IsPartial = false, want true")
	}
}

// TestReadStateSnapshotLookupMissing verifies nil-safe lookup behavior for absent paths.
func TestReadStateSnapshotLookupMissing(t *testing.T) {
	var snapshot ReadStateSnapshot

	state, ok := snapshot.Lookup("/tmp/project/missing.go")
	if ok {
		t.Fatal("Lookup() ok = true, want false")
	}
	if !state.ReadAt.IsZero() {
		t.Fatalf("Lookup() ReadAt = %v, want zero time", state.ReadAt)
	}
	if state.IsPartial {
		t.Fatal("Lookup() IsPartial = true, want false")
	}
}

// TestUseContextLookupReadState verifies that tool invocations can query the shared read snapshot through UseContext.
func TestUseContextLookupReadState(t *testing.T) {
	readAt := time.Date(2026, time.April, 3, 11, 0, 0, 0, time.UTC)
	context := UseContext{
		WorkingDir: "/tmp/project",
		Invoker:    "test",
		ReadState: ReadStateSnapshot{
			Files: map[string]ReadState{
				"/tmp/project/app.go": {
					ReadAt:    readAt,
					IsPartial: false,
				},
			},
		},
	}

	state, ok := context.LookupReadState("/tmp/project/app.go")
	if !ok {
		t.Fatal("LookupReadState() ok = false, want true")
	}
	if !state.ReadAt.Equal(readAt) {
		t.Fatalf("LookupReadState() ReadAt = %v, want %v", state.ReadAt, readAt)
	}
	if state.IsPartial {
		t.Fatal("LookupReadState() IsPartial = true, want false")
	}
}
