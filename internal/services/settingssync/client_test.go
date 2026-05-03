package settingssync

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchUserSettingsGET_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]interface{}{
			"userId":       "user-1",
			"version":      1,
			"lastModified": "2026-01-01",
			"checksum":     "abc",
			"content": map[string]interface{}{
				"entries": map[string]string{
					KeyUserSettings: `{"theme":"dark"}`,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	result, err := fetchUserSettingsGET(t.Context(), srv.URL, "test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if result.IsEmpty {
		t.Error("expected non-empty result")
	}
	if result.Data.UserID != "user-1" {
		t.Errorf("UserID: got %q", result.Data.UserID)
	}
	if result.Data.Version != 1 {
		t.Errorf("Version: got %d", result.Data.Version)
	}
	if len(result.Data.Content.Entries) != 1 {
		t.Errorf("entries count: got %d", len(result.Data.Content.Entries))
	}
}

func TestFetchUserSettingsGET_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	result, err := fetchUserSettingsGET(t.Context(), srv.URL, "test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("404 should return success with IsEmpty=true")
	}
	if !result.IsEmpty {
		t.Error("404 should mark IsEmpty=true")
	}
}

func TestFetchUserSettingsGET_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	result, err := fetchUserSettingsGET(t.Context(), srv.URL, "test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("401 should not be success")
	}
	if !result.SkipRetry {
		t.Error("401 should skip retry")
	}
}

func TestFetchUserSettingsGET_Forbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	result, err := fetchUserSettingsGET(t.Context(), srv.URL, "test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("403 should not be success")
	}
	if !result.SkipRetry {
		t.Error("403 should skip retry")
	}
}

func TestFetchUserSettingsGET_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	result, err := fetchUserSettingsGET(t.Context(), srv.URL, "test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("500 should not be success")
	}
	if result.SkipRetry {
		t.Error("500 should be retryable")
	}
}

func TestFetchUserSettingsGET_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not-json"))
	}))
	defer srv.Close()

	result, err := fetchUserSettingsGET(t.Context(), srv.URL, "test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("invalid JSON should not be success")
	}
}

func TestUploadUserSettings_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("expected Content-Type: application/json")
		}
		resp := map[string]interface{}{
			"checksum":     "new-checksum",
			"lastModified": "2026-05-03",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	entries := map[string]string{
		KeyUserSettings: `{"theme":"dark"}`,
	}

	result, err := UploadUserSettings(t.Context(), srv.URL, "test-token", entries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected upload success")
	}
	if result.Checksum != "new-checksum" {
		t.Errorf("Checksum: got %q", result.Checksum)
	}
	if result.LastModified != "2026-05-03" {
		t.Errorf("LastModified: got %q", result.LastModified)
	}
}

func TestPickDiff_OnlyChanged(t *testing.T) {
	local := map[string]string{
		"a": "1",
		"b": "2-new",
		"c": "3",
	}
	remote := map[string]string{
		"a": "1",
		"b": "2-old",
	}

	diff := pickDiff(local, remote)
	if len(diff) != 2 {
		t.Fatalf("expected 2 changed entries, got %d", len(diff))
	}
	if diff["b"] != "2-new" {
		t.Errorf("b: got %q", diff["b"])
	}
	if diff["c"] != "3" {
		t.Errorf("c (missing in remote): got %q", diff["c"])
	}
}

func TestPickDiff_NoChanges(t *testing.T) {
	local := map[string]string{"a": "1", "b": "2"}
	remote := map[string]string{"a": "1", "b": "2"}

	diff := pickDiff(local, remote)
	if len(diff) != 0 {
		t.Fatalf("expected 0 changes, got %d", len(diff))
	}
}

func TestPickDiff_EmptyRemote(t *testing.T) {
	local := map[string]string{"a": "1", "b": "2"}

	diff := pickDiff(local, nil)
	if len(diff) != 2 {
		t.Fatalf("expected 2 (all) changes, got %d", len(diff))
	}
}

func TestPickDiff_EmptyLocal(t *testing.T) {
	remote := map[string]string{"a": "1"}

	diff := pickDiff(nil, remote)
	if len(diff) != 0 {
		t.Fatalf("expected 0 changes, got %d", len(diff))
	}
}

func TestExponentialBackoff_FirstAttempt(t *testing.T) {
	d := exponentialBackoff(0)
	if d < baseDelay || d > baseDelay+time.Second {
		t.Errorf("backoff(0) out of expected range: %v", d)
	}
}

func TestExponentialBackoff_CapsAtMax(t *testing.T) {
	d := exponentialBackoff(20) // 2^20 = very large → capped
	if d > maxDelay {
		t.Errorf("backoff should cap at maxDelay: got %v", d)
	}
}

var errTestTimeout = errors.New("context deadline exceeded")

func TestClassifyHTTPErr_Timeout(t *testing.T) {
	if classifyHTTPErr(errTestTimeout) != "timeout" {
		t.Error("timeout error should classify as timeout")
	}
}
