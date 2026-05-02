package anthropic

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseFileSpecs(t *testing.T) {
	files := ParseFileSpecs([]string{
		"file_abc123:docs/readme.md",
		"file_def456:src/main.go",
	})

	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if files[0].FileID != "file_abc123" || files[0].RelativePath != "docs/readme.md" {
		t.Fatalf("first file = %+v, want file_abc123:docs/readme.md", files[0])
	}
	if files[1].FileID != "file_def456" || files[1].RelativePath != "src/main.go" {
		t.Fatalf("second file = %+v, want file_def456:src/main.go", files[1])
	}
}

func TestParseFileSpecs_MultiSpace(t *testing.T) {
	files := ParseFileSpecs([]string{
		"file_a:path/1 file_b:path/2",
	})

	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
}

func TestParseFileSpecs_Invalid(t *testing.T) {
	files := ParseFileSpecs([]string{
		"nocolon",
		":emptyid",
		"emptypath:",
	})

	if len(files) != 0 {
		t.Fatalf("expected 0 files, got %d", len(files))
	}
}

func TestFilesAPIDownloadFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/v1/files/file_123/content" {
			t.Fatalf("path = %q, want /v1/files/file_123/content", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("authorization = %q, want Bearer test-token", r.Header.Get("Authorization"))
		}
		w.Write([]byte("file content"))
	}))
	defer server.Close()

	cfg := &FilesAPIConfig{
		OAuthToken: "test-token",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	data, err := cfg.DownloadFile("file_123")
	if err != nil {
		t.Fatalf("DownloadFile error = %v", err)
	}
	if string(data) != "file content" {
		t.Fatalf("content = %q, want file content", string(data))
	}
}

func TestFilesAPIDownloadFile_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &FilesAPIConfig{
		OAuthToken: "test-token",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	_, err := cfg.DownloadFile("file_missing")
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestFilesAPIDownloadAndSaveFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("saved content"))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	cfg := &FilesAPIConfig{
		OAuthToken: "test-token",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	result := cfg.DownloadAndSaveFile(File{FileID: "file_456", RelativePath: "data/output.txt"}, tmpDir)
	if !result.Success {
		t.Fatalf("DownloadAndSaveFile failed: %s", result.Error)
	}

	data, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatalf("read file error = %v", err)
	}
	if string(data) != "saved content" {
		t.Fatalf("content = %q, want saved content", string(data))
	}
}

func TestFilesAPIUploadFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/files" {
			t.Fatalf("path = %q, want /v1/files", r.URL.Path)
		}

		ct := r.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "multipart/form-data") {
			t.Fatalf("content-type = %q, want multipart/form-data", ct)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, `{"id":"file_upload_789","size_bytes":13}`)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(filePath, []byte("hello, world!"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	cfg := &FilesAPIConfig{
		OAuthToken: "test-token",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	result := cfg.UploadFile(filePath, "uploads/test.txt")
	if !result.Success {
		t.Fatalf("UploadFile failed: %s", result.Error)
	}
	if result.FileID != "file_upload_789" {
		t.Fatalf("fileID = %q, want file_upload_789", result.FileID)
	}
	if result.Size != 13 {
		t.Fatalf("size = %d, want 13", result.Size)
	}
}

func TestFilesAPIUploadFile_TooLarge(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "huge.bin")
	if err := os.WriteFile(filePath, make([]byte, maxFileSizeBytes+1), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	cfg := &FilesAPIConfig{OAuthToken: "token"}
	result := cfg.UploadFile(filePath, "huge.bin")
	if result.Success {
		t.Fatal("expected failure for oversized file")
	}
	if result.Error == "" {
		t.Fatal("expected error message")
	}
}

func TestFilesAPIUploadFile_MissingFile(t *testing.T) {
	cfg := &FilesAPIConfig{OAuthToken: "token"}
	result := cfg.UploadFile("/nonexistent/path.txt", "path.txt")
	if result.Success {
		t.Fatal("expected failure for missing file")
	}
}

func TestFilesAPIListFiles(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/v1/files" {
			t.Fatalf("path = %q, want /v1/files", r.URL.Path)
		}

		callCount++
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("after_id") == "" {
			fmt.Fprintf(w, `{"data":[{"id":"file_1","filename":"a.txt","size_bytes":100}],"has_more":true}`)
		} else {
			fmt.Fprintf(w, `{"data":[{"id":"file_2","filename":"b.txt","size_bytes":200}],"has_more":false}`)
		}
	}))
	defer server.Close()

	cfg := &FilesAPIConfig{
		OAuthToken: "test-token",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	files, err := cfg.ListFilesCreatedAfter("2025-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("ListFilesCreatedAfter error = %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if files[0].FileID != "file_1" || files[1].FileID != "file_2" {
		t.Fatalf("files = %+v", files)
	}
	if callCount != 2 {
		t.Fatalf("expected 2 API calls, got %d", callCount)
	}
}

func TestFilesAPIRetry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("success after retry"))
	}))
	defer server.Close()

	cfg := &FilesAPIConfig{
		OAuthToken: "test-token",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	data, err := cfg.DownloadFile("file_retry")
	if err != nil {
		t.Fatalf("DownloadFile error = %v", err)
	}
	if string(data) != "success after retry" {
		t.Fatalf("content = %q", string(data))
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestFilesAPIRetryExhausted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	cfg := &FilesAPIConfig{
		OAuthToken: "test-token",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	_, err := cfg.DownloadFile("file_fail")
	if err == nil {
		t.Fatal("expected error after retries exhausted")
	}
}
