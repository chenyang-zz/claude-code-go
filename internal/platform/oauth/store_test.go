package oauth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func newTestStore(t *testing.T) (*OAuthCredentialStore, string) {
	t.Helper()
	dir := t.TempDir()
	store, err := NewOAuthCredentialStore(dir)
	if err != nil {
		t.Fatalf("NewOAuthCredentialStore: %v", err)
	}
	return store, dir
}

func TestNewOAuthCredentialStore_RejectsEmptyHomeDir(t *testing.T) {
	if _, err := NewOAuthCredentialStore(""); err == nil {
		t.Fatalf("expected error for empty homeDir")
	}
}

func TestOAuthCredentialStore_PathLayout(t *testing.T) {
	store, dir := newTestStore(t)
	want := filepath.Join(dir, ".claude", ".credentials.json")
	if got := store.Path(); got != want {
		t.Fatalf("Path = %q, want %q", got, want)
	}
}

func TestOAuthCredentialStore_LoadMissingFile(t *testing.T) {
	store, _ := newTestStore(t)
	tokens, err := store.Load()
	if err != nil {
		t.Fatalf("Load on missing file should succeed, got %v", err)
	}
	if tokens != nil {
		t.Fatalf("Load on missing file should return nil, got %+v", tokens)
	}
}

func TestOAuthCredentialStore_RoundTrip(t *testing.T) {
	store, _ := newTestStore(t)
	original := &OAuthTokens{
		AccessToken:      "at-1",
		RefreshToken:     "rt-1",
		ExpiresAt:        1700000000000,
		Scopes:           []string{ScopeUserProfile, ScopeUserInference},
		SubscriptionType: SubscriptionMax,
		RateLimitTier:    RateLimitTier("default_claude_max_5x"),
	}
	dec, err := store.Save(original)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if !dec.Persisted {
		t.Fatalf("Save should persist tokens with refresh+expiry+inference scope, got %+v", dec)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded == nil {
		t.Fatalf("Load returned nil tokens after Save")
	}
	if loaded.AccessToken != original.AccessToken ||
		loaded.RefreshToken != original.RefreshToken ||
		loaded.ExpiresAt != original.ExpiresAt ||
		loaded.SubscriptionType != original.SubscriptionType ||
		loaded.RateLimitTier != original.RateLimitTier {
		t.Fatalf("loaded tokens diverge from original\nwant=%+v\ngot=%+v", original, loaded)
	}
	if !reflect.DeepEqual(loaded.Scopes, original.Scopes) {
		t.Fatalf("scopes mismatch\nwant=%v\ngot=%v", original.Scopes, loaded.Scopes)
	}
}

func TestOAuthCredentialStore_FilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits not enforced on Windows")
	}
	store, _ := newTestStore(t)
	tokens := &OAuthTokens{
		AccessToken:  "at",
		RefreshToken: "rt",
		ExpiresAt:    1,
		Scopes:       []string{ScopeUserInference},
	}
	if _, err := store.Save(tokens); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(store.Path())
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != CredentialsFileMode {
		t.Fatalf("file perm = %o, want %o", perm, CredentialsFileMode)
	}
}

func TestOAuthCredentialStore_SaveSkipsScopeWithoutInference(t *testing.T) {
	store, _ := newTestStore(t)
	tokens := &OAuthTokens{
		AccessToken:  "at",
		RefreshToken: "rt",
		ExpiresAt:    1,
		Scopes:       []string{ScopeUserProfile},
	}
	dec, err := store.Save(tokens)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if dec.Persisted {
		t.Fatalf("Save should skip without user:inference, got Persisted=true")
	}
	if !strings.Contains(dec.SkipReason, "user:inference") {
		t.Fatalf("SkipReason = %q", dec.SkipReason)
	}
	if _, err := os.Stat(store.Path()); !os.IsNotExist(err) {
		t.Fatalf("expected file not to exist when Save was skipped, stat err=%v", err)
	}
}

func TestOAuthCredentialStore_SaveSkipsMissingRefreshToken(t *testing.T) {
	store, _ := newTestStore(t)
	tokens := &OAuthTokens{
		AccessToken: "at",
		ExpiresAt:   1,
		Scopes:      []string{ScopeUserInference},
	}
	dec, err := store.Save(tokens)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if dec.Persisted {
		t.Fatalf("Save should skip without refresh+expiry, got Persisted=true")
	}
}

