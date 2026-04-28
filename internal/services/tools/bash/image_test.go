package bash

import (
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// createTestPNGBytes creates a simple PNG image and returns it as bytes.
func createTestPNGBytes(width, height int) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8(x % 256),
				G: uint8(y % 256),
				B: uint8((x + y) % 256),
				A: 255,
			})
		}
	}
	var buf []byte
	w := &byteBuffer{buf: &buf}
	if err := png.Encode(w, img); err != nil {
		return nil, err
	}
	return buf, nil
}

// byteBuffer implements io.Writer over a byte slice pointer.
type byteBuffer struct {
	buf *[]byte
}

func (b *byteBuffer) Write(p []byte) (int, error) {
	*b.buf = append(*b.buf, p...)
	return len(p), nil
}

// createTestDataURI creates a base64 data URI from raw image bytes.
func createTestDataURI(data []byte, mediaType string) string {
	b64 := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mediaType, b64)
}

// TestIsImageOutput verifies detection of image data URIs in stdout.
func TestIsImageOutput(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "PNG data URI",
			content:  "data:image/png;base64,iVBORw0KGgoAAAANSUhEUg==",
			expected: true,
		},
		{
			name:     "JPEG data URI",
			content:  "data:image/jpeg;base64,/9j/4AAQSkZJRg==",
			expected: true,
		},
		{
			name:     "GIF data URI",
			content:  "data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7",
			expected: true,
		},
		{
			name:     "WebP data URI",
			content:  "data:image/webp;base64,UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA",
			expected: true,
		},
		{
			name:     "plain text",
			content:  "file1.txt\nfile2.txt",
			expected: false,
		},
		{
			name:     "empty string",
			content:  "",
			expected: false,
		},
		{
			name:     "not a data URI (HTTP URL)",
			content:  "https://example.com/image.png",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isImageOutput(tt.content)
			if got != tt.expected {
				t.Fatalf("isImageOutput() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestParseDataURI verifies parsing of data URI components.
func TestParseDataURI(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantMediaType string
		wantData      string
		wantNil       bool
	}{
		{
			name:          "valid PNG data URI",
			input:         "data:image/png;base64,ABC123",
			wantMediaType: "image/png",
			wantData:      "ABC123",
		},
		{
			name:          "valid JPEG data URI with padding",
			input:         "  data:image/jpeg;base64,/9j/4Q==  ",
			wantMediaType: "image/jpeg",
			wantData:      "/9j/4Q==",
		},
		{
			name:    "plain text",
			input:   "hello world",
			wantNil: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantNil: true,
		},
		{
			name:    "incomplete data URI (no base64 part)",
			input:   "data:image/png;base64,",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDataURI(tt.input)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("parseDataURI() = %+v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("parseDataURI() = nil, want non-nil")
			}
			if got.mediaType != tt.wantMediaType {
				t.Fatalf("parseDataURI().mediaType = %q, want %q", got.mediaType, tt.wantMediaType)
			}
			if got.data != tt.wantData {
				t.Fatalf("parseDataURI().data = %q, want %q", got.data, tt.wantData)
			}
		})
	}
}

// TestResizeBashImageOutput verifies image resize from data URI stdout.
func TestResizeBashImageOutput(t *testing.T) {
	// Create a small test PNG image and encode as data URI.
	data, err := createTestPNGBytes(100, 80)
	if err != nil {
		t.Fatalf("createTestPNGBytes() error = %v", err)
	}
	dataURI := createTestDataURI(data, "image/png")

	resized, isImage := resizeBashImageOutput(dataURI, "", 0)
	if !isImage {
		t.Fatal("resizeBashImageOutput() isImage = false, want true")
	}
	if !strings.HasPrefix(resized, "data:image/jpeg;base64,") {
		t.Fatalf("resizeBashImageOutput() output = %q, want prefix 'data:image/jpeg;base64,'", resized[:50])
	}

	// Decode and verify the resized image.
	parsed := parseDataURI(resized)
	if parsed == nil {
		t.Fatal("parseDataURI() on resized output = nil")
	}
	decoded, err := base64.StdEncoding.DecodeString(parsed.data)
	if err != nil {
		t.Fatalf("base64 decode error = %v", err)
	}
	decodedImg, err := jpeg.Decode(&byteReader{data: decoded})
	if err != nil {
		t.Fatalf("jpeg decode error = %v", err)
	}
	bounds := decodedImg.Bounds()
	if bounds.Dx() != 100 || bounds.Dy() != 80 {
		t.Fatalf("resized dimensions = %dx%d, want 100x80", bounds.Dx(), bounds.Dy())
	}
}

// byteReader implements io.Reader over a byte slice for image decoding.
type byteReader struct {
	data []byte
	pos  int
}

func (r *byteReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, fmt.Errorf("EOF")
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// TestResizeBashImageOutputPlainText verifies plain text isn't treated as image.
func TestResizeBashImageOutputPlainText(t *testing.T) {
	stdout := "total 42\ndrwxr-xr-x  3 user  staff  96 Jan 1 12:00 dir1\n"
	resized, isImage := resizeBashImageOutput(stdout, "", 0)
	if isImage {
		t.Fatal("resizeBashImageOutput() isImage = true for plain text, want false")
	}
	if resized != stdout {
		t.Fatalf("resizeBashImageOutput() = %q, want original stdout", resized)
	}
}

// TestResizeBashImageOutputFromFile verifies image resize re-reads from file.
func TestResizeBashImageOutputFromFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "output.txt")

	data, err := createTestPNGBytes(50, 50)
	if err != nil {
		t.Fatalf("createTestPNGBytes() error = %v", err)
	}
	dataURI := createTestDataURI(data, "image/png")
	if err := os.WriteFile(filePath, []byte(dataURI), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Pass empty stdout but valid output file path.
	resized, isImage := resizeBashImageOutput("", filePath, int64(len(dataURI)))
	if !isImage {
		t.Fatal("resizeBashImageOutput() from file: isImage = false, want true")
	}
	if !strings.HasPrefix(resized, "data:image/jpeg;base64,") {
		t.Fatalf("resizeBashImageOutput() from file: unexpected result prefix")
	}
}

// TestResizeBashImageOutputInvalidBase64 verifies invalid base64 gracefully fails.
func TestResizeBashImageOutputInvalidBase64(t *testing.T) {
	invalidURI := "data:image/png;base64,!!!not-valid-base64!!!"
	resized, isImage := resizeBashImageOutput(invalidURI, "", 0)
	if isImage {
		t.Fatal("resizeBashImageOutput() isImage = true for invalid base64, want false")
	}
	if resized != invalidURI {
		t.Fatalf("resizeBashImageOutput() = %q, want original on failure", resized)
	}
}

// TestProcessImageOutput verifies the image output processing pipeline.
func TestProcessImageOutput(t *testing.T) {
	// Create a small PNG data URI.
	data, err := createTestPNGBytes(64, 64)
	if err != nil {
		t.Fatalf("createTestPNGBytes() error = %v", err)
	}
	dataURI := createTestDataURI(data, "image/png")

	output := &Output{
		Stdout: dataURI,
	}
	richMeta := processImageOutput(output, "")

	if !richMeta.IsImage {
		t.Fatal("processImageOutput() IsImage = false, want true")
	}
	if richMeta.ImageData == nil {
		t.Fatal("processImageOutput() ImageData = nil")
	}
	if richMeta.ImageData.Type != "image" {
		t.Fatalf("processImageOutput() ImageData.Type = %q, want 'image'", richMeta.ImageData.Type)
	}
	if output.Stdout == dataURI {
		t.Fatal("processImageOutput() did not update stdout")
	}
	if !strings.HasPrefix(output.Stdout, "data:image/jpeg;base64,") {
		t.Fatalf("processImageOutput() stdout not a JPEG data URI")
	}
}

// TestProcessImageOutputPlainText verifies plain text passes through unchanged.
func TestProcessImageOutputPlainText(t *testing.T) {
	stdout := "Hello, world!"
	output := &Output{
		Stdout: stdout,
	}
	richMeta := processImageOutput(output, "")

	if richMeta.IsImage {
		t.Fatal("processImageOutput() IsImage = true for plain text, want false")
	}
	if output.Stdout != stdout {
		t.Fatalf("processImageOutput() modified stdout for plain text")
	}
}

// TestImageOutputFromStdout verifies extraction of image metadata from data URI.
func TestImageOutputFromStdout(t *testing.T) {
	data, err := createTestPNGBytes(30, 40)
	if err != nil {
		t.Fatalf("createTestPNGBytes() error = %v", err)
	}
	dataURI := createTestDataURI(data, "image/png")

	imgOut := imageOutputFromStdout(dataURI, int64(len(data)))
	if imgOut == nil {
		t.Fatal("imageOutputFromStdout() = nil")
	}
	if imgOut.Type != "image" {
		t.Fatalf("imageOutputFromStdout().Type = %q, want 'image'", imgOut.Type)
	}
	if imgOut.MediaType != "image/png" {
		t.Fatalf("imageOutputFromStdout().MediaType = %q, want 'image/png'", imgOut.MediaType)
	}
	if imgOut.OriginalWidth != 30 || imgOut.OriginalHeight != 40 {
		t.Fatalf("imageOutputFromStdout() dimensions = %dx%d, want 30x40", imgOut.OriginalWidth, imgOut.OriginalHeight)
	}
}

// TestImageOutputFromStdoutInvalid verifies invalid data URI returns nil.
func TestImageOutputFromStdoutInvalid(t *testing.T) {
	imgOut := imageOutputFromStdout("not an image", 0)
	if imgOut != nil {
		t.Fatalf("imageOutputFromStdout() = %+v, want nil", imgOut)
	}
}

// TestResizeBashImageOutputLargeImage verifies large image resize behavior.
func TestResizeBashImageOutputLargeImage(t *testing.T) {
	data, err := createTestPNGBytes(2000, 1800)
	if err != nil {
		t.Fatalf("createTestPNGBytes() error = %v", err)
	}
	dataURI := createTestDataURI(data, "image/png")

	resized, isImage := resizeBashImageOutput(dataURI, "", 0)
	if !isImage {
		t.Fatal("resizeBashImageOutput() isImage = false for large image, want true")
	}

	// Verify it was resized to within max dimension.
	parsed := parseDataURI(resized)
	if parsed == nil {
		t.Fatal("parseDataURI() on resized = nil")
	}
	decoded, err := base64.StdEncoding.DecodeString(parsed.data)
	if err != nil {
		t.Fatalf("base64 decode error = %v", err)
	}
	decodedImg, err := jpeg.Decode(&byteReader{data: decoded})
	if err != nil {
		t.Fatalf("jpeg decode error = %v", err)
	}
	bounds := decodedImg.Bounds()
	if bounds.Dx() > 1568 || bounds.Dy() > 1568 {
		t.Fatalf("resized image %dx%d exceeds max dimension 1568", bounds.Dx(), bounds.Dy())
	}
}
