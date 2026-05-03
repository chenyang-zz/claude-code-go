package teammemsync

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ─── Secret Scanner Tests ─────────────────────────────────────────

func TestScanForSecrets(t *testing.T) {
	t.Run("detects aws access token", func(t *testing.T) {
		content := "AKIAIOSFODNN7EXAMPLE"
		matches := ScanForSecrets(content)
		if len(matches) != 1 {
			t.Fatalf("expected 1 match, got %d", len(matches))
		}
		if matches[0].RuleID != "aws-access-token" {
			t.Errorf("expected rule aws-access-token, got %q", matches[0].RuleID)
		}
	})

	t.Run("detects github pat", func(t *testing.T) {
		content := "ghp_123456789012345678901234567890123456"
		matches := ScanForSecrets(content)
		if len(matches) != 1 {
			t.Fatalf("expected 1 match, got %d", len(matches))
		}
		if matches[0].RuleID != "github-pat" {
			t.Errorf("expected rule github-pat, got %q", matches[0].RuleID)
		}
	})

	t.Run("detects anthropic api key", func(t *testing.T) {
		// sk-ant-api03- + 93 alphanumeric/dash/underscore chars + AA
		content := "sk-ant-api03-" + strings.Repeat("a", 93) + "AA"
		matches := ScanForSecrets(content)
		if len(matches) != 1 {
			t.Fatalf("expected 1 match, got %d", len(matches))
		}
		if matches[0].RuleID != "anthropic-api-key" {
			t.Errorf("expected rule anthropic-api-key, got %q", matches[0].RuleID)
		}
	})

	t.Run("detects slack bot token", func(t *testing.T) {
		content := "xoxb-1234567890-1234567890-abcdefg"
		matches := ScanForSecrets(content)
		if len(matches) != 1 {
			t.Fatalf("expected 1 match, got %d", len(matches))
		}
		if matches[0].RuleID != "slack-bot-token" {
			t.Errorf("expected rule slack-bot-token, got %q", matches[0].RuleID)
		}
	})

	t.Run("detects private key", func(t *testing.T) {
		content := `-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQDa+TrACQqjS0E8
Q8mDL8x8bEZi0k7L7e/4gP3pT2GmG8z9OcFvM0d8V8K5qYQ1Xve7W4K84eYm
-----END PRIVATE KEY-----`
		matches := ScanForSecrets(content)
		if len(matches) != 1 {
			t.Fatalf("expected 1 match, got %d", len(matches))
		}
		if matches[0].RuleID != "private-key" {
			t.Errorf("expected rule private-key, got %q", matches[0].RuleID)
		}
	})

	t.Run("detects openai api key", func(t *testing.T) {
		// Uses the second alternative: sk-[a-zA-Z0-9]{20}T3BlbkFJ[a-zA-Z0-9]{20}
		// Split to avoid GitHub push protection false-positive on the literal.
		content := "sk-AAAAAAAAAAAAAAAAAAAA" + "T3" + "BlbkFJ" + "AAAAAAAAAAAAAAAAAAAA"
		matches := ScanForSecrets(content)
		if len(matches) != 1 {
			t.Fatalf("expected 1 match, got %d", len(matches))
		}
		if matches[0].RuleID != "openai-api-key" {
			t.Errorf("expected rule openai-api-key, got %q", matches[0].RuleID)
		}
	})

	t.Run("detects multiple rules in same content", func(t *testing.T) {
		content := "AWS: AKIAIOSFODNN7EXAMPLE\nGitHub: ghp_123456789012345678901234567890123456"
		matches := ScanForSecrets(content)
		if len(matches) < 2 {
			t.Fatalf("expected at least 2 matches, got %d", len(matches))
		}
		found := make(map[string]bool)
		for _, m := range matches {
			found[m.RuleID] = true
		}
		if !found["aws-access-token"] {
			t.Error("expected aws-access-token rule to match")
		}
		if !found["github-pat"] {
			t.Error("expected github-pat rule to match")
		}
	})

	t.Run("returns empty for clean content", func(t *testing.T) {
		content := "This is perfectly safe content without any secrets."
		matches := ScanForSecrets(content)
		if len(matches) != 0 {
			t.Errorf("expected 0 matches for clean content, got %d", len(matches))
		}
	})

	t.Run("deduplicates when same rule fires multiple times", func(t *testing.T) {
		content := "AKIAIOSFODNN7EXAMPLE and also AKIAIOSFODNN7EXAMPLE"
		matches := ScanForSecrets(content)
		if len(matches) != 1 {
			t.Fatalf("expected 1 match for duplicate rule, got %d", len(matches))
		}
		if matches[0].RuleID != "aws-access-token" {
			t.Errorf("expected aws-access-token, got %q", matches[0].RuleID)
		}
	})

	t.Run("handles empty string", func(t *testing.T) {
		matches := ScanForSecrets("")
		if len(matches) != 0 {
			t.Errorf("expected 0 matches for empty string, got %d", len(matches))
		}
	})

	t.Run("detects gcp api key", func(t *testing.T) {
		content := "AIza" + strings.Repeat("a", 35)
		matches := ScanForSecrets(content)
		if len(matches) != 1 {
			t.Fatalf("expected 1 match, got %d", len(matches))
		}
		if matches[0].RuleID != "gcp-api-key" {
			t.Errorf("expected rule gcp-api-key, got %q", matches[0].RuleID)
		}
	})

	t.Run("detects digitalocean pat", func(t *testing.T) {
		content := "dop_v1_" + strings.Repeat("a", 64)
		matches := ScanForSecrets(content)
		if len(matches) != 1 {
			t.Fatalf("expected 1 match, got %d", len(matches))
		}
		if matches[0].RuleID != "digitalocean-pat" {
			t.Errorf("expected rule digitalocean-pat, got %q", matches[0].RuleID)
		}
	})
}

