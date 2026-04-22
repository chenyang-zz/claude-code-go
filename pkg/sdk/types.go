package sdk

// Message is the marker interface for all SDK message types.
type Message interface {
	isSDKMessage()
}

// Base contains fields common to every SDK message.
type Base struct {
	Type      string `json:"type"`
	UUID      string `json:"uuid,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

// MCPServer describes an MCP server configuration.
type MCPServer struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// Plugin describes a plugin configuration.
type Plugin struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Source string `json:"source,omitempty"`
}
