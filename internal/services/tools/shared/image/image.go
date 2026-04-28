// Package image provides shared image processing utilities used by tools that
// need to detect, decode, resize, compress, and encode images (FileReadTool,
// BashTool, etc.).
package image

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"

	"golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

const (
	// DefaultMaxImageDimension is the client-side maximum dimension for image resizing.
	// The API internally resizes images larger than 1568px.
	DefaultMaxImageDimension = 1568

	// DefaultImageMaxTokens is the default token budget for images.
	// Derived from 8192 * 0.75 ≈ 6144.
	DefaultImageMaxTokens = 6144

	// DefaultMaxImageFileSize is the maximum raw image file size (20MB).
	DefaultMaxImageFileSize = 20 * 1024 * 1024

)

// Output is the structured metadata returned for image processing results.
type Output struct {
	// Type identifies the image result branch.
	Type string `json:"type"`
	// Base64 stores the base64-encoded image data.
	Base64 string `json:"base64"`
	// MediaType stores the MIME type of the image (e.g., image/jpeg).
	MediaType string `json:"media_type"`
	// OriginalSize stores the original file size in bytes.
	OriginalSize int `json:"originalSize"`
	// OriginalWidth stores the original image width in pixels.
	OriginalWidth int `json:"originalWidth,omitempty"`
	// OriginalHeight stores the original image height in pixels.
	OriginalHeight int `json:"originalHeight,omitempty"`
	// DisplayWidth stores the displayed width after resizing.
	DisplayWidth int `json:"displayWidth,omitempty"`
	// DisplayHeight stores the displayed height after resizing.
	DisplayHeight int `json:"displayHeight,omitempty"`
}

// DetectFormat identifies the image format from magic bytes.
// Returns the MIME type string (e.g., "image/png", "image/jpeg").
func DetectFormat(data []byte) string {
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

// Decode decodes image data based on its detected format.
func Decode(data []byte, mediaType string) (image.Image, int, int, error) {
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

// ScaleDimensions calculates new dimensions that fit within maxWidth x maxHeight
// while preserving the aspect ratio.
func ScaleDimensions(width, height, maxWidth, maxHeight int) (int, int) {
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

// Resize resizes the image to the specified dimensions using Catmull-Rom interpolation.
func Resize(src image.Image, width, height int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}

// EncodeToBase64 encodes an image to JPEG and returns the base64 string.
func EncodeToBase64(img image.Image, opts jpeg.Options) (string, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &opts); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// CompressWithTokenLimit progressively compresses an image to fit within the
// token budget. Returns the base64 string and display dimensions.
func CompressWithTokenLimit(
	img image.Image,
	originalWidth, originalHeight, maxTokens int,
) (string, int, int, error) {
	// Strategy 1: Resize to max 1024, JPEG quality 80.
	w, h := ScaleDimensions(originalWidth, originalHeight, 1024, 1024)
	resized := Resize(img, w, h)
	base64Str, err := EncodeToBase64(resized, jpeg.Options{Quality: 80})
	if err != nil {
		return "", 0, 0, err
	}
	if EstimatedTokens(base64Str) <= maxTokens {
		return base64Str, w, h, nil
	}

	// Strategy 2: Resize to max 512, JPEG quality 60.
	w, h = ScaleDimensions(originalWidth, originalHeight, 512, 512)
	resized = Resize(img, w, h)
	base64Str, err = EncodeToBase64(resized, jpeg.Options{Quality: 60})
	if err != nil {
		return "", 0, 0, err
	}
	if EstimatedTokens(base64Str) <= maxTokens {
		return base64Str, w, h, nil
	}

	// Strategy 3: Resize to max 400, JPEG quality 20.
	w, h = ScaleDimensions(originalWidth, originalHeight, 400, 400)
	resized = Resize(img, w, h)
	base64Str, err = EncodeToBase64(resized, jpeg.Options{Quality: 20})
	if err != nil {
		return "", 0, 0, err
	}
	if EstimatedTokens(base64Str) <= maxTokens {
		return base64Str, w, h, nil
	}

	return "", 0, 0, fmt.Errorf("unable to compress image to fit within token limit")
}

// EstimatedTokens estimates token count from a base64 string length.
func EstimatedTokens(base64Str string) int {
	return int(math.Ceil(float64(len(base64Str)) * 0.125))
}
