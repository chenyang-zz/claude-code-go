package file_read

import (
	"context"
	"fmt"
	"image/jpeg"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	sharedimage "github.com/sheepzhao/claude-code-go/internal/services/tools/shared/image"
)

const (
	// defaultMaxImageFileSize is the maximum raw image file size (20MB).
	defaultMaxImageFileSize = 20 * 1024 * 1024
)

// readImage reads an image file, detects its format, applies resize/compression
// as needed, and returns a base64-encoded image result.
func (t *Tool) readImage(ctx context.Context, filePath string, size int64, workingDir string) (coretool.Result, error) {
	if size == 0 {
		return coretool.Result{Error: fmt.Sprintf("Image file is empty: %s", filePath)}, nil
	}

	if size > defaultMaxImageFileSize {
		return coretool.Result{Error: fmt.Sprintf(
			"Image file (%s) exceeds maximum allowed size (%s)",
			formatByteSize(size),
			formatByteSize(defaultMaxImageFileSize),
		)}, nil
	}

	// Read file once, capped to max size.
	imageData, err := t.fs.ReadFile(filePath)
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("Failed to read image file: %v", err)}, nil
	}

	if len(imageData) == 0 {
		return coretool.Result{Error: fmt.Sprintf("Image file is empty: %s", filePath)}, nil
	}

	// Detect format from magic bytes.
	mediaType := sharedimage.DetectFormat(imageData)

	// Decode the image.
	img, originalWidth, originalHeight, err := sharedimage.Decode(imageData, mediaType)
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("Failed to decode image: %v", err)}, nil
	}

	// Apply standard resize: constrain to max dimension while preserving aspect ratio.
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
		return coretool.Result{Error: fmt.Sprintf("Failed to encode image: %v", err)}, nil
	}

	maxTokens := sharedimage.DefaultImageMaxTokens

	if sharedimage.EstimatedTokens(base64Str) > maxTokens {
		// Aggressive compression: try progressively smaller sizes and lower quality.
		base64Str, displayWidth, displayHeight, err = sharedimage.CompressWithTokenLimit(
			img, originalWidth, originalHeight, maxTokens,
		)
		if err != nil {
			// Fallback: 400x400 JPEG quality 20.
			fallbackWidth, fallbackHeight := sharedimage.ScaleDimensions(
				originalWidth, originalHeight, 400, 400,
			)
			fallbackImg := sharedimage.Resize(img, fallbackWidth, fallbackHeight)
			base64Str, err = sharedimage.EncodeToBase64(fallbackImg, jpeg.Options{Quality: 20})
			if err != nil {
				return coretool.Result{Error: fmt.Sprintf("Failed to compress image: %v", err)}, nil
			}
			displayWidth, displayHeight = fallbackWidth, fallbackHeight
		}
	}

	output := ImageOutput{
		Type:           "image",
		Base64:         base64Str,
		MediaType:      "image/jpeg",
		OriginalSize:   int(size),
		OriginalWidth:  originalWidth,
		OriginalHeight: originalHeight,
		DisplayWidth:   displayWidth,
		DisplayHeight:  displayHeight,
	}

	summary := fmt.Sprintf("Read image (%s", formatByteSize(size))
	if displayWidth > 0 && displayHeight > 0 {
		summary = fmt.Sprintf("%s, %dx%d", summary, displayWidth, displayHeight)
	}
	summary += ")"

	return coretool.Result{
		Output: summary,
		Meta: map[string]any{
			"data": output,
			"image": coretool.ImageData{
				MediaType: output.MediaType,
				Base64:    output.Base64,
			},
		},
	}, nil
}
