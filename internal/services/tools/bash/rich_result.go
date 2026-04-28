package bash

import (
	sharedimage "github.com/sheepzhao/claude-code-go/internal/services/tools/shared/image"
)

// RichResultMetadata holds additional structured metadata about a Bash
// command's output beyond the basic stdout/stderr/exit code.
type RichResultMetadata struct {
	// IsImage indicates whether stdout contains a base64-encoded image data URI.
	IsImage bool `json:"isImage,omitempty"`
	// PersistedOutputPath is the path where full output was persisted when the
	// inline output exceeded the maximum length.
	PersistedOutputPath string `json:"persistedOutputPath,omitempty"`
	// PersistedOutputSize is the total size in bytes of the persisted output.
	PersistedOutputSize int64 `json:"persistedOutputSize,omitempty"`
	// ImageData holds the structured image output when IsImage is true.
	ImageData *sharedimage.Output `json:"imageData,omitempty"`
}

// buildRichMetaFromOutput constructs the Meta map for the tool result,
// combining the standard output data with rich result metadata.
func buildRichMetaFromOutput(output Output, richMeta RichResultMetadata) map[string]any {
	meta := map[string]any{
		"data": output,
	}
	if richMeta.IsImage && richMeta.ImageData != nil {
		meta["image"] = richMeta.ImageData
	}
	return meta
}
