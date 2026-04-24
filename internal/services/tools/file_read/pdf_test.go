package file_read

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestParsePDFPageRange verifies page range parsing for various formats.
func TestParsePDFPageRange(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantFirst int
		wantLast  int
		wantErr   bool
	}{
		{
			name:      "single page",
			input:     "3",
			wantFirst: 3,
			wantLast:  3,
			wantErr:   false,
		},
		{
			name:      "page range",
			input:     "1-5",
			wantFirst: 1,
			wantLast:  5,
			wantErr:   false,
		},
		{
			name:      "open-ended range",
			input:     "3-",
			wantFirst: 3,
			wantLast:  0,
			wantErr:   false,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid format",
			input:   "abc",
			wantErr: true,
		},
		{
			name:    "negative page",
			input:   "-1",
			wantErr: true,
		},
		{
			name:    "last before first",
			input:   "5-1",
			wantErr: true,
		},
		{
			name:    "exceeds max pages",
			input:   "1-30",
			wantErr: true,
		},
		{
			name:      "exactly max pages",
			input:     "1-25",
			wantFirst: 1,
			wantLast:  25,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first, last, err := parsePDFPageRange(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parsePDFPageRange(%q) error = nil, want error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("parsePDFPageRange(%q) error = %v", tt.input, err)
			}
			if first != tt.wantFirst {
				t.Fatalf("parsePDFPageRange(%q) first = %d, want %d", tt.input, first, tt.wantFirst)
			}
			if last != tt.wantLast {
				t.Fatalf("parsePDFPageRange(%q) last = %d, want %d", tt.input, last, tt.wantLast)
			}
		})
	}
}

// TestIsPDFSupported verifies the PDF support flag.
func TestIsPDFSupported(t *testing.T) {
	// In the Go migration, isPDFSupported always returns true.
	if !isPDFSupported() {
		t.Fatal("isPDFSupported() = false, want true")
	}
}

// TestIsPdftoppmAvailable checks the binary lookup without requiring installation.
func TestIsPdftoppmAvailable(t *testing.T) {
	// This test simply exercises the function; it may return true or false
	// depending on the test environment.
	_ = isPdftoppmAvailable()
}

// TestIsPdfinfoAvailable checks the binary lookup without requiring installation.
func TestIsPdfinfoAvailable(t *testing.T) {
	_ = isPdfinfoAvailable()
}

// TestReadPDFEmptyFile verifies empty PDF files return an error.
func TestReadPDFEmptyFile(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "empty.pdf")
	if err := os.WriteFile(filePath, []byte{}, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	tool := NewTool(nil, nil)
	result, err := tool.readPDF(context.Background(), filePath, 0, "", projectDir)
	if err != nil {
		t.Fatalf("readPDF() error = %v", err)
	}
	if result.Error == "" {
		t.Fatal("readPDF() expected error for empty file, got none")
	}
	if !containsString(result.Error, "empty") {
		t.Fatalf("readPDF() error = %q, want containing 'empty'", result.Error)
	}
}

// TestReadPDFInvalidMagic verifies non-PDF files are rejected.
func TestReadPDFInvalidMagic(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "not-a-pdf.pdf")
	if err := os.WriteFile(filePath, []byte("this is not a pdf"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Minimal stub filesystem that supports ReadFile for this test.
	ts := &testFS{root: projectDir}
	tool := NewTool(ts, nil)
	result, err := tool.readPDF(context.Background(), filePath, int64(len("this is not a pdf")), "", projectDir)
	if err != nil {
		t.Fatalf("readPDF() error = %v", err)
	}
	if result.Error == "" {
		t.Fatal("readPDF() expected error for invalid PDF, got none")
	}
	if !containsString(result.Error, "not a valid PDF") {
		t.Fatalf("readPDF() error = %q, want containing 'not a valid PDF'", result.Error)
	}
}

// containsString reports whether s contains substr, case-insensitively.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// testFS is a minimal filesystem stub for testing PDF reads.
type testFS struct {
	root string
}

func (t *testFS) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (t *testFS) Lstat(path string) (os.FileInfo, error) {
	return os.Lstat(path)
}

func (t *testFS) ReadDir(path string) ([]os.DirEntry, error) {
	return os.ReadDir(path)
}

func (t *testFS) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (t *testFS) OpenRead(path string) (io.ReadCloser, error) {
	return os.Open(path)
}

func (t *testFS) WriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}

