package teleport

import (
	"context"
	"testing"
)

// TestTypes_Construction verifies all core types can be constructed.
func TestTypes_Construction(t *testing.T) {
	t.Run("TeleportProgressStep", func(t *testing.T) {
		if ProgressValidating != "validating" {
			t.Fatalf("ProgressValidating = %q, want \"validating\"", ProgressValidating)
		}
		if ProgressDone != "done" {
			t.Fatalf("ProgressDone = %q, want \"done\"", ProgressDone)
		}
	})

	t.Run("SessionStatus", func(t *testing.T) {
		if SessionRunning != "running" {
			t.Fatalf("SessionRunning = %q, want \"running\"", SessionRunning)
		}
		if SessionArchived != "archived" {
			t.Fatalf("SessionArchived = %q, want \"archived\"", SessionArchived)
		}
	})

	t.Run("EnvironmentKind", func(t *testing.T) {
		if EnvironmentAnthropicCloud != "anthropic_cloud" {
			t.Fatalf("EnvironmentAnthropicCloud = %q, want \"anthropic_cloud\"", EnvironmentAnthropicCloud)
		}
		if EnvironmentBYOC != "byoc" {
			t.Fatalf("EnvironmentBYOC = %q, want \"byoc\"", EnvironmentBYOC)
		}
	})

	t.Run("BundleScope", func(t *testing.T) {
		if BundleScopeAll != "all" {
			t.Fatalf("BundleScopeAll = %q, want \"all\"", BundleScopeAll)
		}
		if BundleScopeSquashed != "squashed" {
			t.Fatalf("BundleScopeSquashed = %q, want \"squashed\"", BundleScopeSquashed)
		}
	})

	t.Run("BundleFailReason", func(t *testing.T) {
		if BundleFailGitError != "git_error" {
			t.Fatalf("BundleFailGitError = %q, want \"git_error\"", BundleFailGitError)
		}
	})

	t.Run("CCRBetaHeader", func(t *testing.T) {
		if CCRBetaHeader != "ccr-byoc-2025-07-29" {
			t.Fatalf("CCRBetaHeader = %q, want \"ccr-byoc-2025-07-29\"", CCRBetaHeader)
		}
	})
}

// TestTeleportOperationError verifies the teleport-specific error type.
func TestTeleportOperationError(t *testing.T) {
	err := NewTeleportOperationError("internal error", "User message")

	if err.Error() != "internal error" {
		t.Fatalf("Error() = %q, want \"internal error\"", err.Error())
	}

	if msg := err.UserFacingMessage(); msg != "User message" {
		t.Fatalf("UserFacingMessage() = %q, want \"User message\"", msg)
	}

	errNoUser := NewTeleportOperationError("only msg", "")
	if msg := errNoUser.UserFacingMessage(); msg != "only msg" {
		t.Fatalf("UserFacingMessage() = %q, want \"only msg\"", msg)
	}
}

// TestGitSource_ImplementsInterface verifies GitSource satisfies SessionContextSource.
func TestGitSource_ImplementsInterface(t *testing.T) {
	var src SessionContextSource = GitSource{Type: "git_repository", URL: "https://github.com/test/repo"}
	_ = src // compile-time check
}

// TestGitRepositoryOutcome verifies outcome construction.
func TestGitRepositoryOutcome(t *testing.T) {
	outcome := GitRepositoryOutcome{
		Type: "git_repository",
		GitInfo: OutcomeGitInfo{
			Type:     "github",
			Repo:     "owner/repo",
			Branches: []string{"main"},
		},
	}
	if outcome.GitInfo.Repo != "owner/repo" {
		t.Fatalf("Repo = %q, want \"owner/repo\"", outcome.GitInfo.Repo)
	}
	if len(outcome.GitInfo.Branches) != 1 || outcome.GitInfo.Branches[0] != "main" {
		t.Fatalf("Branches = %v, want [\"main\"]", outcome.GitInfo.Branches)
	}
}

// TestSessionResource_Construction verifies SessionResource construction.
func TestSessionResource_Construction(t *testing.T) {
	session := SessionResource{
		Type:          "session",
		ID:            "sess_123",
		Title:         "Test session",
		SessionStatus: SessionRunning,
		EnvironmentID: "env_1",
		SessionContext: SessionContext{
			CWD: "/home/user/project",
		},
	}

	if session.ID != "sess_123" {
		t.Fatalf("ID = %q, want \"sess_123\"", session.ID)
	}
	if session.SessionStatus != SessionRunning {
		t.Fatalf("SessionStatus = %q, want \"running\"", session.SessionStatus)
	}
}

