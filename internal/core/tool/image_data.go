package tool

// ImageData carries the normalized image payload for tool results that
// produce image output. It is stored in Result.Meta under the "image" key
// so the runtime engine can convert it into an image content block.
type ImageData struct {
	// MediaType is the MIME type of the image (e.g., image/jpeg).
	MediaType string `json:"media_type"`
	// Base64 is the base64-encoded image data.
	Base64 string `json:"base64"`
}
