package tool

import (
	"testing"
	"time"
)

// TestReadStateSnapshotLookup verifies that the minimal read snapshot can expose read metadata by path.
func TestReadStateSnapshotLookup(t *testing.T) {
	readAt := time.Date(2026, time.April, 3, 10, 0, 0, 0, time.UTC)
	modTime := time.Date(2026, time.April, 3, 9, 58, 0, 0, time.UTC)
	snapshot := ReadStateSnapshot{
		Files: map[string]ReadState{
			"/tmp/project/main.go": {
				ReadAt:          readAt,
				ObservedModTime: modTime,
				ReadOffset:      10,
				ReadLimit:       20,
				IsPartial:       true,
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
	if !state.ObservedModTime.Equal(modTime) {
		t.Fatalf("Lookup() ObservedModTime = %v, want %v", state.ObservedModTime, modTime)
	}
	if state.ReadOffset != 10 {
		t.Fatalf("Lookup() ReadOffset = %d, want %d", state.ReadOffset, 10)
	}
	if state.ReadLimit != 20 {
		t.Fatalf("Lookup() ReadLimit = %d, want %d", state.ReadLimit, 20)
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
	if !state.ObservedModTime.IsZero() {
		t.Fatalf("Lookup() ObservedModTime = %v, want zero time", state.ObservedModTime)
	}
	if state.IsPartial {
		t.Fatal("Lookup() IsPartial = true, want false")
	}
}

// TestUseContextLookupReadState verifies that tool invocations can query the shared read snapshot through UseContext.
func TestUseContextLookupReadState(t *testing.T) {
	readAt := time.Date(2026, time.April, 3, 11, 0, 0, 0, time.UTC)
	modTime := time.Date(2026, time.April, 3, 10, 59, 0, 0, time.UTC)
	context := UseContext{
		WorkingDir: "/tmp/project",
		Invoker:    "test",
		ReadState: ReadStateSnapshot{
			Files: map[string]ReadState{
				"/tmp/project/app.go": {
					ReadAt:          readAt,
					ObservedModTime: modTime,
					ReadOffset:      1,
					ReadLimit:       0,
					IsPartial:       false,
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
	if !state.ObservedModTime.Equal(modTime) {
		t.Fatalf("LookupReadState() ObservedModTime = %v, want %v", state.ObservedModTime, modTime)
	}
	if state.ReadOffset != 1 {
		t.Fatalf("LookupReadState() ReadOffset = %d, want %d", state.ReadOffset, 1)
	}
	if state.ReadLimit != 0 {
		t.Fatalf("LookupReadState() ReadLimit = %d, want %d", state.ReadLimit, 0)
	}
	if state.IsPartial {
		t.Fatal("LookupReadState() IsPartial = true, want false")
	}
}

// TestReadStateSnapshotCloneAndMerge verifies the executor can safely copy and overlay read-state snapshots.
func TestReadStateSnapshotCloneAndMerge(t *testing.T) {
	base := ReadStateSnapshot{
		Files: map[string]ReadState{
			"/tmp/project/base.go": {
				ReadAt:          time.Date(2026, time.April, 3, 12, 0, 0, 0, time.UTC),
				ObservedModTime: time.Date(2026, time.April, 3, 11, 59, 0, 0, time.UTC),
				ReadOffset:      1,
			},
		},
	}

	cloned := base.Clone()
	cloned.Merge(ReadStateSnapshot{
		Files: map[string]ReadState{
			"/tmp/project/other.go": {
				ReadAt:          time.Date(2026, time.April, 3, 12, 1, 0, 0, time.UTC),
				ObservedModTime: time.Date(2026, time.April, 3, 12, 0, 30, 0, time.UTC),
				ReadOffset:      20,
				ReadLimit:       5,
				IsPartial:       true,
			},
		},
	})

	if _, ok := base.Lookup("/tmp/project/other.go"); ok {
		t.Fatal("base snapshot mutated after Clone()+Merge()")
	}
	state, ok := cloned.Lookup("/tmp/project/other.go")
	if !ok {
		t.Fatal("Clone()+Merge() missing merged path")
	}
	if !state.IsPartial {
		t.Fatal("Clone()+Merge() IsPartial = false, want true")
	}
	if state.ReadOffset != 20 {
		t.Fatalf("Clone()+Merge() ReadOffset = %d, want %d", state.ReadOffset, 20)
	}
	if state.ReadLimit != 5 {
		t.Fatalf("Clone()+Merge() ReadLimit = %d, want %d", state.ReadLimit, 5)
	}
}