// TestBundleUploadResult_Construction verifies BundleUploadResult construction.
func TestBundleUploadResult_Construction(t *testing.T) {
	success := BundleUploadResult{
		Success:         true,
		FileID:          "file_abc",
		BundleSizeBytes: 1024,
		Scope:           BundleScopeHead,
		HasWip:          false,
	}
	if !success.Success {
		t.Fatal("Success should be true")
	}
	if success.FileID != "file_abc" {
		t.Fatalf("FileID = %q, want \"file_abc\"", success.FileID)
	}

	failure := BundleUploadResult{
		Success:    false,
		Error:      "something went wrong",
		FailReason: BundleFailGitError,
	}
	if failure.Success {
		t.Fatal("Success should be false")
	}
	if failure.FailReason != BundleFailGitError {
		t.Fatalf("FailReason = %q, want \"git_error\"", failure.FailReason)
	}
}

// TestRepoValidationStatus verifies all repo validation status values.
func TestRepoValidationStatus(t *testing.T) {
	if RepoMatch != "match" {
		t.Fatalf("RepoMatch = %q, want \"match\"", RepoMatch)
	}
	if RepoMismatch != "mismatch" {
		t.Fatalf("RepoMismatch = %q, want \"mismatch\"", RepoMismatch)
	}
	if RepoNotInRepo != "not_in_repo" {
		t.Fatalf("RepoNotInRepo = %q, want \"not_in_repo\"", RepoNotInRepo)
	}
	if RepoNoRepoReq != "no_repo_required" {
		t.Fatalf("RepoNoRepoReq = %q, want \"no_repo_required\"", RepoNoRepoReq)
	}
	if RepoError != "error" {
		t.Fatalf("RepoError = %q, want \"error\"", RepoError)
	}
}

// TestEnvironmentSelectionInfo verifies environment selection info.
func TestEnvironmentSelectionInfo(t *testing.T) {
	info := EnvironmentSelectionInfo{
		AvailableEnvironments: []EnvironmentResource{
			{EnvironmentID: "env_1", Kind: EnvironmentAnthropicCloud, Name: "Default", State: "active"},
		},
		SelectedEnvironment: &EnvironmentResource{
			EnvironmentID: "env_1", Kind: EnvironmentAnthropicCloud, Name: "Default", State: "active",
		},
		SelectedEnvironmentSource: "settings",
	}

	if len(info.AvailableEnvironments) != 1 {
		t.Fatalf("AvailableEnvironments count = %d, want 1", len(info.AvailableEnvironments))
	}
	if info.SelectedEnvironmentSource != "settings" {
		t.Fatalf("SelectedEnvironmentSource = %q, want \"settings\"", info.SelectedEnvironmentSource)
	}
	if info.AvailableEnvironments[0].Kind != EnvironmentAnthropicCloud {
		t.Fatalf("Kind = %q, want \"anthropic_cloud\"", info.AvailableEnvironments[0].Kind)
	}
}

// TestMessage_Construction verifies Message construction.
func TestMessage_Construction(t *testing.T) {
	msg := Message{
		Role:    "user",
		Content: "Hello",
		IsMeta:  true,
	}
	if msg.Role != "user" {
		t.Fatalf("Role = %q, want \"user\"", msg.Role)
	}
	if !msg.IsMeta {
		t.Fatal("IsMeta should be true")
	}
}

// TestTeleportService_Constructor verifies TeleportService creation.
func TestTeleportService_Constructor(t *testing.T) {
	svc := NewTeleportService("https://api.anthropic.com", "test-token", "test-org")
	if svc == nil {
		t.Fatal("NewTeleportService() returned nil")
	}
	if svc.baseURL != "https://api.anthropic.com" {
		t.Fatalf("baseURL = %q, want \"https://api.anthropic.com\"", svc.baseURL)
	}
}

// TestValidateSessionRepository verifies repository validation logic.
func TestValidateSessionRepository(t *testing.T) {
	t.Run("no repo required", func(t *testing.T) {
		session := &SessionResource{
			SessionContext: SessionContext{
				Sources: []SessionContextSource{},
			},
		}
		result := ValidateSessionRepository(session)
		if result.Status != RepoNoRepoReq {
			t.Fatalf("Status = %q, want \"no_repo_required\"", result.Status)
		}
	})
}

// TestProcessMessagesForTeleportResume verifies message processing.
func TestProcessMessagesForTeleportResume(t *testing.T) {
	input := []Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi"},
	}

	result := ProcessMessagesForTeleportResume(input)

	if len(result) != 3 {
		t.Fatalf("result length = %d, want 3", len(result))
	}

	if result[0].Role != "user" || result[0].Content != "Hello" {
		t.Fatal("First message should be preserved")
	}

	last := result[2]
	if last.Role != "user" || !last.IsMeta {
		t.Fatal("Last message should be a meta user message")
	}
}

