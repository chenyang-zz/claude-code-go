package file_read

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	platformfs "github.com/sheepzhao/claude-code-go/internal/platform/fs"
)

// parsePDFPageRange parses a page range string into firstPage and lastPage numbers.
// Supported formats:
//   - "3"       → firstPage=3, lastPage=3
//   - "1-5"     → firstPage=1, lastPage=5
//   - "3-"      → firstPage=3, lastPage=0 (open-ended, caller decides)
//
// Pages are 1-indexed. Returns error on invalid input.
func parsePDFPageRange(pages string) (firstPage int, lastPage int, err error) {
	trimmed := strings.TrimSpace(pages)
	if trimmed == "" {
		return 0, 0, fmt.Errorf("empty page range")
	}

	// Open-ended range: "N-"
	if strings.HasSuffix(trimmed, "-") {
		first, parseErr := strconv.Atoi(trimmed[:len(trimmed)-1])
		if parseErr != nil || first < 1 {
			return 0, 0, fmt.Errorf("invalid page range: %q", pages)
		}
		return first, 0, nil
	}

	dashIndex := strings.Index(trimmed, "-")
	if dashIndex == -1 {
		// Single page: "5"
		page, parseErr := strconv.Atoi(trimmed)
		if parseErr != nil || page < 1 {
			return 0, 0, fmt.Errorf("invalid page range: %q", pages)
		}
		return page, page, nil
	}

	// Range: "1-10"
	first, err1 := strconv.Atoi(trimmed[:dashIndex])
	last, err2 := strconv.Atoi(trimmed[dashIndex+1:])
	if err1 != nil || err2 != nil || first < 1 || last < 1 || last < first {
		return 0, 0, fmt.Errorf("invalid page range: %q", pages)
	}

	pageCount := last - first + 1
	if pageCount > pdfMaxPagesPerRead {
		return 0, 0, fmt.Errorf(
			"page range %q exceeds maximum of %d pages per request (requested %d pages)",
			pages, pdfMaxPagesPerRead, pageCount,
		)
	}

	return first, last, nil
}

// isPDFSupported checks whether the current environment supports PDF document blocks.
// In the Go migration, we assume PDF support is available (equivalent to not being
// on Claude 3 Haiku). The caller can override this behavior if needed.
func isPDFSupported() bool {
	return true
}

// isPdftoppmAvailable checks whether the pdftoppm binary (from poppler-utils) is available.
func isPdftoppmAvailable() bool {
	_, err := exec.LookPath("pdftoppm")
	return err == nil
}

// isPdfinfoAvailable checks whether the pdfinfo binary (from poppler-utils) is available.
func isPdfinfoAvailable() bool {
	_, err := exec.LookPath("pdfinfo")
	return err == nil
}

// getPDFPageCount returns the number of pages in a PDF file using pdfinfo.
// Returns 0 if pdfinfo is not available or the page count cannot be determined.
func getPDFPageCount(filePath string) int {
	if !isPdfinfoAvailable() {
		return 0
	}

	cmd := exec.Command("pdfinfo", filePath)
	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	// Parse "Pages: N" from output
	re := regexp.MustCompile(`(?m)^Pages:\s+(\d+)`)
	matches := re.FindSubmatch(output)
	if matches == nil {
		return 0
	}

	count, err := strconv.Atoi(string(matches[1]))
	if err != nil {
		return 0
	}
	return count
}

