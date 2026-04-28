package file_read

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	platformfs "github.com/sheepzhao/claude-code-go/internal/platform/fs"
	sharedimage "github.com/sheepzhao/claude-code-go/internal/services/tools/shared/image"
)

// createTestPNG creates a simple PNG image with the specified dimensions.
func createTestPNG(width, height int) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	// Fill with a gradient pattern.
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
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// createTestJPEG creates a JPEG image with the specified dimensions.
func createTestJPEG(width, height int, quality int) ([]byte, error) {
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
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// createTestGIF creates a simple GIF image.
func createTestGIF(width, height int) ([]byte, error) {
	img := image.NewPaletted(image.Rect(0, 0, width, height), color.Palette{
		color.RGBA{255, 0, 0, 255},
		color.RGBA{0, 255, 0, 255},
		color.RGBA{0, 0, 255, 255},
	})
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetColorIndex(x, y, uint8((x+y)%3))
		}
	}
	var buf bytes.Buffer
	if err := gif.Encode(&buf, img, nil); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// TestReadImagePNG verifies PNG image reading and metadata extraction.
func TestReadImagePNG(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "test.png")

	data, err := createTestPNG(100, 80)
	if err != nil {
		t.Fatalf("createTestPNG() error = %v", err)
	}
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)
	result, err := tool.readImage(context.Background(), filePath, int64(len(data)), projectDir)
	if err != nil {
		t.Fatalf("readImage() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("readImage() result.Error = %q", result.Error)
	}

	imgOut, ok := result.Meta["data"].(ImageOutput)
	if !ok {
		t.Fatalf("readImage() meta data type = %T", result.Meta["data"])
	}
	if imgOut.Type != "image" {
		t.Fatalf("readImage() type = %q, want %q", imgOut.Type, "image")
	}
	if imgOut.MediaType != "image/jpeg" {
		t.Fatalf("readImage() mediaType = %q, want %q", imgOut.MediaType, "image/jpeg")
	}
	if imgOut.OriginalSize != len(data) {
		t.Fatalf("readImage() originalSize = %d, want %d", imgOut.OriginalSize, len(data))
	}
	if imgOut.OriginalWidth != 100 || imgOut.OriginalHeight != 80 {
		t.Fatalf("readImage() original dimensions = %dx%d, want 100x80", imgOut.OriginalWidth, imgOut.OriginalHeight)
	}
	if imgOut.Base64 == "" {
		t.Fatal("readImage() base64 is empty")
	}

	// Verify base64 is valid.
	decoded, err := base64.StdEncoding.DecodeString(imgOut.Base64)
	if err != nil {
		t.Fatalf("readImage() base64 decode error = %v", err)
	}
	if len(decoded) == 0 {
		t.Fatal("readImage() decoded base64 is empty")
	}

	// Output should contain summary.
	if !strings.HasPrefix(result.Output, "Read image (") {
		t.Fatalf("readImage() output = %q, want 'Read image (...' prefix", result.Output)
	}
}

// TestReadImageJPEG verifies JPEG image reading.
func TestReadImageJPEG(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "test.jpg")

	data, err := createTestJPEG(200, 150, 90)
	if err != nil {
		t.Fatalf("createTestJPEG() error = %v", err)
	}
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)
	result, err := tool.readImage(context.Background(), filePath, int64(len(data)), projectDir)
	if err != nil {
		t.Fatalf("readImage() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("readImage() result.Error = %q", result.Error)
	}

	imgOut, ok := result.Meta["data"].(ImageOutput)
	if !ok {
		t.Fatalf("readImage() meta data type = %T", result.Meta["data"])
	}
	if imgOut.OriginalWidth != 200 || imgOut.OriginalHeight != 150 {
		t.Fatalf("readImage() original dimensions = %dx%d, want 200x150", imgOut.OriginalWidth, imgOut.OriginalHeight)
	}
	if imgOut.Base64 == "" {
		t.Fatal("readImage() base64 is empty")
	}
}