func TestGetSecretLabel(t *testing.T) {
	t.Run("returns correct labels for known rules", func(t *testing.T) {
		tests := []struct {
			ruleID string
			want   string
		}{
			{"aws-access-token", "AWS Access Token"},
			{"github-pat", "GitHub PAT"},
			{"digitalocean-pat", "DigitalOcean PAT"},
			{"openai-api-key", "OpenAI API Key"},
			{"anthropic-api-key", "Anthropic API Key"},
			{"slack-bot-token", "Slack Bot Token"},
			{"gcp-api-key", "GCP API Key"},
			{"gitlab-pat", "GitLab PAT"},
			{"huggingface-access-token", "HuggingFace Access Token"},
			{"hashicorp-tf-api-token", "HashiCorp TF API Token"},
			{"sendgrid-api-token", "SendGrid API Token"},
		}
		for _, tc := range tests {
			t.Run(tc.ruleID, func(t *testing.T) {
				if got := GetSecretLabel(tc.ruleID); got != tc.want {
					t.Errorf("GetSecretLabel(%q) = %q, want %q", tc.ruleID, got, tc.want)
				}
			})
		}
	})

	t.Run("falls back to capitalized kebab-case for unknown IDs", func(t *testing.T) {
		label := GetSecretLabel("my-custom-secret")
		want := "My Custom Secret"
		if label != want {
			t.Errorf("GetSecretLabel(%q) = %q, want %q", "my-custom-secret", label, want)
		}
	})
}

// ─── File Watcher Tests ────────────────────────────────────────────

func TestNewTeamMemoryWatcher(t *testing.T) {
	cfg := WatcherConfig{
		TeamDir: t.TempDir(),
		PullFunc: func(ctx context.Context) (*FetchResult, error) {
			return &FetchResult{Success: true}, nil
		},
		PushFunc: func(ctx context.Context) (*PushResult, error) {
			return &PushResult{Success: true}, nil
		},
	}

	w := NewTeamMemoryWatcher(cfg)
	if w == nil {
		t.Fatal("NewTeamMemoryWatcher returned nil")
	}

	// Verify config was stored by checking behaviour.
	if w.teamDir != cfg.TeamDir {
		t.Errorf("teamDir: got %q, want %q", w.teamDir, cfg.TeamDir)
	}
	if w.pullFunc == nil {
		t.Error("pullFunc should not be nil")
	}
	if w.pushFunc == nil {
		t.Error("pushFunc should not be nil")
	}
}

