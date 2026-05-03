package settingssync

import (
	"encoding/json"
	"testing"
)

func TestUserSyncData_JSON_RoundTrip(t *testing.T) {
	data := UserSyncData{
		UserID:       "user-123",
		Version:      5,
		LastModified: "2026-05-03T10:00:00Z",
		Checksum:     "abc123",
		Content: UserSyncContent{
			Entries: map[string]string{
				KeyUserSettings: `{"theme":"dark"}`,
				KeyUserMemory:   "Remember to use Go.",
			},
		},
	}

	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded UserSyncData
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.UserID != "user-123" {
		t.Errorf("UserID: got %q, want %q", decoded.UserID, "user-123")
	}
	if decoded.Version != 5 {
		t.Errorf("Version: got %d, want 5", decoded.Version)
	}
	if decoded.Checksum != "abc123" {
		t.Errorf("Checksum: got %q, want %q", decoded.Checksum, "abc123")
	}
	if len(decoded.Content.Entries) != 2 {
		t.Fatalf("entries count: got %d, want 2", len(decoded.Content.Entries))
	}
	if decoded.Content.Entries[KeyUserSettings] != `{"theme":"dark"}` {
		t.Error("user settings entry mismatch")
	}
	if decoded.Content.Entries[KeyUserMemory] != "Remember to use Go." {
		t.Error("user memory entry mismatch")
	}
}

func TestUserSyncData_EmptyEntries(t *testing.T) {
	data := UserSyncData{
		UserID: "empty-user",
		Content: UserSyncContent{
			Entries: map[string]string{},
		},
	}

	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded UserSyncData
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(decoded.Content.Entries) != 0 {
		t.Errorf("expected empty entries, got %d", len(decoded.Content.Entries))
	}
}

func TestSettingsSyncFetchResult_Success(t *testing.T) {
	r := SettingsSyncFetchResult{Success: true}
	if !r.Success {
		t.Error("expected success")
	}
}

func TestSettingsSyncFetchResult_IsEmpty(t *testing.T) {
	r := SettingsSyncFetchResult{Success: true, IsEmpty: true}
	if !r.IsEmpty {
		t.Error("expected IsEmpty=true")
	}
}

func TestSettingsSyncFetchResult_SkipRetry(t *testing.T) {
	r := SettingsSyncFetchResult{Success: false, SkipRetry: true, Error: "auth error"}
	if r.Success {
		t.Error("expected failure")
	}
	if !r.SkipRetry {
		t.Error("expected SkipRetry=true")
	}
}

func TestSettingsSyncUploadResult(t *testing.T) {
	r := SettingsSyncUploadResult{
		Success:      true,
		Checksum:     "xyz789",
		LastModified: "2026-05-03",
	}
	if !r.Success {
		t.Error("expected success")
	}
	if r.Checksum != "xyz789" {
		t.Errorf("Checksum: got %q", r.Checksum)
	}
}

func TestSyncKeyConstants(t *testing.T) {
	if KeyUserSettings != "~/.claude/settings.json" {
		t.Errorf("KeyUserSettings: got %q", KeyUserSettings)
	}
	if KeyUserMemory != "~/.claude/CLAUDE.md" {
		t.Errorf("KeyUserMemory: got %q", KeyUserMemory)
	}
}

func TestProjectSettingsKey(t *testing.T) {
	got := ProjectSettingsKey("org/repo")
	want := "projects/org/repo/.claude/settings.local.json"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestProjectMemoryKey(t *testing.T) {
	got := ProjectMemoryKey("org/repo")
	want := "projects/org/repo/CLAUDE.local.md"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