// TestReadImageGIF verifies GIF image reading (converted to JPEG).
func TestReadImageGIF(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "test.gif")

	data, err := createTestGIF(64, 64)
	if err != nil {
		t.Fatalf("createTestGIF() error = %v", err)
	}
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)
	result, err := tool.readImage(context.Background(), filePath, int64(len(data)), projectDir)
	if err != nil {
		t.Fatalf("readImage() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("readImage() result.Error = %q", result.Error)
	}

	imgOut, ok := result.Meta["data"].(ImageOutput)
	if !ok {
		t.Fatalf("readImage() meta data type = %T", result.Meta["data"])
	}
	if imgOut.OriginalWidth != 64 || imgOut.OriginalHeight != 64 {
		t.Fatalf("readImage() original dimensions = %dx%d, want 64x64", imgOut.OriginalWidth, imgOut.OriginalHeight)
	}
	if imgOut.MediaType != "image/jpeg" {
		t.Fatalf("readImage() mediaType = %q, want %q", imgOut.MediaType, "image/jpeg")
	}
}

// TestReadImageLargeImageAutoResize verifies large images are automatically resized.
func TestReadImageLargeImageAutoResize(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "large.png")

	// Create a large image (2000x1800) that exceeds sharedimage.DefaultMaxImageDimension (1568).
	data, err := createTestPNG(2000, 1800)
	if err != nil {
		t.Fatalf("createTestPNG() error = %v", err)
	}
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)
	result, err := tool.readImage(context.Background(), filePath, int64(len(data)), projectDir)
	if err != nil {
		t.Fatalf("readImage() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("readImage() result.Error = %q", result.Error)
	}

	imgOut, ok := result.Meta["data"].(ImageOutput)
	if !ok {
		t.Fatalf("readImage() meta data type = %T", result.Meta["data"])
	}

	// Original dimensions should be preserved.
	if imgOut.OriginalWidth != 2000 || imgOut.OriginalHeight != 1800 {
		t.Fatalf("readImage() original dimensions = %dx%d, want 2000x1800", imgOut.OriginalWidth, imgOut.OriginalHeight)
	}

	// Display dimensions should be scaled down.
	if imgOut.DisplayWidth > sharedimage.DefaultMaxImageDimension {
		t.Fatalf("readImage() display width = %d, want <= %d", imgOut.DisplayWidth, sharedimage.DefaultMaxImageDimension)
	}
	if imgOut.DisplayHeight > sharedimage.DefaultMaxImageDimension {
		t.Fatalf("readImage() display height = %d, want <= %d", imgOut.DisplayHeight, sharedimage.DefaultMaxImageDimension)
	}

	// Aspect ratio should be preserved.
	origRatio := float64(2000) / float64(1800)
	displayRatio := float64(imgOut.DisplayWidth) / float64(imgOut.DisplayHeight)
	ratioDiff := math.Abs(origRatio - displayRatio)
	if ratioDiff > 0.01 {
		t.Fatalf("readImage() aspect ratio changed too much: original=%.4f, display=%.4f", origRatio, displayRatio)
	}
}

// TestReadImageEmptyFile verifies empty files return an error.
func TestReadImageEmptyFile(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "empty.png")

	if err := os.WriteFile(filePath, []byte{}, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)
	result, err := tool.readImage(context.Background(), filePath, 0, projectDir)
	if err != nil {
		t.Fatalf("readImage() error = %v", err)
	}
	if result.Error == "" {
		t.Fatal("readImage() expected error for empty file, got none")
	}
	if !strings.Contains(result.Error, "empty") {
		t.Fatalf("readImage() error = %q, want to contain 'empty'", result.Error)
	}
}

// TestReadImageOversizedFile verifies files exceeding max size are rejected.
func TestReadImageOversizedFile(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "huge.png")

	// Create a file that reports a large size but is actually small.
	data, err := createTestPNG(10, 10)
	if err != nil {
		t.Fatalf("createTestPNG() error = %v", err)
	}
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)
	// Pass an oversized size to trigger the size limit check.
	result, err := tool.readImage(context.Background(), filePath, defaultMaxImageFileSize+1, projectDir)
	if err != nil {
		t.Fatalf("readImage() error = %v", err)
	}
	if result.Error == "" {
		t.Fatal("readImage() expected error for oversized file, got none")
	}
	if !strings.Contains(result.Error, "exceeds maximum allowed size") {
		t.Fatalf("readImage() error = %q, want to contain 'exceeds maximum allowed size'", result.Error)
	}
}