// TestFindGitRootFromCWD verifies findGitRootFromCWD returns empty when not in a repo.
func TestFindGitRootFromCWD(t *testing.T) {
	root := findGitRootFromCWD()
	// This test runs inside claude-code-go which IS a git repo,
	// so root should be non-empty.
	if root == "" {
		t.Fatal("findGitRootFromCWD() returned empty, expected a git root")
	}
}

// TestPollRemoteSessionEventsResponse verifies the response type.
func TestPollRemoteSessionEventsResponse(t *testing.T) {
	resp := &PollRemoteSessionEventsResponse{
		NewEvents:     []interface{}{},
		LastEventID:   "evt_123",
		Branch:        "main",
		SessionStatus: SessionRunning,
	}
	if resp.LastEventID != "evt_123" {
		t.Fatalf("LastEventID = %q, want \"evt_123\"", resp.LastEventID)
	}
	if resp.SessionStatus != SessionRunning {
		t.Fatalf("SessionStatus = %q, want \"running\"", resp.SessionStatus)
	}
}

// TestContentBlock verifies content block construction.
func TestContentBlock(t *testing.T) {
	block := ContentBlock{
		Type: "text",
		Data: map[string]interface{}{"text": "hello"},
	}
	if block.Type != "text" {
		t.Fatalf("Type = %q, want \"text\"", block.Type)
	}
}

// TestTeleportService_TeleportToRemoteNoToken verifies error on missing token.
func TestTeleportService_TeleportToRemoteNoToken(t *testing.T) {
	svc := NewTeleportService("https://api.anthropic.com", "", "test-org")
	_, err := svc.TeleportToRemote(context.Background(), TeleportToRemoteOptions{})
	if err == nil {
		t.Fatal("TeleportToRemote() error = nil, want no access token error")
	}
}

// TestTeleportService_TeleportToRemoteNoOrg verifies error on missing org UUID.
func TestTeleportService_TeleportToRemoteNoOrg(t *testing.T) {
	svc := NewTeleportService("https://api.anthropic.com", "test-token", "")
	_, err := svc.TeleportToRemote(context.Background(), TeleportToRemoteOptions{})
	if err == nil {
		t.Fatal("TeleportToRemote() error = nil, want no org UUID error")
	}
}

// TestTruncateStr verifies the truncation helper.
func TestTruncateStr(t *testing.T) {
	if s := truncateStr("short", 10); s != "short" {
		t.Fatalf("truncateStr(\"short\", 10) = %q, want \"short\"", s)
	}
	if s := truncateStr("this is a long string", 10); s != "this is a " {
		t.Fatalf("truncateStr(\"long\", 10) = %q, want \"this is a \"", s)
	}
	if s := truncateStr("", 5); s != "" {
		t.Fatalf("truncateStr(\"\", 5) = %q, want \"\"", s)
	}
}

// TestFileSize verifies the file size helper.
func TestFileSize(t *testing.T) {
	size := fileSize("/nonexistent/file.test")
	if size != -1 {
		t.Fatalf("fileSize(\"/nonexistent/file.test\") = %d, want -1", size)
	}
}

// TestGetBranchFromSession verifies branch extraction from session.
func TestGetBranchFromSession(t *testing.T) {
	t.Run("no outcomes", func(t *testing.T) {
		session := &SessionResource{
			SessionContext: SessionContext{
				Outcomes: nil,
			},
		}
		if b := getBranchFromSession(session); b != "" {
			t.Fatalf("getBranchFromSession() = %q, want \"\"", b)
		}
	})

	t.Run("with git outcome", func(t *testing.T) {
		session := &SessionResource{
			SessionContext: SessionContext{
				Outcomes: []GitRepositoryOutcome{
					{
						Type: "git_repository",
						GitInfo: OutcomeGitInfo{
							Type:     "github",
							Repo:     "owner/repo",
							Branches: []string{"feature/test"},
						},
					},
				},
			},
		}
		if b := getBranchFromSession(session); b != "feature/test" {
			t.Fatalf("getBranchFromSession() = %q, want \"feature/test\"", b)
		}
	})
}

// TestTeleportService_TeleportResumeNoToken verifies error on missing token.
func TestTeleportService_TeleportResumeNoToken(t *testing.T) {
	svc := NewTeleportService("https://api.anthropic.com", "", "test-org")
	_, _, err := svc.TeleportResumeCodeSession(context.Background(), "sess_123")
	if err == nil {
		t.Fatal("TeleportResumeCodeSession() error = nil, want error")
	}
}