// extractPDFPages extracts PDF pages as JPEG images using pdftoppm.
// Produces page-01.jpg, page-02.jpg, etc. in an output directory.
func extractPDFPages(filePath string, firstPage, lastPage int) (outputDir string, count int, err error) {
	if !isPdftoppmAvailable() {
		return "", 0, fmt.Errorf("pdftoppm is not installed. Install poppler-utils (e.g. `brew install poppler` or `apt-get install poppler-utils`) to enable PDF page rendering")
	}

	// Create temporary directory for extracted images
	outputDir, err = os.MkdirTemp("", "pdf-extract-*")
	if err != nil {
		return "", 0, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// pdftoppm produces files like <prefix>-01.jpg, <prefix>-02.jpg, etc.
	prefix := filepath.Join(outputDir, "page")

	args := []string{"-jpeg", "-r", "100"}
	if firstPage > 0 {
		args = append(args, "-f", strconv.Itoa(firstPage))
	}
	if lastPage > 0 {
		args = append(args, "-l", strconv.Itoa(lastPage))
	}
	args = append(args, filePath, prefix)

	cmd := exec.Command("pdftoppm", args...)
	stderr, execErr := cmd.CombinedOutput()
	if execErr != nil {
		_ = os.RemoveAll(outputDir)
		stderrStr := string(stderr)
		if strings.Contains(stderrStr, "password") {
			return "", 0, fmt.Errorf("PDF is password-protected. Please provide an unprotected version")
		}
		if strings.Contains(stderrStr, "damaged") || strings.Contains(stderrStr, "corrupt") || strings.Contains(stderrStr, "invalid") {
			return "", 0, fmt.Errorf("PDF file is corrupted or invalid")
		}
		return "", 0, fmt.Errorf("pdftoppm failed: %s", stderrStr)
	}

	// Read generated image files and count
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		_ = os.RemoveAll(outputDir)
		return "", 0, fmt.Errorf("failed to read output directory: %w", err)
	}

	var imageFiles []string
	for _, entry := range entries {
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".jpg") {
			imageFiles = append(imageFiles, entry.Name())
		}
	}

	if len(imageFiles) == 0 {
		_ = os.RemoveAll(outputDir)
		return "", 0, fmt.Errorf("pdftoppm produced no output pages. The PDF may be invalid")
	}

	sort.Strings(imageFiles)
	return outputDir, len(imageFiles), nil
}

