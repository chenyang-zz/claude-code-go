package file_read

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	"golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

const (
	// defaultMaxImageDimension is the client-side maximum dimension for image resizing.
	// The API internally resizes images larger than 1568px.
	defaultMaxImageDimension = 1568

	// defaultImageMaxTokens is the default token budget for images.
	// Derived from 8192 * 0.75 ≈ 6144.
	defaultImageMaxTokens = 6144

	// defaultMaxImageFileSize is the maximum raw image file size (20MB).
	defaultMaxImageFileSize = 20 * 1024 * 1024

	// tokenToBase64Ratio converts estimated tokens to base64 length.
	tokenToBase64Ratio = 8.0 // 1 / 0.125

	// base64OverheadFactor accounts for base64 encoding overhead.
	base64OverheadFactor = 4.0 / 3.0
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
	mediaType := detectImageFormat(imageData)

	// Decode the image.
	img, originalWidth, originalHeight, err := decodeImage(imageData, mediaType)
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("Failed to decode image: %v", err)}, nil
	}

	// Apply standard resize: constrain to max dimension while preserving aspect ratio.
	displayWidth, displayHeight := originalWidth, originalHeight
	if originalWidth > defaultMaxImageDimension || originalHeight > defaultMaxImageDimension {
		displayWidth, displayHeight = scaleDimensions(
			originalWidth, originalHeight,
			defaultMaxImageDimension, defaultMaxImageDimension,
		)
		img = resizeImage(img, displayWidth, displayHeight)
	}

	// Encode as JPEG quality 80 and check token budget.
	base64Str, err := encodeImageToBase64(img, jpeg.Options{Quality: 80})
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("Failed to encode image: %v", err)}, nil
	}

	maxTokens := defaultImageMaxTokens
	estimatedTokens := int(math.Ceil(float64(len(base64Str)) * 0.125))

	if estimatedTokens > maxTokens {
		// Aggressive compression: try progressively smaller sizes and lower quality.
		base64Str, displayWidth, displayHeight, err = compressImageWithTokenLimit(
			img, originalWidth, originalHeight, maxTokens,
		)
		if err != nil {
			// Fallback: 400x400 JPEG quality 20.
			fallbackWidth, fallbackHeight := scaleDimensions(
				originalWidth, originalHeight, 400, 400,
			)
			fallbackImg := resizeImage(img, fallbackWidth, fallbackHeight)
			base64Str, err = encodeImageToBase64(fallbackImg, jpeg.Options{Quality: 20})
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
		},
	}, nil
}

// detectImageFormat identifies the image format from magic bytes.
func detectImageFormat(data []byte) string {
	if len(data) < 4 {
		return "image/png" // default
	}

	// PNG: 89 50 4E 47
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4e && data[3] == 0x47 {
		return "image/png"
	}

	// JPEG: FF D8 FF
	if data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return "image/jpeg"
	}

	// GIF: 47 49 46
	if data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 {
		return "image/gif"
	}

	// WebP: RIFF....WEBP
	if len(data) >= 12 && data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 {
		if data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50 {
			return "image/webp"
		}
	}

	return "image/png" // default
}

// decodeImage decodes image data based on its detected format.
func decodeImage(data []byte, mediaType string) (image.Image, int, int, error) {
	var img image.Image
	var err error

	switch mediaType {
	case "image/png":
		img, err = png.Decode(bytes.NewReader(data))
	case "image/jpeg":
		img, err = jpeg.Decode(bytes.NewReader(data))
	case "image/gif":
		img, err = gif.Decode(bytes.NewReader(data))
	case "image/webp":
		img, err = webp.Decode(bytes.NewReader(data))
	default:
		img, err = png.Decode(bytes.NewReader(data))
	}

	if err != nil {
		return nil, 0, 0, err
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	return img, width, height, nil
}

// scaleDimensions calculates new dimensions that fit within maxWidth x maxHeight
// while preserving the aspect ratio.
func scaleDimensions(width, height, maxWidth, maxHeight int) (int, int) {
	if width <= 0 || height <= 0 {
		return maxWidth, maxHeight
	}

	scaleW := float64(maxWidth) / float64(width)
	scaleH := float64(maxHeight) / float64(height)
	scale := math.Min(scaleW, scaleH)

	if scale >= 1.0 {
		return width, height
	}

	newWidth := int(math.Round(float64(width) * scale))
	newHeight := int(math.Round(float64(height) * scale))

	// Ensure at least 1 pixel.
	if newWidth < 1 {
		newWidth = 1
	}
	if newHeight < 1 {
		newHeight = 1
	}

	return newWidth, newHeight
}

// resizeImage resizes the image to the specified dimensions using Catmull-Rom interpolation.
func resizeImage(src image.Image, width, height int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}

// encodeImageToBase64 encodes an image to JPEG and returns base64 string.
func encodeImageToBase64(img image.Image, opts jpeg.Options) (string, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &opts); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// compressImageWithTokenLimit progressively compresses image to fit token budget.
// Returns base64 string and display dimensions.
func compressImageWithTokenLimit(
	img image.Image,
	originalWidth, originalHeight, maxTokens int,
) (string, int, int, error) {
	// Strategy 1: Resize to max 1024, JPEG quality 80.
	w, h := scaleDimensions(originalWidth, originalHeight, 1024, 1024)
	resized := resizeImage(img, w, h)
	base64Str, err := encodeImageToBase64(resized, jpeg.Options{Quality: 80})
	if err != nil {
		return "", 0, 0, err
	}
	if estimatedTokens(base64Str) <= maxTokens {
		return base64Str, w, h, nil
	}

	// Strategy 2: Resize to max 512, JPEG quality 60.
	w, h = scaleDimensions(originalWidth, originalHeight, 512, 512)
	resized = resizeImage(img, w, h)
	base64Str, err = encodeImageToBase64(resized, jpeg.Options{Quality: 60})
	if err != nil {
		return "", 0, 0, err
	}
	if estimatedTokens(base64Str) <= maxTokens {
		return base64Str, w, h, nil
	}

	// Strategy 3: Resize to max 400, JPEG quality 20.
	w, h = scaleDimensions(originalWidth, originalHeight, 400, 400)
	resized = resizeImage(img, w, h)
	base64Str, err = encodeImageToBase64(resized, jpeg.Options{Quality: 20})
	if err != nil {
		return "", 0, 0, err
	}
	if estimatedTokens(base64Str) <= maxTokens {
		return base64Str, w, h, nil
	}

	return "", 0, 0, fmt.Errorf("unable to compress image to fit within token limit")
}

// estimatedTokens estimates token count from base64 string length.
func estimatedTokens(base64Str string) int {
	return int(math.Ceil(float64(len(base64Str)) * 0.125))
}