// TestDetectImageFormat verifies format detection from magic bytes.
func TestDetectImageFormat(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected string
	}{
		{
			name:     "PNG",
			data:     []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a},
			expected: "image/png",
		},
		{
			name:     "JPEG",
			data:     []byte{0xff, 0xd8, 0xff, 0xe0},
			expected: "image/jpeg",
		},
		{
			name:     "GIF",
			data:     []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61},
			expected: "image/gif",
		},
		{
			name:     "WebP",
			data:     []byte{0x52, 0x49, 0x46, 0x46, 0x00, 0x00, 0x00, 0x00, 0x57, 0x45, 0x42, 0x50},
			expected: "image/webp",
		},
		{
			name:     "too short",
			data:     []byte{0x89, 0x50},
			expected: "image/png", // default
		},
		{
			name:     "unknown",
			data:     []byte{0x00, 0x01, 0x02, 0x03},
			expected: "image/png", // default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sharedimage.DetectFormat(tt.data)
			if got != tt.expected {
				t.Fatalf("sharedimage.DetectFormat() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestScaleDimensions verifies aspect ratio preservation during scaling.
func TestScaleDimensions(t *testing.T) {
	tests := []struct {
		width      int
		height     int
		maxWidth   int
		maxHeight  int
		wantWidth  int
		wantHeight int
	}{
		{100, 80, 50, 50, 50, 40},       // Scale by width
		{80, 100, 50, 50, 40, 50},       // Scale by height
		{100, 100, 200, 200, 100, 100},  // No scaling needed
		{2000, 1000, 1568, 1568, 1568, 784}, // Large image
		{1000, 2000, 1568, 1568, 784, 1568}, // Tall image
		{0, 100, 50, 50, 50, 50},        // Zero width fallback
		{100, 0, 50, 50, 50, 50},        // Zero height fallback
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%dx%d_to_%dx%d", tt.width, tt.height, tt.maxWidth, tt.maxHeight), func(t *testing.T) {
			gotW, gotH := sharedimage.ScaleDimensions(tt.width, tt.height, tt.maxWidth, tt.maxHeight)
			if gotW != tt.wantWidth || gotH != tt.wantHeight {
				t.Fatalf("sharedimage.ScaleDimensions() = %dx%d, want %dx%d", gotW, gotH, tt.wantWidth, tt.wantHeight)
			}
		})
	}
}

// TestReadImageCompression verifies that very large images trigger compression.
func TestReadImageCompression(t *testing.T) {
	projectDir := t.TempDir()
	filePath := filepath.Join(projectDir, "compress.png")

	// Create a very large image that will definitely exceed token budget.
	data, err := createTestPNG(3000, 3000)
	if err != nil {
		t.Fatalf("createTestPNG() error = %v", err)
	}
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	policy, err := corepermission.NewFilesystemPolicy(corepermission.RuleSet{})
	if err != nil {
		t.Fatalf("NewFilesystemPolicy() error = %v", err)
	}

	tool := NewTool(platformfs.NewLocalFS(), policy)
	result, err := tool.readImage(context.Background(), filePath, int64(len(data)), projectDir)
	if err != nil {
		t.Fatalf("readImage() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("readImage() result.Error = %q", result.Error)
	}

	imgOut, ok := result.Meta["data"].(ImageOutput)
	if !ok {
		t.Fatalf("readImage() meta data type = %T", result.Meta["data"])
	}

	// Should have been resized down.
	if imgOut.DisplayWidth > sharedimage.DefaultMaxImageDimension {
		t.Fatalf("readImage() display width = %d, want <= %d", imgOut.DisplayWidth, sharedimage.DefaultMaxImageDimension)
	}
	if imgOut.DisplayHeight > sharedimage.DefaultMaxImageDimension {
		t.Fatalf("readImage() display height = %d, want <= %d", imgOut.DisplayHeight, sharedimage.DefaultMaxImageDimension)
	}

	// Base64 should be non-empty.
	if imgOut.Base64 == "" {
		t.Fatal("readImage() base64 is empty")
	}

	// Verify the base64 decodes to a valid JPEG.
	decoded, err := base64.StdEncoding.DecodeString(imgOut.Base64)
	if err != nil {
		t.Fatalf("base64 decode error = %v", err)
	}
	decodedImg, err := jpeg.Decode(bytes.NewReader(decoded))
	if err != nil {
		t.Fatalf("jpeg decode error = %v", err)
	}
	bounds := decodedImg.Bounds()
	if bounds.Dx() != imgOut.DisplayWidth || bounds.Dy() != imgOut.DisplayHeight {
		t.Fatalf("decoded dimensions = %dx%d, want %dx%d", bounds.Dx(), bounds.Dy(), imgOut.DisplayWidth, imgOut.DisplayHeight)
	}
}