func TestIsPermanentFailure(t *testing.T) {
	t.Run("returns true for no_oauth error type", func(t *testing.T) {
		r := &PushResult{ErrorType: "no_oauth"}
		if !IsPermanentFailure(r) {
			t.Error("expected IsPermanentFailure to be true for no_oauth")
		}
	})

	t.Run("returns true for no_repo error type", func(t *testing.T) {
		r := &PushResult{ErrorType: "no_repo"}
		if !IsPermanentFailure(r) {
			t.Error("expected IsPermanentFailure to be true for no_repo")
		}
	})

	t.Run("returns true for 403 HTTP status", func(t *testing.T) {
		r := &PushResult{HTTPStatus: 403}
		if !IsPermanentFailure(r) {
			t.Error("expected IsPermanentFailure to be true for 403")
		}
	})

	t.Run("returns false for 409 Conflict", func(t *testing.T) {
		r := &PushResult{HTTPStatus: 409}
		if IsPermanentFailure(r) {
			t.Error("expected IsPermanentFailure to be false for 409 Conflict")
		}
	})

	t.Run("returns false for 429 Rate Limit", func(t *testing.T) {
		r := &PushResult{HTTPStatus: 429}
		if IsPermanentFailure(r) {
			t.Error("expected IsPermanentFailure to be false for 429 Rate Limit")
		}
	})

	t.Run("returns false for success result", func(t *testing.T) {
		r := &PushResult{Success: true}
		if IsPermanentFailure(r) {
			t.Error("expected IsPermanentFailure to be false for success result")
		}
	})
}

func TestWatcher_Start(t *testing.T) {
	t.Run("creates directory if missing", func(t *testing.T) {
		teamDir := filepath.Join(t.TempDir(), "does-not-exist-yet", "team")
		pullCalled := false

		w := NewTeamMemoryWatcher(WatcherConfig{
			TeamDir: teamDir,
			PullFunc: func(ctx context.Context) (*FetchResult, error) {
				pullCalled = true
				return &FetchResult{Success: true}, nil
			},
		})

		err := w.Start(context.Background())
		if err != nil {
			t.Fatalf("Start returned error: %v", err)
		}

		// Clean up the watcher (the fsnotify goroutine may or may not
		// have started depending on platform support).
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			w.Stop(ctx)
		}()

		if _, statErr := os.Stat(teamDir); os.IsNotExist(statErr) {
			t.Error("Start should create the team directory when it does not exist")
		}

		if !pullCalled {
			t.Error("PullFunc should have been called during Start")
		}
	})

	t.Run("handles nil pull func", func(t *testing.T) {
		teamDir := t.TempDir()

		w := NewTeamMemoryWatcher(WatcherConfig{
			TeamDir: teamDir,
			// PullFunc is nil
		})

		err := w.Start(context.Background())
		if err != nil {
			t.Fatalf("Start with nil PullFunc returned error: %v", err)
		}

		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			w.Stop(ctx)
		}()
	})
}