// readPDF reads a PDF file. If pages is non-empty, it extracts the specified
// page range as images. Otherwise it reads the full PDF inline when supported.
func (t *Tool) readPDF(ctx context.Context, filePath string, size int64, pages string, workingDir string) (coretool.Result, error) {
	relativePath := platformfs.ToRelativePath(filePath, workingDir)

	// Check if file is empty
	if size == 0 {
		return coretool.Result{Error: fmt.Sprintf("PDF file is empty: %s", relativePath)}, nil
	}

	// If pages parameter is provided, extract those pages as images
	if pages != "" {
		firstPage, lastPage, err := parsePDFPageRange(pages)
		if err != nil {
			return coretool.Result{Error: err.Error()}, nil
		}

		outputDir, count, err := extractPDFPages(filePath, firstPage, lastPage)
		if err != nil {
			return coretool.Result{Error: err.Error()}, nil
		}
		defer func() {
			_ = os.RemoveAll(outputDir)
		}()

		// Read extracted images and convert to base64
		entries, err := os.ReadDir(outputDir)
		if err != nil {
			return coretool.Result{Error: fmt.Sprintf("failed to read extracted pages: %v", err)}, nil
		}

		var imageFiles []string
		for _, entry := range entries {
			if strings.HasSuffix(strings.ToLower(entry.Name()), ".jpg") {
				imageFiles = append(imageFiles, entry.Name())
			}
		}
		sort.Strings(imageFiles)

		// Build image blocks from extracted pages
		var imageBlocks []map[string]any
		var images []coretool.ImageData
		for _, imgFile := range imageFiles {
			imgPath := filepath.Join(outputDir, imgFile)
			imgData, err := os.ReadFile(imgPath)
			if err != nil {
				return coretool.Result{Error: fmt.Sprintf("failed to read extracted image %s: %v", imgFile, err)}, nil
			}

			// For now, return the JPEG directly. Image resize/compression will be
			// handled by the image reading pipeline when it is implemented.
			base64Data := base64.StdEncoding.EncodeToString(imgData)
			imageBlocks = append(imageBlocks, map[string]any{
				"type": "image",
				"source": map[string]any{
					"type":       "base64",
					"media_type": "image/jpeg",
					"data":       base64Data,
				},
			})
			images = append(images, coretool.ImageData{
				MediaType: "image/jpeg",
				Base64:    base64Data,
			})
		}

		output := PartsOutput{
			Type:         "parts",
			FilePath:     relativePath,
			OriginalSize: int(size),
			Count:        count,
			OutputDir:    outputDir,
		}

		result := coretool.Result{
			Output: fmt.Sprintf("Extracted %d pages from PDF %s", count, relativePath),
			Meta: map[string]any{
				"data": output,
			},
		}

		// Include image blocks as meta message data if any were produced.
		// "image_blocks" is the legacy raw-map representation retained for
		// backwards-compatible tests; "images" is the typed key consumed by
		// the runtime engine to inject ImagePart blocks alongside tool_result.
		if len(imageBlocks) > 0 {
			result.Meta["image_blocks"] = imageBlocks
			result.Meta["images"] = images
		}

		return result, nil
	}

	// No pages specified — read full PDF
	pageCount := getPDFPageCount(filePath)
	if pageCount > pdfAtMentionInlineThreshold {
		return coretool.Result{
			Error: fmt.Sprintf(
				"This PDF has %d pages, which is too many to read at once. "+
					"Use the pages parameter to read specific page ranges (e.g., pages: \"1-5\"). "+
					"Maximum %d pages per request.",
				pageCount, pdfMaxPagesPerRead,
			),
		}, nil
	}

	shouldExtractPages := !isPDFSupported() || size > pdfExtractSizeThreshold

	if shouldExtractPages {
		outputDir, count, err := extractPDFPages(filePath, 0, 0)
		if err != nil {
			// Extraction failed but we may still be able to read inline
			// Continue to try inline read
			_ = outputDir
			_ = count
		} else {
			defer func() {
				_ = os.RemoveAll(outputDir)
			}()

			// Read extracted images
			entries, err := os.ReadDir(outputDir)
			if err == nil {
				var imageFiles []string
				for _, entry := range entries {
					if strings.HasSuffix(strings.ToLower(entry.Name()), ".jpg") {
						imageFiles = append(imageFiles, entry.Name())
					}
				}
				sort.Strings(imageFiles)

				var imageBlocks []map[string]any
				var images []coretool.ImageData
				for _, imgFile := range imageFiles {
					imgPath := filepath.Join(outputDir, imgFile)
					imgData, err := os.ReadFile(imgPath)
					if err != nil {
						continue
					}
					base64Data := base64.StdEncoding.EncodeToString(imgData)
					imageBlocks = append(imageBlocks, map[string]any{
						"type": "image",
						"source": map[string]any{
							"type":       "base64",
							"media_type": "image/jpeg",
							"data":       base64Data,
						},
					})
					images = append(images, coretool.ImageData{
						MediaType: "image/jpeg",
						Base64:    base64Data,
					})
				}

				output := PartsOutput{
					Type:         "parts",
					FilePath:     relativePath,
					OriginalSize: int(size),
					Count:        count,
					OutputDir:    outputDir,
				}

				result := coretool.Result{
					Output: fmt.Sprintf("Extracted %d pages from PDF %s", count, relativePath),
					Meta: map[string]any{
						"data": output,
					},
				}
				if len(imageBlocks) > 0 {
					result.Meta["image_blocks"] = imageBlocks
					result.Meta["images"] = images
				}
				return result, nil
			}
		}
	}

	if !isPDFSupported() {
		return coretool.Result{
			Error: fmt.Sprintf(
				"Reading full PDFs is not supported with this model. Use a newer model (Sonnet 3.5 v2 or later), "+
					"or use the pages parameter to read specific page ranges (e.g., pages: \"1-5\", maximum %d pages per request). "+
					"Page extraction requires poppler-utils: install with `brew install poppler` on macOS or `apt-get install poppler-utils` on Debian/Ubuntu.",
				pdfMaxPagesPerRead,
			),
		}, nil
	}

	// Read full PDF as base64
	fileData, err := t.fs.ReadFile(filePath)
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("Failed to read PDF file: %v", err)}, nil
	}

	// Validate PDF magic bytes
	if len(fileData) < 5 || string(fileData[:5]) != "%PDF-" {
		return coretool.Result{Error: fmt.Sprintf("File is not a valid PDF (missing %%PDF- header): %s", relativePath)}, nil
	}

	base64Data := base64.StdEncoding.EncodeToString(fileData)

	output := PDFOutput{
		Type:         "pdf",
		FilePath:     relativePath,
		Base64:       base64Data,
		OriginalSize: int(size),
	}

	// Build document block for meta message.
	// "document_block" is the legacy raw-map representation retained for
	// backwards-compatible tests; "document" is the typed key consumed by the
	// runtime engine to inject a DocumentPart block alongside tool_result.
	documentBlock := map[string]any{
		"type": "document",
		"source": map[string]any{
			"type":       "base64",
			"media_type": "application/pdf",
			"data":       base64Data,
		},
	}

	return coretool.Result{
		Output: fmt.Sprintf("Read PDF file %s (%s)", relativePath, formatByteSize(size)),
		Meta: map[string]any{
			"data":           output,
			"document_block": documentBlock,
			"document": coretool.DocumentData{
				MediaType: "application/pdf",
				Base64:    base64Data,
			},
		},
	}, nil
}