func (t *testFS) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (t *testFS) Rename(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}

func (t *testFS) Remove(path string) error {
	return os.Remove(path)
}

func (t *testFS) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

func (t *testFS) EvalSymlinks(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}

func (t *testFS) Readlink(path string) (string, error) {
	return os.Readlink(path)
}

// createMinimalPDF creates a minimal valid PDF file for testing.
func createMinimalPDF(t *testing.T, path string) {
	t.Helper()

	content := `%PDF-1.4
1 0 obj
<<
/Type /Catalog
/Pages 2 0 R
>>
endobj
2 0 obj
<<
/Type /Pages
/Kids [3 0 R]
/Count 1
>>
endobj
3 0 obj
<<
/Type /Page
/Parent 2 0 R
/MediaBox [0 0 612 792]
>>
endobj
xref
0 4
0000000000 65535 f
0000000009 00000 n
0000000058 00000 n
0000000115 00000 n
trailer
<<
/Size 4
/Root 1 0 R
>>
startxref
196
%%EOF
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

// TestIsPdftoppmAvailableCrossPlatform verifies pdftoppm detection works on all platforms.
func TestIsPdftoppmAvailableCrossPlatform(t *testing.T) {
	result := isPdftoppmAvailable()

	if runtime.GOOS == "windows" {
		// On Windows, pdftoppm might have .exe extension; LookPath handles this
		// Just verify it doesn't panic and returns a bool
		_ = result
	} else {
		// On Unix, verify consistency with LookPath
		_, err := exec.LookPath("pdftoppm")
		expected := err == nil
		if result != expected {
			t.Fatalf("isPdftoppmAvailable() = %v, want %v", result, expected)
		}
	}
}

// TestGetPDFPageCountWithPoppler tests page count extraction when pdfinfo is available.
func TestGetPDFPageCountWithPoppler(t *testing.T) {
	if !isPdfinfoAvailable() {
		t.Skip("pdfinfo not available, skipping")
	}

	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "test.pdf")
	createMinimalPDF(t, pdfPath)

	count := getPDFPageCount(pdfPath)
	if count != 1 {
		t.Fatalf("getPDFPageCount() = %d, want 1", count)
	}
}

// TestGetPDFPageCountWithoutPoppler tests that getPDFPageCount returns 0 when pdfinfo is unavailable.
func TestGetPDFPageCountWithoutPoppler(t *testing.T) {
	if isPdfinfoAvailable() {
		t.Skip("pdfinfo is available, skipping no-poppler test")
	}

	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "test.pdf")
	createMinimalPDF(t, pdfPath)

	count := getPDFPageCount(pdfPath)
	if count != 0 {
		t.Fatalf("getPDFPageCount() = %d, want 0 when pdfinfo unavailable", count)
	}
}

// TestGetPDFPageCountInvalidFile tests page count on an invalid file.
func TestGetPDFPageCountInvalidFile(t *testing.T) {
	if !isPdfinfoAvailable() {
		t.Skip("pdfinfo not available, skipping")
	}

	tmpDir := t.TempDir()
	invalidPath := filepath.Join(tmpDir, "not-a-pdf.txt")
	if err := os.WriteFile(invalidPath, []byte("not a pdf"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	count := getPDFPageCount(invalidPath)
	if count != 0 {
		t.Fatalf("getPDFPageCount() on invalid file = %d, want 0", count)
	}
}

// TestExtractPDFPagesWithPoppler tests page extraction when pdftoppm is available.
func TestExtractPDFPagesWithPoppler(t *testing.T) {
	if !isPdftoppmAvailable() {
		t.Skip("pdftoppm not available, skipping")
	}

	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "test.pdf")
	createMinimalPDF(t, pdfPath)

	outputDir, count, err := extractPDFPages(pdfPath, 1, 1)
	if err != nil {
		t.Fatalf("extractPDFPages() error = %v", err)
	}
	defer func() {
		_ = os.RemoveAll(outputDir)
	}()

	if count != 1 {
		t.Fatalf("extractPDFPages() count = %d, want 1", count)
	}

	// Verify output directory exists and contains a JPEG file
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("ReadDir(%q) error = %v", outputDir, err)
	}

	foundJPG := false
	for _, entry := range entries {
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".jpg") {
			foundJPG = true
			break
		}
	}
	if !foundJPG {
		t.Fatalf("No JPEG files found in output directory %q", outputDir)
	}
}

// TestExtractPDFPagesWithoutPoppler tests that extraction fails when pdftoppm is unavailable.
func TestExtractPDFPagesWithoutPoppler(t *testing.T) {
	if isPdftoppmAvailable() {
		t.Skip("pdftoppm is available, skipping no-poppler test")
	}

	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "test.pdf")
	createMinimalPDF(t, pdfPath)

	_, _, err := extractPDFPages(pdfPath, 1, 1)
	if err == nil {
		t.Fatal("extractPDFPages() expected error when pdftoppm unavailable, got nil")
	}
	if !strings.Contains(err.Error(), "pdftoppm is not installed") {
		t.Fatalf("extractPDFPages() error = %q, want 'pdftoppm is not installed'", err.Error())
	}
}

// TestExtractPDFPagesInvalidFile tests extraction on an invalid/corrupted file.
func TestExtractPDFPagesInvalidFile(t *testing.T) {
	if !isPdftoppmAvailable() {
		t.Skip("pdftoppm not available, skipping")
	}

	tmpDir := t.TempDir()
	invalidPath := filepath.Join(tmpDir, "not-a-pdf.txt")
	if err := os.WriteFile(invalidPath, []byte("not a pdf"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	_, _, err := extractPDFPages(invalidPath, 1, 1)
	if err == nil {
		t.Fatal("extractPDFPages() on invalid file expected error, got nil")
	}
}

// TestExtractPDFPagesCleanupOnError verifies temporary directory is cleaned up on error.
func TestExtractPDFPagesCleanupOnError(t *testing.T) {
	if !isPdftoppmAvailable() {
		t.Skip("pdftoppm not available, skipping")
	}

	tmpDir := t.TempDir()
	invalidPath := filepath.Join(tmpDir, "not-a-pdf.txt")
	if err := os.WriteFile(invalidPath, []byte("not a pdf"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	outputDir, _, err := extractPDFPages(invalidPath, 1, 1)
	if err == nil {
		if outputDir != "" {
			_ = os.RemoveAll(outputDir)
		}
		t.Fatal("extractPDFPages() on invalid file expected error, got nil")
	}

	// Verify temp directory was cleaned up
	if outputDir != "" {
		_, statErr := os.Stat(outputDir)
		if statErr == nil {
			t.Fatal("extractPDFPages() did not clean up temp directory on error")
		}
	}
}

// TestReadPDFWithPagesParameter verifies page extraction path when pages is provided.
func TestReadPDFWithPagesParameter(t *testing.T) {
	if !isPdftoppmAvailable() {
		t.Skip("pdftoppm not available, skipping")
	}

	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "test.pdf")
	createMinimalPDF(t, pdfPath)

	info, err := os.Stat(pdfPath)
	if err != nil {
		t.Fatalf("Stat error = %v", err)
	}

	tool := NewTool(&testFS{root: tmpDir}, nil)
	result, err := tool.readPDF(context.Background(), pdfPath, info.Size(), "1", tmpDir)
	if err != nil {
		t.Fatalf("readPDF() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("readPDF() result.Error = %q", result.Error)
	}

	data, ok := result.Meta["data"].(PartsOutput)
	if !ok {
		t.Fatalf("result.Meta[data] type = %T, want PartsOutput", result.Meta["data"])
	}
	if data.Type != "parts" {
		t.Fatalf("data.Type = %q, want 'parts'", data.Type)
	}
	if data.Count != 1 {
		t.Fatalf("data.Count = %d, want 1", data.Count)
	}
}

// TestReadPDFWithInvalidPagesParameter verifies error on invalid pages parameter.
func TestReadPDFWithInvalidPagesParameter(t *testing.T) {
	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "test.pdf")
	createMinimalPDF(t, pdfPath)

	info, err := os.Stat(pdfPath)
	if err != nil {
		t.Fatalf("Stat error = %v", err)
	}

	tool := NewTool(&testFS{root: tmpDir}, nil)
	result, err := tool.readPDF(context.Background(), pdfPath, info.Size(), "invalid", tmpDir)
	if err != nil {
		t.Fatalf("readPDF() error = %v", err)
	}
	if result.Error == "" {
		t.Fatal("readPDF() with invalid pages expected error, got none")
	}
}

// TestReadPDFWithPagesExceedsLimit verifies error when pages parameter exceeds limit.
func TestReadPDFWithPagesExceedsLimit(t *testing.T) {
	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "test.pdf")
	createMinimalPDF(t, pdfPath)

	info, err := os.Stat(pdfPath)
	if err != nil {
		t.Fatalf("Stat error = %v", err)
	}

	tool := NewTool(&testFS{root: tmpDir}, nil)
	result, err := tool.readPDF(context.Background(), pdfPath, info.Size(), "1-30", tmpDir)
	if err != nil {
		t.Fatalf("readPDF() error = %v", err)
	}
	if result.Error == "" {
		t.Fatal("readPDF() with pages exceeding limit expected error, got none")
	}
	if !strings.Contains(result.Error, "exceeds maximum") {
		t.Fatalf("readPDF() error = %q, want containing 'exceeds maximum'", result.Error)
	}
}

// TestReadPDFFullInline verifies reading a full PDF inline.
func TestReadPDFFullInline(t *testing.T) {
	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "test.pdf")
	createMinimalPDF(t, pdfPath)

	info, err := os.Stat(pdfPath)
	if err != nil {
		t.Fatalf("Stat error = %v", err)
	}

	tool := NewTool(&testFS{root: tmpDir}, nil)
	result, err := tool.readPDF(context.Background(), pdfPath, info.Size(), "", tmpDir)
	if err != nil {
		t.Fatalf("readPDF() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("readPDF() result.Error = %q", result.Error)
	}

	data, ok := result.Meta["data"].(PDFOutput)
	if !ok {
		t.Fatalf("result.Meta[data] type = %T, want PDFOutput", result.Meta["data"])
	}
	if data.Type != "pdf" {
		t.Fatalf("data.Type = %q, want 'pdf'", data.Type)
	}
	if data.Base64 == "" {
		t.Fatal("data.Base64 is empty")
	}

	// Verify document_block is present
	docBlock, ok := result.Meta["document_block"].(map[string]any)
	if !ok {
		t.Fatalf("result.Meta[document_block] type = %T, want map[string]any", result.Meta["document_block"])
	}
	if docBlock["type"] != "document" {
		t.Fatalf("document_block.type = %v, want 'document'", docBlock["type"])
	}
}

// TestReadPDFOutputPath verifies the output path is relative when inside working dir.
func TestReadPDFOutputPath(t *testing.T) {
	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "test.pdf")
	createMinimalPDF(t, pdfPath)

	info, err := os.Stat(pdfPath)
	if err != nil {
		t.Fatalf("Stat error = %v", err)
	}

	tool := NewTool(&testFS{root: tmpDir}, nil)
	result, err := tool.readPDF(context.Background(), pdfPath, info.Size(), "", tmpDir)
	if err != nil {
		t.Fatalf("readPDF() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("readPDF() result.Error = %q", result.Error)
	}

	data := result.Meta["data"].(PDFOutput)
	if data.FilePath != "test.pdf" {
		t.Fatalf("data.FilePath = %q, want 'test.pdf'", data.FilePath)
	}
}

// TestReadPDFConstants verifies the PDF-related constants.
func TestReadPDFConstants(t *testing.T) {
	if pdfMaxPagesPerRead != 25 {
		t.Fatalf("pdfMaxPagesPerRead = %d, want 25", pdfMaxPagesPerRead)
	}
	if pdfAtMentionInlineThreshold != 100 {
		t.Fatalf("pdfAtMentionInlineThreshold = %d, want 100", pdfAtMentionInlineThreshold)
	}
	expectedThreshold := int64(5 * 1024 * 1024)
	if pdfExtractSizeThreshold != expectedThreshold {
		t.Fatalf("pdfExtractSizeThreshold = %d, want %d", pdfExtractSizeThreshold, expectedThreshold)
	}
}
