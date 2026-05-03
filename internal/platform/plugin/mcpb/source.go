package mcpb

import "strings"

// IsMcpbSource reports whether source identifies an MCPB file reference.
// It returns true for paths ending with .mcpb or .dxt.
func IsMcpbSource(source string) bool {
	return strings.HasSuffix(source, ".mcpb") || strings.HasSuffix(source, ".dxt")
}

// IsURL reports whether source is an HTTP or HTTPS URL.
func IsURL(source string) bool {
	return strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://")
}