func TestOAuthCredentialStore_PreservesOtherFields(t *testing.T) {
	store, dir := newTestStore(t)
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	preExisting := []byte(`{
		"otherTool": {"foo": "bar"},
		"claudeAiOauth": {
			"accessToken": "old-at",
			"refreshToken": "old-rt",
			"expiresAt": 1,
			"scopes": ["user:inference"]
		}
	}`)
	if err := os.WriteFile(store.Path(), preExisting, 0o600); err != nil {
		t.Fatalf("write seed: %v", err)
	}

	tokens := &OAuthTokens{
		AccessToken:  "new-at",
		RefreshToken: "new-rt",
		ExpiresAt:    2,
		Scopes:       []string{ScopeUserInference},
	}
	if _, err := store.Save(tokens); err != nil {
		t.Fatalf("Save: %v", err)
	}

	raw, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatalf("read after save: %v", err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode after save: %v", err)
	}
	if _, ok := doc["otherTool"]; !ok {
		t.Fatalf("Save dropped sibling field otherTool: %s", string(raw))
	}
}

func TestOAuthCredentialStore_PreservesSubscriptionAndTierOnRefresh(t *testing.T) {
	store, dir := newTestStore(t)
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	preExisting := []byte(`{
		"claudeAiOauth": {
			"accessToken": "old-at",
			"refreshToken": "old-rt",
			"expiresAt": 1,
			"scopes": ["user:inference"],
			"subscriptionType": "max",
			"rateLimitTier": "default_claude_max_5x"
		}
	}`)
	if err := os.WriteFile(store.Path(), preExisting, 0o600); err != nil {
		t.Fatalf("write seed: %v", err)
	}

	// New tokens missing subscriptionType / rateLimitTier (e.g. profile fetch
	// failed during a refresh) should retain the old values.
	refreshed := &OAuthTokens{
		AccessToken:  "new-at",
		RefreshToken: "new-rt",
		ExpiresAt:    2,
		Scopes:       []string{ScopeUserInference},
	}
	if _, err := store.Save(refreshed); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.SubscriptionType != SubscriptionMax {
		t.Fatalf("SubscriptionType lost: got %q, want %q", loaded.SubscriptionType, SubscriptionMax)
	}
	if string(loaded.RateLimitTier) != "default_claude_max_5x" {
		t.Fatalf("RateLimitTier lost: got %q", loaded.RateLimitTier)
	}
}

func TestOAuthCredentialStore_LoadCorruptedFile(t *testing.T) {
	store, dir := newTestStore(t)
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(store.Path(), []byte("{not valid json"), 0o600); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}
	_, err := store.Load()
	if err == nil {
		t.Fatalf("expected decode error for corrupt file")
	}
	if !strings.Contains(err.Error(), "decode") {
		t.Fatalf("expected decode error message, got %v", err)
	}
}

func TestOAuthCredentialStore_DeleteWithSiblingFieldsKeepsFile(t *testing.T) {
	store, dir := newTestStore(t)
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	preExisting := []byte(`{
		"otherTool": {"foo": "bar"},
		"claudeAiOauth": {"accessToken": "at"}
	}`)
	if err := os.WriteFile(store.Path(), preExisting, 0o600); err != nil {
		t.Fatalf("write seed: %v", err)
	}

	if err := store.Delete(); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	raw, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatalf("read after delete: %v", err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode after delete: %v", err)
	}
	if _, ok := doc["claudeAiOauth"]; ok {
		t.Fatalf("claudeAiOauth was not removed: %s", string(raw))
	}
	if _, ok := doc["otherTool"]; !ok {
		t.Fatalf("sibling field dropped after delete: %s", string(raw))
	}
}

func TestOAuthCredentialStore_DeleteOnlyClaudeAIDeletesFile(t *testing.T) {
	store, _ := newTestStore(t)
	tokens := &OAuthTokens{
		AccessToken:  "at",
		RefreshToken: "rt",
		ExpiresAt:    1,
		Scopes:       []string{ScopeUserInference},
	}
	if _, err := store.Save(tokens); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := store.Delete(); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(store.Path()); !os.IsNotExist(err) {
		t.Fatalf("file should be removed when only claudeAiOauth was present, stat err=%v", err)
	}
}

func TestOAuthCredentialStore_DeleteMissingFile(t *testing.T) {
	store, _ := newTestStore(t)
	if err := store.Delete(); err != nil {
		t.Fatalf("Delete on missing file should succeed, got %v", err)
	}
}

func TestOAuthCredentialStore_SaveNilTokensFails(t *testing.T) {
	store, _ := newTestStore(t)
	if _, err := store.Save(nil); err == nil {
		t.Fatalf("expected error for nil tokens")
	}
}
