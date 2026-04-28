package bash

import (
	"encoding/base64"
	"fmt"
	"image/jpeg"
	"os"
	"regexp"
	"strings"

	sharedimage "github.com/sheepzhao/claude-code-go/internal/services/tools/shared/image"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// dataURIPattern matches a base64-encoded image data URI in shell output.
// Format: data:image/<type>;base64,<data>
var dataURIPattern = regexp.MustCompile(`^data:image/[a-z0-9.+_-]+;base64,`)

// maxImageFileReadSize caps the file read for image processing to 20 MB.
// Data URIs larger than this are well beyond what the API accepts (5 MB base64).
const maxImageFileReadSize = 20 * 1024 * 1024

// dataURIParseRe extracts media type and base64 payload from a data URI.
var dataURIParseRe = regexp.MustCompile(`^data:([^;]+);base64,(.+)$`)

// isImageOutput checks if the stdout content is a base64-encoded image data URI.
func isImageOutput(content string) bool {
	return dataURIPattern.MatchString(content)
}

// parsedDataURI holds the components parsed from a data URI.
type parsedDataURI struct {
	mediaType string
	data      string
}

// parseDataURI extracts the media type and base64 data from a data URI string.
// Returns nil if the string does not match the expected data URI format.
func parseDataURI(s string) *parsedDataURI {
	trimmed := strings.TrimSpace(s)
	matches := dataURIParseRe.FindStringSubmatch(trimmed)
	if len(matches) != 3 || matches[1] == "" || matches[2] == "" {
		return nil
	}
	return &parsedDataURI{
		mediaType: matches[1],
		data:      matches[2],
	}
}

// resizeBashImageOutput resizes image output from a shell command. It first
// tries to use the stdout data URI, but if the output was persisted to a file
// (outputFilePath is set), it reads the full image data from disk to avoid
// corruption from truncated base64. Returns the resized data URI string and
// whether the result is still an image. If parsing or processing fails
// gracefully, it returns the original stdout and false so the caller can
// fall through to text rendering.
func resizeBashImageOutput(stdout string, outputFilePath string, outputFileSize int64) (string, bool) {
	source := stdout

	// If the output was persisted to disk, re-read the full data URI
	// from the file to avoid truncated base64 corruption.
	if outputFilePath != "" {
		size := outputFileSize
		if size <= 0 {
			info, err := os.Stat(outputFilePath)
			if err != nil {
				logger.DebugCF("bash_tool", "failed to stat output file for image resize", map[string]any{
					"output_file": outputFilePath,
					"error":       err.Error(),
				})
				return stdout, false
			}
			size = info.Size()
		}
		if size > maxImageFileReadSize {
			logger.DebugCF("bash_tool", "image output file too large for resize", map[string]any{
				"output_file": outputFilePath,
				"size":        size,
			})
			return stdout, false
		}
		data, err := os.ReadFile(outputFilePath)
		if err != nil {
			logger.DebugCF("bash_tool", "failed to read output file for image resize", map[string]any{
				"output_file": outputFilePath,
				"error":       err.Error(),
			})
			return stdout, false
		}
		source = string(data)
	}

	parsed := parseDataURI(source)
	if parsed == nil {
		return stdout, false
	}

	// Decode base64 to raw bytes.
	buf, err := base64.StdEncoding.DecodeString(parsed.data)
	if err != nil {
		logger.DebugCF("bash_tool", "failed to decode base64 image data", map[string]any{
			"error": err.Error(),
		})
		return stdout, false
	}

	// Extract format extension from media type (e.g., "image/png" -> "png").
	ext := "png"
	if parts := strings.SplitN(parsed.mediaType, "/", 2); len(parts) == 2 {
		ext = parts[1]
	}
	// Normalize "jpeg" ext for media type compatibility (shared image detects
	// via magic bytes, but the media type is used for the final data URI).
	if ext == "jpg" {
		ext = "jpeg"
	}

	// Detect actual format from magic bytes.
	mediaType := sharedimage.DetectFormat(buf)

	// Decode the image.
	img, originalWidth, originalHeight, err := sharedimage.Decode(buf, mediaType)
	if err != nil {
		logger.DebugCF("bash_tool", "failed to decode image for resize", map[string]any{
			"error":      err.Error(),
			"media_type": mediaType,
			"size":       len(buf),
		})
		return stdout, false
	}

	// Apply standard resize: constrain to max dimension.
	displayWidth, displayHeight := originalWidth, originalHeight
	if originalWidth > sharedimage.DefaultMaxImageDimension || originalHeight > sharedimage.DefaultMaxImageDimension {
		displayWidth, displayHeight = sharedimage.ScaleDimensions(
			originalWidth, originalHeight,
			sharedimage.DefaultMaxImageDimension, sharedimage.DefaultMaxImageDimension,
		)
		img = sharedimage.Resize(img, displayWidth, displayHeight)
	}

	// Encode as JPEG quality 80 and check token budget.
	base64Str, err := sharedimage.EncodeToBase64(img, jpeg.Options{Quality: 80})
	if err != nil {
		logger.DebugCF("bash_tool", "failed to encode image to base64", map[string]any{
			"error": err.Error(),
		})
		return stdout, false
	}

	maxTokens := sharedimage.DefaultImageMaxTokens

	if sharedimage.EstimatedTokens(base64Str) > maxTokens {
		compressed, w, h, err := sharedimage.CompressWithTokenLimit(
			img, originalWidth, originalHeight, maxTokens,
		)
		if err != nil {
			// Fallback: 400x400 JPEG quality 20.
			fbW, fbH := sharedimage.ScaleDimensions(originalWidth, originalHeight, 400, 400)
			fbImg := sharedimage.Resize(img, fbW, fbH)
			compressed, err = sharedimage.EncodeToBase64(fbImg, jpeg.Options{Quality: 20})
			if err != nil {
				logger.DebugCF("bash_tool", "failed to compress image", map[string]any{
					"error": err.Error(),
				})
				return stdout, false
			}
			displayWidth, displayHeight = fbW, fbH
		} else {
			displayWidth, displayHeight = w, h
		}
		base64Str = compressed
	}

	// Build the resized data URI using the original output format extension
	// for the media type, but always JPEG since we re-encode as JPEG.
	return fmt.Sprintf("data:image/jpeg;base64,%s", base64Str), true
}

// imageOutputFromStdout extracts a shared ImageOutput from image data URI stdout.
// Returns nil if the stdout is not a valid data URI.
func imageOutputFromStdout(stdout string, originalFileSize int64) *sharedimage.Output {
	parsed := parseDataURI(stdout)
	if parsed == nil {
		return nil
	}

	data, err := base64.StdEncoding.DecodeString(parsed.data)
	if err != nil {
		return nil
	}

	mediaType := sharedimage.DetectFormat(data)
	_, width, height, err := sharedimage.Decode(data, mediaType)
	if err != nil {
		// Return partial output even if decode fails (can't get dimensions).
		return &sharedimage.Output{
			Type:         "image",
			Base64:       parsed.data,
			MediaType:    parsed.mediaType,
			OriginalSize: int(originalFileSize),
		}
	}

	return &sharedimage.Output{
		Type:           "image",
		Base64:         parsed.data,
		MediaType:      parsed.mediaType,
		OriginalSize:   int(originalFileSize),
		OriginalWidth:  width,
		OriginalHeight: height,
		DisplayWidth:   width,
		DisplayHeight:  height,
	}
}
