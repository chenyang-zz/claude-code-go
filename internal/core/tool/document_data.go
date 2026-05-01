package tool

// DocumentData carries the normalized document payload for tool results that
// produce a single document output (typically PDF). It is stored in
// Result.Meta under the "document" key so the runtime engine can convert it
// into a document content block alongside the tool_result block.
type DocumentData struct {
	// MediaType is the MIME type of the document (e.g., application/pdf).
	MediaType string `json:"media_type"`
	// Base64 is the base64-encoded document payload.
	Base64 string `json:"base64"`
}
