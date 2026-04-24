package openai

import "strings"

// UseResponsesAPI reports whether the given model name should use the native
// OpenAI Responses API instead of the Chat Completions compatibility path.
//
// The heuristic matches models that are documented as Responses-API-only or
// where Responses API is the recommended path (o1-pro, o3-mini, o3, etc.).
// It can be overridden at the call site when the user explicitly configures
// an API path.
func UseResponsesAPI(model string) bool {
	switch {
	case strings.HasPrefix(model, "o1-pro"):
		return true
	case strings.HasPrefix(model, "o3-mini"):
		return true
	case strings.HasPrefix(model, "o3-"):
		return true
	case model == "o1" || model == "o3":
		return true
	case strings.HasPrefix(model, "computer-use-"):
		return true
	default:
		return false
	}
}