func TestWatcher_Stop(t *testing.T) {
	t.Run("graceful shutdown without start", func(t *testing.T) {
		w := NewTeamMemoryWatcher(WatcherConfig{
			TeamDir: t.TempDir(),
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Stop on a watcher that was never started should not panic.
		w.Stop(ctx)
	})

	t.Run("graceful shutdown after start", func(t *testing.T) {
		teamDir := t.TempDir()

		w := NewTeamMemoryWatcher(WatcherConfig{
			TeamDir: teamDir,
			PullFunc: func(ctx context.Context) (*FetchResult, error) {
				return &FetchResult{Success: true}, nil
			},
		})

		if err := w.Start(context.Background()); err != nil {
			t.Fatalf("Start failed: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Should not deadlock or panic.
		w.Stop(ctx)
	})
}

func TestWatcher_NotifyWrite(t *testing.T) {
	t.Run("schedules a push", func(t *testing.T) {
		pushed := make(chan struct{}, 1)

		w := NewTeamMemoryWatcher(WatcherConfig{
			TeamDir: t.TempDir(),
			PushFunc: func(ctx context.Context) (*PushResult, error) {
				select {
				case pushed <- struct{}{}:
				default:
				}
				return &PushResult{Success: true}, nil
			},
		})

		w.NotifyWrite()

		select {
		case <-pushed:
			// Push was scheduled and executed by the debounce timer.
		case <-time.After(debounceDuration + 2*time.Second):
			t.Error("timed out waiting for debounced push after NotifyWrite")
		}
	})

	t.Run("does not panic with nil push func", func(t *testing.T) {
		w := NewTeamMemoryWatcher(WatcherConfig{
			TeamDir: t.TempDir(),
			// PushFunc is nil
		})

		// NotifyWrite should not panic even with nil PushFunc.
		w.NotifyWrite()

		// Wait briefly for the debounce timer to fire (the nil-push path
		// in executePush handles this gracefully).
		time.Sleep(debounceDuration + 500*time.Millisecond)
	})
}

// ─── Integration: PushTeamMemory Secret Filtering ──────────────────

func TestPushTeamMemory_SecretFiltering(t *testing.T) {
	t.Run("filters entries with secrets — clean entries pass through", func(t *testing.T) {
		projectRoot := t.TempDir()
		teamDir := GetTeamMemPath(projectRoot)
		if err := os.MkdirAll(teamDir, 0o755); err != nil {
			t.Fatalf("failed to create team dir: %v", err)
		}

		// One clean file and one file containing an AWS access key.
		if err := os.WriteFile(filepath.Join(teamDir, "clean.md"), []byte("this is clean content"), 0o644); err != nil {
			t.Fatalf("write failed: %v", err)
		}
		if err := os.WriteFile(filepath.Join(teamDir, "secret.md"), []byte("AKIAIOSFODNN7EXAMPLE"), 0o644); err != nil {
			t.Fatalf("write failed: %v", err)
		}

		var uploadedEntries map[string]string
		_, url := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPut {
				var body struct {
					Entries map[string]string `json:"entries"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatalf("failed to decode PUT body: %v", err)
				}
				uploadedEntries = body.Entries
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{
					"checksum":     "sha256:new123",
					"lastModified": "2026-01-01T00:00:01Z",
				})
			} else {
				// Handle potential hashes probe (GET ?view=hashes) gracefully.
				w.WriteHeader(http.StatusNotFound)
			}
		})

		state := NewSyncState()
		result := PushTeamMemory(context.Background(), state, url, "owner/repo", "token-xxx", projectRoot)

		if !result.Success {
			t.Fatalf("PushTeamMemory failed: %s", result.Error)
		}
		if result.FilesUploaded != 1 {
			t.Errorf("expected 1 file uploaded, got %d", result.FilesUploaded)
		}

		// Verify the secret file was NOT uploaded.
		if _, ok := uploadedEntries["secret.md"]; ok {
			t.Error("secret.md should not have been uploaded")
		}
		if _, ok := uploadedEntries["clean.md"]; !ok {
			t.Error("clean.md should have been uploaded")
		}
	})

	t.Run("skips entries with detected secrets and reports SkippedSecrets", func(t *testing.T) {
		projectRoot := t.TempDir()
		teamDir := GetTeamMemPath(projectRoot)
		if err := os.MkdirAll(teamDir, 0o755); err != nil {
			t.Fatalf("failed to create team dir: %v", err)
		}

		// Two clean files and one file with a GitHub PAT.
		if err := os.WriteFile(filepath.Join(teamDir, "a.md"), []byte("alpha"), 0o644); err != nil {
			t.Fatalf("write failed: %v", err)
		}
		if err := os.WriteFile(filepath.Join(teamDir, "b.md"), []byte("bravo"), 0o644); err != nil {
			t.Fatalf("write failed: %v", err)
		}
		if err := os.WriteFile(filepath.Join(teamDir, "secret.md"), []byte("ghp_123456789012345678901234567890123456"), 0o644); err != nil {
			t.Fatalf("write failed: %v", err)
		}

		var uploadedEntries map[string]string
		_, url := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPut {
				var body struct {
					Entries map[string]string `json:"entries"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatalf("failed to decode PUT body: %v", err)
				}
				uploadedEntries = body.Entries
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{
					"checksum":     "sha256:new456",
					"lastModified": "2026-01-01T00:00:02Z",
				})
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		})

		state := NewSyncState()
		result := PushTeamMemory(context.Background(), state, url, "owner/repo", "token-xxx", projectRoot)

		if !result.Success {
			t.Fatalf("PushTeamMemory failed: %s", result.Error)
		}
		if result.FilesUploaded != 2 {
			t.Errorf("expected 2 files uploaded, got %d", result.FilesUploaded)
		}
		if len(result.SkippedSecrets) != 1 {
			t.Fatalf("expected 1 skipped secret, got %d", len(result.SkippedSecrets))
		}

		skipped := result.SkippedSecrets[0]
		if skipped.Path != "secret.md" {
			t.Errorf("expected skipped path 'secret.md', got %q", skipped.Path)
		}
		if skipped.RuleID != "github-pat" {
			t.Errorf("expected rule 'github-pat', got %q", skipped.RuleID)
		}
		if skipped.Label != "GitHub PAT" {
			t.Errorf("expected label 'GitHub PAT', got %q", skipped.Label)
		}

		// Verify only clean files were uploaded.
		if _, ok := uploadedEntries["secret.md"]; ok {
			t.Error("secret.md should not have been uploaded")
		}
		if _, ok := uploadedEntries["a.md"]; !ok {
			t.Error("a.md should have been uploaded")
		}
		if _, ok := uploadedEntries["b.md"]; !ok {
			t.Error("b.md should have been uploaded")
		}
	})

	t.Run("returns all entries skipped when all files contain secrets", func(t *testing.T) {
		projectRoot := t.TempDir()
		teamDir := GetTeamMemPath(projectRoot)
		if err := os.MkdirAll(teamDir, 0o755); err != nil {
			t.Fatalf("failed to create team dir: %v", err)
		}

		// All files contain secrets.
		if err := os.WriteFile(filepath.Join(teamDir, "aws.md"), []byte("AKIAIOSFODNN7EXAMPLE"), 0o644); err != nil {
			t.Fatalf("write failed: %v", err)
		}
		if err := os.WriteFile(filepath.Join(teamDir, "anthropic.md"), []byte("sk-ant-api03-"+strings.Repeat("a", 93)+"AA"), 0o644); err != nil {
			t.Fatalf("write failed: %v", err)
		}

		// No mock server needed — PushTeamMemory returns early when all
		// entries are filtered out, before any HTTP call.
		state := NewSyncState()
		result := PushTeamMemory(context.Background(), state, "http://127.0.0.1:1", "owner/repo", "token-xxx", projectRoot)

		if !result.Success {
			t.Fatalf("expected success (no files to push), got error: %s", result.Error)
		}
		if result.FilesUploaded != 0 {
			t.Errorf("expected 0 files uploaded, got %d", result.FilesUploaded)
		}
		if len(result.SkippedSecrets) != 2 {
			t.Fatalf("expected 2 skipped secrets, got %d", len(result.SkippedSecrets))
		}

		// Verify individual skipped entries.
		skippedByPath := make(map[string]SkippedSecretFile)
		for _, s := range result.SkippedSecrets {
			skippedByPath[s.Path] = s
		}

		awsSkipped, ok := skippedByPath["aws.md"]
		if !ok {
			t.Error("expected aws.md in SkippedSecrets")
		} else {
			if awsSkipped.RuleID != "aws-access-token" {
				t.Errorf("aws.md rule: expected aws-access-token, got %q", awsSkipped.RuleID)
			}
		}

		antSkipped, ok := skippedByPath["anthropic.md"]
		if !ok {
			t.Error("expected anthropic.md in SkippedSecrets")
		} else {
			if antSkipped.RuleID != "anthropic-api-key" {
				t.Errorf("anthropic.md rule: expected anthropic-api-key, got %q", antSkipped.RuleID)
			}
		}
	})
}
